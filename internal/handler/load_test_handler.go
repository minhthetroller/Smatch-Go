package handler

import (
	"context"
	"math"
	"net/http"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/smatch/badminton-backend/internal/dto"
)

const (
	defaultStressDurationMS = 750
	maxStressDurationMS     = 5000
	defaultStressWorkers    = 1
)

type LoadTestHandler struct {
	enabled     bool
	adminSecret string
}

func NewLoadTestHandler(enabled bool, adminSecret string) *LoadTestHandler {
	return &LoadTestHandler{
		enabled:     enabled,
		adminSecret: adminSecret,
	}
}

func (h *LoadTestHandler) Stress(w http.ResponseWriter, r *http.Request) {
	if !h.enabled {
		sendError(w, "Not found", "NOT_FOUND", http.StatusNotFound)
		return
	}
	if !h.authorized(r) {
		sendError(w, "Forbidden", "FORBIDDEN", http.StatusForbidden)
		return
	}

	durationMS, err := boundedIntQuery(r, "duration_ms", defaultStressDurationMS, 1, maxStressDurationMS)
	if err != nil {
		sendError(w, err.Error(), "BAD_REQUEST", http.StatusBadRequest)
		return
	}
	workers, err := boundedIntQuery(r, "workers", defaultStressWorkers, 1, runtime.NumCPU())
	if err != nil {
		sendError(w, err.Error(), "BAD_REQUEST", http.StatusBadRequest)
		return
	}

	start := time.Now()
	checksum := runStress(r.Context(), time.Duration(durationMS)*time.Millisecond, workers)
	elapsed := time.Since(start)

	sendSuccess(w, dto.LoadTestStressResponse{
		DurationMS: durationMS,
		Workers:    workers,
		ElapsedMS:  elapsed.Milliseconds(),
		Checksum:   checksum,
	}, http.StatusOK)
}

func (h *LoadTestHandler) authorized(r *http.Request) bool {
	return h.adminSecret != "" && r.Header.Get("X-Admin-Secret") == h.adminSecret
}

func boundedIntQuery(r *http.Request, name string, defaultValue, minValue, maxValue int) (int, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return defaultValue, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, errInvalidQuery(name, minValue, maxValue)
	}
	if value < minValue || value > maxValue {
		return 0, errInvalidQuery(name, minValue, maxValue)
	}
	return value, nil
}

type invalidQueryError struct {
	name     string
	minValue int
	maxValue int
}

func errInvalidQuery(name string, minValue, maxValue int) error {
	return invalidQueryError{name: name, minValue: minValue, maxValue: maxValue}
}

func (e invalidQueryError) Error() string {
	return e.name + " must be an integer between " + strconv.Itoa(e.minValue) + " and " + strconv.Itoa(e.maxValue)
}

func runStress(ctx context.Context, duration time.Duration, workers int) uint64 {
	deadline := time.Now().Add(duration)
	var wg sync.WaitGroup
	var checksum uint64

	for i := 0; i < workers; i++ {
		seed := uint64(i + 1)
		wg.Add(1)
		go func() {
			defer wg.Done()
			local := seed
			for iterations := 0; time.Now().Before(deadline); iterations++ {
				select {
				case <-ctx.Done():
					atomic.AddUint64(&checksum, local)
					return
				default:
				}

				local ^= local << 13
				local ^= local >> 7
				local ^= local << 17
				root := math.Sqrt(float64((local & 0xfffff) + uint64(iterations+1)))
				local += uint64(root*1000003) * 0x9e3779b185ebca87

				if iterations%4096 == 0 {
					atomic.AddUint64(&checksum, local)
				}
			}
			atomic.AddUint64(&checksum, local)
		}()
	}

	wg.Wait()
	return atomic.LoadUint64(&checksum)
}
