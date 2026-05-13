package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smatch/badminton-backend/internal/domain"
)

type MatchRepository struct {
	db *pgxpool.Pool
}

func NewMatchRepository(db *pgxpool.Pool) *MatchRepository {
	return &MatchRepository{db: db}
}

// MatchRow is a match with host + court info joined in.
type MatchRow struct {
	domain.Match
	CourtName       string
	CourtAddressStr string
	HostFirstName   *string
	HostLastName    *string
	HostUsername    *string
	HostEmail       *string
	SlotsAccepted   int
}

// MatchPlayerRow is a player with user info.
type MatchPlayerRow struct {
	domain.MatchPlayer
	UserFirstName *string
	UserLastName  *string
	UserUsername  *string
	UserEmail     *string
	UserPhotoURL  *string
}

const matchSelectCols = `
	m.id, m.court_id, m.host_user_id,
	m.title, m.description, m.images,
	m.skill_level, m.shuttle_type, m.player_format,
	TO_CHAR(m.date,'YYYY-MM-DD'), TO_CHAR(m.start_time,'HH24:MI'), TO_CHAR(m.end_time,'HH24:MI'),
	m.is_private, m.price, m.slots_needed, m.status,
	m.created_at, m.updated_at,
	c.name AS court_name,
	CONCAT_WS(', ', c.address_street, c.address_ward, c.address_district, c.address_city) AS court_address,
	u.first_name, u.last_name, u.username, u.email,
	(SELECT COUNT(*) FROM match_players mp WHERE mp.match_id = m.id AND mp.status = 'ACCEPTED') AS slots_accepted
`

