package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const (
	bookingLockPrefix    = "booking:lock"
	searchAutocomplete   = "search:autocomplete"
	searchCourtNames     = "search:courts"
	searchCachePrefix    = "search:cache"
	searchPopular        = "search:popular"
	matchJoinQueuePrefix = "match:join:queue"
	matchPlayerLock      = "match:player:lock"

	searchCacheTTL  = 300  // seconds
	searchPopularTTL = 86400 // seconds
)

type RedisService struct {
	client *goredis.Client
	ttlSec int
}

func NewRedisService(client *goredis.Client, slotLockTTL int) *RedisService {
	return &RedisService{client: client, ttlSec: slotLockTTL}
}

func (s *RedisService) slotKey(subCourtID, date, startTime, endTime string) string {
	return fmt.Sprintf("%s:%s:%s:%s:%s", bookingLockPrefix, subCourtID, date, startTime, endTime)
}

// AcquireSlotLock uses SET NX to atomically claim a time slot.
func (s *RedisService) AcquireSlotLock(ctx context.Context, subCourtID, date, startTime, endTime, bookingID string) (bool, error) {
	key := s.slotKey(subCourtID, date, startTime, endTime)
	ok, err := s.client.SetNX(ctx, key, bookingID, goredis.KeepTTL).Result()
	if err == nil && ok {
		s.client.Expire(ctx, key, time.Duration(s.ttlSec)*time.Second) //nolint:errcheck
	}
	return ok, err
}

// ReleaseSlotLock atomically releases a lock only if owned by bookingID.
var releaseScript = goredis.NewScript(`
if redis.call("get",KEYS[1]) == ARGV[1] then
  return redis.call("del",KEYS[1])
else
  return 0
end`)

func (s *RedisService) ReleaseSlotLock(ctx context.Context, subCourtID, date, startTime, endTime, bookingID string) (bool, error) {
	key := s.slotKey(subCourtID, date, startTime, endTime)
	n, err := releaseScript.Run(ctx, s.client, []string{key}, bookingID).Int()
	return n == 1, err
}

// AcquireSlotLocks acquires multiple slot locks atomically (all-or-nothing).
type SlotLockSpec struct {
	SubCourtID string
	Date       string
	StartTime  string
	EndTime    string
	BookingID  string
}

func (s *RedisService) AcquireSlotLocks(ctx context.Context, slots []SlotLockSpec) (bool, error) {
	acquired := make([]string, 0, len(slots))
	for _, slot := range slots {
		ok, err := s.AcquireSlotLock(ctx, slot.SubCourtID, slot.Date, slot.StartTime, slot.EndTime, slot.BookingID)
		if err != nil {
			return false, err
		}
		if !ok {
			// Release all previously acquired
			for _, k := range acquired {
				s.client.Del(ctx, k) //nolint:errcheck
			}
			return false, nil
		}
		acquired = append(acquired, s.slotKey(slot.SubCourtID, slot.Date, slot.StartTime, slot.EndTime))
	}
	return true, nil
}

func (s *RedisService) ReleaseSlotLocks(ctx context.Context, slots []SlotLockSpec) {
	for _, slot := range slots {
		s.ReleaseSlotLock(ctx, slot.SubCourtID, slot.Date, slot.StartTime, slot.EndTime, slot.BookingID) //nolint:errcheck
	}
}

// --- Autocomplete ---

