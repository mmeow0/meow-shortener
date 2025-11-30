package model

// URL представляет сущность сокращённого URL
type URL struct {
	ID          string // Короткий идентификатор
	OriginalURL string // Оригинальный URL
}

// ShortenRequest представляет JSON запрос для сокращения URL
type ShortenRequest struct {
	URL string `json:"url"`
}

// ShortenResponse представляет JSON ответ с сокращённым URL
type ShortenResponse struct {
	Result string `json:"result"`
}

