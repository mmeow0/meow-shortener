package audit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	// ActionShorten — событие создания или повторного получения короткой ссылки.
	ActionShorten = "shorten"
	// ActionFollow — переход по короткой ссылке (редирект).
	ActionFollow = "follow"
)

// Event описывает одно событие аудита (формат JSON для приёмников).
type Event struct {
	TS     int64  `json:"ts"`
	Action string `json:"action"`
	UserID string `json:"user_id,omitempty"`
	URL    string `json:"url"`
}

// Observer — наблюдатель в смысле паттерна Observer: получает событие и доставляет его в приёмник.
type Observer interface {
	Notify(event Event) error
}

// Publisher — субъект: хранит наблюдателей и рассылает им события.
type Publisher struct {
	observers []Observer
	logger    *zap.Logger
}

// NewPublisher создаёт издателя аудита; observers может быть пустым.
func NewPublisher(observers []Observer, logger *zap.Logger) *Publisher {
	return &Publisher{observers: observers, logger: logger}
}

// Publish асинхронно уведомляет всех наблюдателей.
func (p *Publisher) Publish(e Event) {
	if p == nil || len(p.observers) == 0 {
		return
	}
	for _, o := range p.observers {
		o := o
		go func() {
			if err := o.Notify(e); err != nil && p.logger != nil {
				p.logger.Warn("audit delivery failed", zap.Error(err))
			}
		}()
	}
}

// FileObserver дописывает JSON-событие в конец файла (новая строка).
type FileObserver struct {
	path string
	mu   sync.Mutex
}

// NewFileObserver возвращает наблюдателя, пишущего в указанный файл.
func NewFileObserver(path string) *FileObserver {
	return &FileObserver{path: path}
}

// Notify реализует Observer.
func (f *FileObserver) Notify(e Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	data, err := json.Marshal(e)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(f.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(append(data, '\n'))
	return err
}

// HTTPObserver отправляет событие POST на заданный URL.
type HTTPObserver struct {
	endpoint string
	client   *http.Client
}

// NewHTTPObserver возвращает наблюдателя для удалённого приёмника.
func NewHTTPObserver(endpoint string) *HTTPObserver {
	return &HTTPObserver{
		endpoint: endpoint,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Notify реализует Observer.
func (h *HTTPObserver) Notify(e Event) error {
	body, err := json.Marshal(e)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, h.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("audit remote status %s", resp.Status)
	}
	return nil
}
