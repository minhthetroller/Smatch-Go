package dto

// AutocompleteSuggestion sent to Flutter
type AutocompleteSuggestion struct {
	ID    string  `json:"id"`
	Text  string  `json:"text"`
	Score float64 `json:"score"`
}

// AutocompleteResponse sent to Flutter
type AutocompleteResponse struct {
	Suggestions []AutocompleteSuggestion `json:"suggestions"`
}

// PopularSearchesResponse sent to Flutter
type PopularSearchesResponse struct {
	Queries []string `json:"queries"`
}