func (s *RedisService) AddToAutocomplete(ctx context.Context, courtID, courtName string, terms []string, score float64) error {
	pipe := s.client.Pipeline()
	pipe.HSet(ctx, searchCourtNames, courtID, courtName)
	for _, term := range terms {
		t := strings.ToLower(strings.TrimSpace(term))
		if len(t) >= 2 {
			pipe.ZAdd(ctx, searchAutocomplete, goredis.Z{Score: score, Member: t + "|" + courtID})
		}
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (s *RedisService) RemoveFromAutocomplete(ctx context.Context, courtID string) error {
	all, err := s.client.ZRange(ctx, searchAutocomplete, 0, -1).Result()
	if err != nil {
		return err
	}
	var toRemove []interface{}
	suffix := "|" + courtID
	for _, m := range all {
		if strings.HasSuffix(m, suffix) {
			toRemove = append(toRemove, m)
		}
	}
	if len(toRemove) > 0 {
		if err := s.client.ZRem(ctx, searchAutocomplete, toRemove...).Err(); err != nil {
			return err
		}
	}
	return s.client.HDel(ctx, searchCourtNames, courtID).Err()
}

type AutocompleteSuggestion struct {
	ID    string
	Text  string
	Score float64
}

func (s *RedisService) SearchAutocomplete(ctx context.Context, prefix string, limit int) ([]AutocompleteSuggestion, error) {
	norm := strings.ToLower(strings.TrimSpace(prefix))
	if len(norm) < 2 {
		return nil, nil
	}
	words := strings.Fields(norm)

	allMembers, err := s.client.ZRevRangeWithScores(ctx, searchAutocomplete, 0, -1).Result()
	if err != nil {
		return nil, err
	}

	type courtData struct {
		terms    []string
		maxScore float64
	}
	courtMap := make(map[string]*courtData)
	for _, z := range allMembers {
		member := z.Member.(string)
		parts := strings.SplitN(member, "|", 2)
		if len(parts) != 2 {
			continue
		}
		term, courtID := parts[0], parts[1]
		if cd, ok := courtMap[courtID]; ok {
			cd.terms = append(cd.terms, term)
			if z.Score > cd.maxScore {
				cd.maxScore = z.Score
			}
		} else {
			courtMap[courtID] = &courtData{terms: []string{term}, maxScore: z.Score}
		}
	}

	type match struct {
		courtID string
		score   float64
	}
	var matches []match
	for courtID, cd := range courtMap {
		allMatch := true
		for _, w := range words {
			found := false
			for _, t := range cd.terms {
				if strings.HasPrefix(t, w) {
					found = true
					break
				}
			}
			if !found {
				allMatch = false
				break
			}
		}
		if allMatch {
			matches = append(matches, match{courtID, cd.maxScore})
		}
		if len(matches) >= limit {
			break
		}
	}

	if len(matches) == 0 {
		return nil, nil
	}

	courtIDs := make([]string, len(matches))
	for i, m := range matches {
		courtIDs[i] = m.courtID
	}

	names, err := s.client.HMGet(ctx, searchCourtNames, courtIDs...).Result()
	if err != nil {
		return nil, err
	}

	var suggestions []AutocompleteSuggestion
	for i, m := range matches {
		text := courtIDs[i]
		if names[i] != nil {
			text = names[i].(string)
		}
		suggestions = append(suggestions, AutocompleteSuggestion{
			ID:    m.courtID,
			Text:  text,
			Score: m.score,
		})
	}
	return suggestions, nil
}

func (s *RedisService) ClearAutocomplete(ctx context.Context) error {
	return s.client.Del(ctx, searchAutocomplete, searchCourtNames).Err()
}

func (s *RedisService) GetAutocompleteCount(ctx context.Context) (int64, error) {
	return s.client.ZCard(ctx, searchAutocomplete).Result()
}

// --- Search Cache ---

func (s *RedisService) CacheSearchResults(ctx context.Context, hash string, data []byte) error {
	return s.client.SetEx(ctx, searchCachePrefix+":"+hash, string(data), time.Duration(searchCacheTTL)*time.Second).Err()
}

func (s *RedisService) GetCachedSearch(ctx context.Context, hash string) ([]byte, error) {
	v, err := s.client.Get(ctx, searchCachePrefix+":"+hash).Bytes()
	if err == goredis.Nil {
		return nil, nil
	}
	return v, err
}

// --- Popular Searches ---

func (s *RedisService) TrackSearch(ctx context.Context, query string) error {
	norm := strings.ToLower(strings.TrimSpace(query))
	if len(norm) < 2 {
		return nil
	}
	s.client.ZIncrBy(ctx, searchPopular, 1, norm) //nolint:errcheck
	return s.client.Expire(ctx, searchPopular, time.Duration(searchPopularTTL)*time.Second).Err()
}

func (s *RedisService) GetPopularSearches(ctx context.Context, limit int) ([]string, error) {
	return s.client.ZRevRange(ctx, searchPopular, 0, int64(limit-1)).Result()
}

// --- Match Join Queue ---

func (s *RedisService) AcquireMatchPlayerLock(ctx context.Context, matchID, userID string) (bool, error) {
	key := fmt.Sprintf("%s:%s:%s", matchPlayerLock, matchID, userID)
	return s.client.SetNX(ctx, key, "1", 60*time.Second).Result()
}

func (s *RedisService) ReleaseMatchPlayerLock(ctx context.Context, matchID, userID string) {
	s.client.Del(ctx, fmt.Sprintf("%s:%s:%s", matchPlayerLock, matchID, userID)) //nolint:errcheck
}

func (s *RedisService) AddToMatchJoinQueue(ctx context.Context, matchID, userID string) error {
	key := fmt.Sprintf("%s:%s", matchJoinQueuePrefix, matchID)
	return s.client.ZAdd(ctx, key, goredis.Z{Score: float64(time.Now().UnixMilli()), Member: userID}).Err()
}

func (s *RedisService) RemoveFromMatchJoinQueue(ctx context.Context, matchID, userID string) error {
	return s.client.ZRem(ctx, fmt.Sprintf("%s:%s", matchJoinQueuePrefix, matchID), userID).Err()
}

func (s *RedisService) GetMatchJoinQueuePosition(ctx context.Context, matchID, userID string) (int64, error) {
	rank, err := s.client.ZRank(ctx, fmt.Sprintf("%s:%s", matchJoinQueuePrefix, matchID), userID).Result()
	if err == goredis.Nil {
		return 0, nil
	}
	return rank + 1, err
}

func (s *RedisService) ClearMatchJoinQueue(ctx context.Context, matchID string) error {
	return s.client.Del(ctx, fmt.Sprintf("%s:%s", matchJoinQueuePrefix, matchID)).Err()
}
