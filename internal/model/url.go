package model

// URL представляет сущность сокращённого URL
type URL struct {
	UUID        string `json:"uuid"`         // Уникальный идентификатор записи
	ShortURL    string `json:"short_url"`    // Короткий идентификатор
	OriginalURL string `json:"original_url"` // Оригинальный URL
	UserID      string `json:"user_id"`      // ID пользователя, создавшего URL
}

// ShortenRequest представляет JSON запрос для сокращения URL
type ShortenRequest struct {
	URL string `json:"url"`
}

// ShortenResponse представляет JSON ответ с сокращённым URL
type ShortenResponse struct {
	Result string `json:"result"`
}

// UserURLsResponse представляет элемент списка URL пользователя
type UserURLsResponse struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}