func scanMatchRow(row pgx.Row) (*MatchRow, error) {
	mr := &MatchRow{}
	var dateStr, startStr, endStr string
	err := row.Scan(
		&mr.ID, &mr.CourtID, &mr.HostUserID,
		&mr.Title, &mr.Description, &mr.Images,
		&mr.SkillLevel, &mr.ShuttleType, &mr.PlayerFormat,
		&dateStr, &startStr, &endStr,
		&mr.IsPrivate, &mr.Price, &mr.SlotsNeeded, &mr.Status,
		&mr.CreatedAt, &mr.UpdatedAt,
		&mr.CourtName, &mr.CourtAddressStr,
		&mr.HostFirstName, &mr.HostLastName, &mr.HostUsername, &mr.HostEmail,
		&mr.SlotsAccepted,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	mr.Date, _ = time.Parse("2006-01-02", dateStr)
	mr.StartTime = startStr
	mr.EndTime = endStr
	return mr, nil
}

// FindByID fetches a single match with all joins.
func (r *MatchRepository) FindByID(ctx context.Context, id string) (*MatchRow, error) {
	return scanMatchRow(r.db.QueryRow(ctx, `
		SELECT `+matchSelectCols+`
		FROM matches m
		JOIN courts c ON m.court_id = c.id
		JOIN users u ON m.host_user_id = u.id
		WHERE m.id = $1::uuid
	`, id))
}

// FindAll returns matches with optional filters.
type MatchFilter struct {
	CourtID        *string
	SkillLevel     *string
	PlayerFormat   *string
	Status         *string
	Date           *string
	DateFrom       *string
	DateTo         *string
	IncludeExpired bool
	Page           int
	Limit          int
}

func (r *MatchRepository) FindAll(ctx context.Context, f MatchFilter) ([]*MatchRow, int, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	i := 1

	if f.CourtID != nil {
		where = append(where, fmt.Sprintf("m.court_id = $%d::uuid", i))
		args = append(args, *f.CourtID)
		i++
	}
	if f.SkillLevel != nil {
		where = append(where, fmt.Sprintf("m.skill_level = $%d", i))
		args = append(args, *f.SkillLevel)
		i++
	}
	if f.PlayerFormat != nil {
		where = append(where, fmt.Sprintf("m.player_format = $%d", i))
		args = append(args, *f.PlayerFormat)
		i++
	}
	if f.Status != nil {
		where = append(where, fmt.Sprintf("m.status = $%d", i))
		args = append(args, *f.Status)
		i++
	}
	if f.Date != nil {
		where = append(where, fmt.Sprintf("m.date = $%d::date", i))
		args = append(args, *f.Date)
		i++
	}
	if f.DateFrom != nil {
		where = append(where, fmt.Sprintf("m.date >= $%d::date", i))
		args = append(args, *f.DateFrom)
		i++
	}
	if f.DateTo != nil {
		where = append(where, fmt.Sprintf("m.date <= $%d::date", i))
		args = append(args, *f.DateTo)
		i++
	}
	if !f.IncludeExpired {
		where = append(where, "(m.date > CURRENT_DATE OR (m.date = CURRENT_DATE AND m.end_time > CURRENT_TIME))")
	}

	whereClause := strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM matches m WHERE `+whereClause, args...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	page := f.Page
	limit := f.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	args = append(args, limit, offset)
	rows, err := r.db.Query(ctx, `
		SELECT `+matchSelectCols+`
		FROM matches m
		JOIN courts c ON m.court_id = c.id
		JOIN users u ON m.host_user_id = u.id
		WHERE `+whereClause+`
		ORDER BY m.date ASC, m.start_time ASC
		LIMIT $`+fmt.Sprintf("%d", i)+` OFFSET $`+fmt.Sprintf("%d", i+1),
		args...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var result []*MatchRow
	for rows.Next() {
		mr, err := scanMatchRow(rows)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, mr)
	}
	return result, total, rows.Err()
}

// FindByHostUserID returns all matches hosted by a user.
func (r *MatchRepository) FindByHostUserID(ctx context.Context, userID string, includeExpired bool) ([]*MatchRow, error) {
	expired := ""
	if !includeExpired {
		expired = "AND (m.date > CURRENT_DATE OR (m.date = CURRENT_DATE AND m.end_time > CURRENT_TIME))"
	}
	rows, err := r.db.Query(ctx, `
		SELECT `+matchSelectCols+`
		FROM matches m
		JOIN courts c ON m.court_id = c.id
		JOIN users u ON m.host_user_id = u.id
		WHERE m.host_user_id = $1::uuid `+expired+`
		ORDER BY m.date DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMatchRows(rows)
}

// FindByPlayerUserID returns matches a user has joined.
func (r *MatchRepository) FindByPlayerUserID(ctx context.Context, userID string, includeExpired bool) ([]*MatchRow, error) {
	expired := ""
	if !includeExpired {
		expired = "AND (m.date > CURRENT_DATE OR (m.date = CURRENT_DATE AND m.end_time > CURRENT_TIME))"
	}
	rows, err := r.db.Query(ctx, `
		SELECT `+matchSelectCols+`
		FROM matches m
		JOIN courts c ON m.court_id = c.id
		JOIN users u ON m.host_user_id = u.id
		JOIN match_players mp ON mp.match_id = m.id
		WHERE mp.user_id = $1::uuid
		  AND mp.status IN ('ACCEPTED','PENDING_PAYMENT') `+expired+`
		ORDER BY m.date DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMatchRows(rows)
}

// Create inserts a new match.
func (r *MatchRepository) Create(ctx context.Context, m *domain.Match, hostUserID string) (*MatchRow, error) {
	return scanMatchRow(r.db.QueryRow(ctx, `
		INSERT INTO matches (court_id, host_user_id, title, description, images,
		                     skill_level, shuttle_type, player_format,
		                     date, start_time, end_time,
		                     is_private, price, slots_needed, status)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5,
		        $6, $7, $8,
		        $9::date, $10::time, $11::time,
		        $12, $13, $14, 'OPEN')
		RETURNING id,
		  (SELECT `+matchSelectCols+` FROM matches m JOIN courts c ON m.court_id=c.id JOIN users u ON m.host_user_id=u.id WHERE m.id=currval('matches_id_seq'::regclass))
	`, m.CourtID, hostUserID, m.Title, m.Description, m.Images,
		string(m.SkillLevel), string(m.ShuttleType), string(m.PlayerFormat),
		m.Date.Format("2006-01-02"), m.StartTime, m.EndTime,
		m.IsPrivate, m.Price, m.SlotsNeeded))
}

// simpler Create that returns the ID, then re-fetch
func (r *MatchRepository) CreateAndFetch(ctx context.Context, m *domain.Match, hostUserID string) (*MatchRow, error) {
	var id string
	err := r.db.QueryRow(ctx, `
		INSERT INTO matches (court_id, host_user_id, title, description, images,
		                     skill_level, shuttle_type, player_format,
		                     date, start_time, end_time,
		                     is_private, price, slots_needed, status)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5,
		        $6, $7, $8,
		        $9::date, $10::time, $11::time,
		        $12, $13, $14, 'OPEN')
		RETURNING id
	`, m.CourtID, hostUserID, m.Title, m.Description, m.Images,
		string(m.SkillLevel), string(m.ShuttleType), string(m.PlayerFormat),
		m.Date.Format("2006-01-02"), m.StartTime, m.EndTime,
		m.IsPrivate, m.Price, m.SlotsNeeded,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

// UpdateStatus sets a match status.
func (r *MatchRepository) UpdateStatus(ctx context.Context, id string, status domain.MatchStatus) error {
	_, err := r.db.Exec(ctx, `UPDATE matches SET status=$2, updated_at=NOW() WHERE id=$1::uuid`, id, string(status))
	return err
}

// Update applies partial field updates.
func (r *MatchRepository) Update(ctx context.Context, id string, fields map[string]interface{}) (*MatchRow, error) {
	if len(fields) == 0 {
		return r.FindByID(ctx, id)
	}
	sets := make([]string, 0, len(fields))
	args := make([]interface{}, 0, len(fields)+1)
	idx := 1
	for col, val := range fields {
		sets = append(sets, fmt.Sprintf("%s = $%d", col, idx))
		args = append(args, val)
		idx++
	}
	args = append(args, id)
	_, err := r.db.Exec(ctx,
		`UPDATE matches SET `+strings.Join(sets, ", ")+`, updated_at=NOW() WHERE id=$`+fmt.Sprintf("%d", idx)+`::uuid`,
		args...)
	if err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

// FindPlayer finds a player entry for a match+user pair.
func (r *MatchRepository) FindPlayer(ctx context.Context, matchID, userID string) (*MatchPlayerRow, error) {
	return scanPlayerRow(r.db.QueryRow(ctx, `
		SELECT mp.id, mp.match_id, mp.user_id, mp.status, mp.message, mp.position, mp.requested_at, mp.responded_at,
		       u.first_name, u.last_name, u.username, u.email, u.photo_url
		FROM match_players mp JOIN users u ON mp.user_id = u.id
		WHERE mp.match_id = $1::uuid AND mp.user_id = $2::uuid
	`, matchID, userID))
}

// FindPlayerByID fetches a player by its primary key.
func (r *MatchRepository) FindPlayerByID(ctx context.Context, playerID string) (*MatchPlayerRow, error) {
	return scanPlayerRow(r.db.QueryRow(ctx, `
		SELECT mp.id, mp.match_id, mp.user_id, mp.status, mp.message, mp.position, mp.requested_at, mp.responded_at,
		       u.first_name, u.last_name, u.username, u.email, u.photo_url
		FROM match_players mp JOIN users u ON mp.user_id = u.id
		WHERE mp.id = $1::uuid
	`, playerID))
}

// GetPlayersByMatchID returns players for a match with optional status filter.
func (r *MatchRepository) GetPlayersByMatchID(ctx context.Context, matchID string, status *string) ([]*MatchPlayerRow, error) {
	q := `
		SELECT mp.id, mp.match_id, mp.user_id, mp.status, mp.message, mp.position, mp.requested_at, mp.responded_at,
		       u.first_name, u.last_name, u.username, u.email, u.photo_url
		FROM match_players mp JOIN users u ON mp.user_id = u.id
		WHERE mp.match_id = $1::uuid`
	args := []interface{}{matchID}
	if status != nil {
		q += " AND mp.status = $2"
		args = append(args, *status)
	}
	q += " ORDER BY mp.requested_at"
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPlayerRows(rows)
}

// CountAcceptedPlayers returns the count of ACCEPTED players for a match.
func (r *MatchRepository) CountAcceptedPlayers(ctx context.Context, matchID string) (int, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM match_players WHERE match_id = $1::uuid AND status = 'ACCEPTED'`,
		matchID).Scan(&count)
	return count, err
}

