package dto

type LoadTestStressResponse struct {
	DurationMS int    `json:"durationMs"`
	Workers    int    `json:"workers"`
	ElapsedMS  int64  `json:"elapsedMs"`
	Checksum   uint64 `json:"checksum"`
}
