// Package model задаёт JSON-структуры запросов и ответов HTTP API практического трека.
package model

// URL — сущность сокращённой ссылки в хранилище.
type URL struct {
	UUID        string `json:"uuid"`         // Уникальный идентификатор записи
	ShortURL    string `json:"short_url"`    // Короткий идентификатор
	OriginalURL string `json:"original_url"` // Оригинальный URL
	UserID      string `json:"user_id"`      // ID пользователя, создавшего URL
	IsDeleted   bool   `json:"is_deleted"`   // Флаг удаления (soft delete)
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

// BatchShortenRequest представляет элемент батч-запроса для сокращения URL
type BatchShortenRequest struct {
	CorrelationID string `json:"correlation_id"` // Идентификатор для связи запроса и ответа
	OriginalURL   string `json:"original_url"`   // URL для сокращения
}

// BatchShortenResponse представляет элемент батч-ответа с сокращённым URL
type BatchShortenResponse struct {
	CorrelationID string `json:"correlation_id"` // Идентификатор из запроса
	ShortURL      string `json:"short_url"`      // Результирующий сокращённый URL
}

// DeleteURLsRequest — тело DELETE /api/user/urls: JSON-массив коротких id или полных коротких URL.
type DeleteURLsRequest []string
