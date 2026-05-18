package router_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/mmeow0/meow-shortener/internal/facade"
	"github.com/mmeow0/meow-shortener/internal/handler"
	"github.com/mmeow0/meow-shortener/internal/model"
	"github.com/mmeow0/meow-shortener/internal/repository"
	"github.com/mmeow0/meow-shortener/internal/router"
	"github.com/mmeow0/meow-shortener/internal/service"
	"go.uber.org/zap"
)

// newPracticumServer поднимает тестовый сервер с тем же роутером, что и в практическом треке.
// baseURL совпадает с префиксом коротких ссылок в ответах.
func newPracticumServer() (*httptest.Server, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	baseURL := "http://" + ln.Addr().String()

	log := zap.NewNop()
	repo := repository.NewInMemoryURLRepository()
	svc := service.NewURLService(repo)
	urlH := handler.NewURLHandler(facade.NewURLFacade(svc, baseURL, nil), "", log)
	pingH := handler.NewPingHandler(nil, log)
	mux := router.NewRouter(urlH, pingH, "example-secret-for-docs", log)

	srv := &httptest.Server{
		Listener: ln,
		Config:   &http.Server{Handler: mux},
	}
	srv.Start()
	return srv, nil
}

// Пример: POST / с телом text/plain — сокращение ссылки.
func ExampleNewRouter_shortenPlain() {
	srv, err := newPracticumServer()
	if err != nil {
		panic(err)
	}
	defer srv.Close()

	resp, err := srv.Client().Post(srv.URL+"/", "text/plain", strings.NewReader("https://example.com/doc"))
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	fmt.Println(resp.StatusCode)
	// Output:
	// 201
}

// Пример: POST /api/shorten с JSON — сокращение и ответ {"result": "..."}.
func ExampleNewRouter_shortenJSON() {
	srv, err := newPracticumServer()
	if err != nil {
		panic(err)
	}
	defer srv.Close()

	body := strings.NewReader(`{"url":"https://practicum.yandex.ru/"}`)
	resp, err := srv.Client().Post(srv.URL+"/api/shorten", "application/json", body)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	fmt.Println(resp.StatusCode)
	var out model.ShortenResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		panic(err)
	}
	fmt.Println(strings.HasPrefix(out.Result, srv.URL+"/"))
	// Output:
	// 201
	// true
}

// Пример: GET /{id} — редирект на оригинальный URL (307 и заголовок Location).
func ExampleNewRouter_redirect() {
	srv, err := newPracticumServer()
	if err != nil {
		panic(err)
	}
	defer srv.Close()

	create, err := srv.Client().Post(srv.URL+"/", "text/plain", strings.NewReader("https://redirect.example/path"))
	if err != nil {
		panic(err)
	}
	defer create.Body.Close()
	shortFull, err := io.ReadAll(create.Body)
	if err != nil {
		panic(err)
	}
	shortID := strings.TrimPrefix(string(shortFull), srv.URL+"/")

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/"+shortID, nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Accept-Encoding", "identity")

	client := srv.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	fmt.Println(resp.StatusCode)
	fmt.Println(resp.Header.Get("Location"))
	// Output:
	// 307
	// https://redirect.example/path
}

// Пример: GET /api/user/urls — список ссылок пользователя (нужна одна и та же cookie jar).
func ExampleNewRouter_userURLs() {
	srv, err := newPracticumServer()
	if err != nil {
		panic(err)
	}
	defer srv.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		panic(err)
	}
	client := &http.Client{Jar: jar}

	shortenResp, err := client.Post(srv.URL+"/api/shorten", "application/json", strings.NewReader(`{"url":"https://a.example"}`))
	if err != nil {
		panic(err)
	}
	defer shortenResp.Body.Close()
	_, _ = io.Copy(io.Discard, shortenResp.Body)

	resp, err := client.Get(srv.URL + "/api/user/urls")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	fmt.Println(resp.StatusCode)
	var list []model.UserURLsResponse
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		panic(err)
	}
	fmt.Println(len(list) >= 1)
	// Output:
	// 200
	// true
}

// Пример: POST /api/shorten/batch — пакетное сокращение.
func ExampleNewRouter_batchShorten() {
	srv, err := newPracticumServer()
	if err != nil {
		panic(err)
	}
	defer srv.Close()

	batch := []model.BatchShortenRequest{
		{CorrelationID: "1", OriginalURL: "https://one.example"},
		{CorrelationID: "2", OriginalURL: "https://two.example"},
	}
	raw, err := json.Marshal(batch)
	if err != nil {
		panic(err)
	}

	resp, err := srv.Client().Post(srv.URL+"/api/shorten/batch", "application/json", bytes.NewReader(raw))
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	fmt.Println(resp.StatusCode)
	var out []model.BatchShortenResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		panic(err)
	}
	fmt.Println(len(out))
	// Output:
	// 201
	// 2
}

// Пример: DELETE /api/user/urls — постановка ссылок на удаление (202 Accepted).
func ExampleNewRouter_deleteUserURLs() {
	srv, err := newPracticumServer()
	if err != nil {
		panic(err)
	}
	defer srv.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		panic(err)
	}
	client := &http.Client{Jar: jar}

	cr, err := client.Post(srv.URL+"/api/shorten", "application/json", strings.NewReader(`{"url":"https://del.example"}`))
	if err != nil {
		panic(err)
	}
	defer cr.Body.Close()
	var sr model.ShortenResponse
	if err := json.NewDecoder(cr.Body).Decode(&sr); err != nil {
		panic(err)
	}

	shortID := strings.TrimPrefix(sr.Result, srv.URL+"/")
	delBody := fmt.Sprintf(`["%s"]`, shortID)
	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/api/user/urls", strings.NewReader(delBody))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	fmt.Println(resp.StatusCode)

	// Асинхронное удаление: даём фоновой обработке время завершиться перед проверкой редиректа.
	time.Sleep(50 * time.Millisecond)

	get, err := client.Get(srv.URL + "/" + shortID)
	if err != nil {
		panic(err)
	}
	defer get.Body.Close()
	_, _ = io.Copy(io.Discard, get.Body)
	fmt.Println(get.StatusCode)
	// Output:
	// 202
	// 410
}