// GetNextPosition returns the next available position for a match.
func (r *MatchRepository) GetNextPosition(ctx context.Context, matchID string) (int, error) {
	var maxPos *int
	_ = r.db.QueryRow(ctx,
		`SELECT MAX(position) FROM match_players WHERE match_id=$1::uuid AND status='ACCEPTED'`,
		matchID).Scan(&maxPos)
	if maxPos == nil {
		return 1, nil
	}
	return *maxPos + 1, nil
}

// AddPlayer inserts a new match_players row.
func (r *MatchRepository) AddPlayer(ctx context.Context, matchID, userID string, message *string, status domain.MatchPlayerStatus) (*MatchPlayerRow, error) {
	var playerID string
	err := r.db.QueryRow(ctx, `
		INSERT INTO match_players (match_id, user_id, status, message)
		VALUES ($1::uuid, $2::uuid, $3, $4)
		RETURNING id
	`, matchID, userID, string(status), message).Scan(&playerID)
	if err != nil {
		return nil, err
	}
	return r.FindPlayerByID(ctx, playerID)
}

// UpdatePlayerStatus updates a match_players status and optional position.
func (r *MatchRepository) UpdatePlayerStatus(ctx context.Context, playerID string, status domain.MatchPlayerStatus, position *int) (*MatchPlayerRow, error) {
	_, err := r.db.Exec(ctx, `
		UPDATE match_players
		SET status = $2, position = COALESCE($3, position), responded_at = NOW()
		WHERE id = $1::uuid
	`, playerID, string(status), position)
	if err != nil {
		return nil, err
	}
	return r.FindPlayerByID(ctx, playerID)
}

// GetJoinedMatchIDs returns all match IDs where a user is ACCEPTED.
func (r *MatchRepository) GetJoinedMatchIDs(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT match_id::text FROM match_players
		WHERE user_id = $1::uuid AND status = 'ACCEPTED'
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		rows.Scan(&id) //nolint:errcheck
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetAcceptedMatchesByUserAndDate returns ACCEPTED match_players for a user on a date.
func (r *MatchRepository) GetAcceptedMatchesByUserAndDate(ctx context.Context, userID string, date time.Time) ([]*MatchPlayerRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT mp.id, mp.match_id, mp.user_id, mp.status, mp.message, mp.position, mp.requested_at, mp.responded_at,
		       u.first_name, u.last_name, u.username, u.email, u.photo_url
		FROM match_players mp
		JOIN users u ON mp.user_id = u.id
		JOIN matches m ON mp.match_id = m.id
		WHERE mp.user_id = $1::uuid AND mp.status = 'ACCEPTED' AND m.date = $2::date
	`, userID, date.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPlayerRows(rows)
}

// MarkExpiredMatchPlayers sets PENDING_PAYMENT players to EXPIRED given a list of player IDs.
func (r *MatchRepository) MarkExpiredMatchPlayers(ctx context.Context, playerIDs []string) error {
	if len(playerIDs) == 0 {
		return nil
	}
	// Build $1,$2,... placeholders
	ph := make([]string, len(playerIDs))
	args := make([]interface{}, len(playerIDs))
	for i, id := range playerIDs {
		ph[i] = fmt.Sprintf("$%d::uuid", i+1)
		args[i] = id
	}
	_, err := r.db.Exec(ctx,
		`UPDATE match_players SET status='EXPIRED' WHERE id IN (`+strings.Join(ph, ",")+`)`,
		args...)
	return err
}

func scanPlayerRow(row pgx.Row) (*MatchPlayerRow, error) {
	mp := &MatchPlayerRow{}
	err := row.Scan(
		&mp.ID, &mp.MatchID, &mp.UserID, &mp.Status, &mp.Message, &mp.Position, &mp.RequestedAt, &mp.RespondedAt,
		&mp.UserFirstName, &mp.UserLastName, &mp.UserUsername, &mp.UserEmail, &mp.UserPhotoURL,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return mp, err
}

func scanPlayerRows(rows pgx.Rows) ([]*MatchPlayerRow, error) {
	var result []*MatchPlayerRow
	for rows.Next() {
		mp := &MatchPlayerRow{}
		if err := rows.Scan(
			&mp.ID, &mp.MatchID, &mp.UserID, &mp.Status, &mp.Message, &mp.Position, &mp.RequestedAt, &mp.RespondedAt,
			&mp.UserFirstName, &mp.UserLastName, &mp.UserUsername, &mp.UserEmail, &mp.UserPhotoURL,
		); err != nil {
			return nil, err
		}
		result = append(result, mp)
	}
	return result, rows.Err()
}

func scanMatchRows(rows pgx.Rows) ([]*MatchRow, error) {
	var result []*MatchRow
	for rows.Next() {
		mr, err := scanMatchRow(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, mr)
	}
	return result, rows.Err()
}
