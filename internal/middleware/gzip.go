package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

// compressWriter реализует интерфейс http.ResponseWriter и позволяет прозрачно для сервера
// сжимать передаваемые данные и выставлять правильные HTTP-заголовки
type compressWriter struct {
	w        http.ResponseWriter
	zw       *gzip.Writer
	compress bool // флаг, указывающий нужно ли сжимать
}

func newCompressWriter(w http.ResponseWriter) *compressWriter {
	return &compressWriter{
		w:        w,
		compress: false,
	}
}

func (c *compressWriter) Header() http.Header {
	return c.w.Header()
}

func (c *compressWriter) Write(p []byte) (int, error) {
	if c.compress {
		return c.zw.Write(p)
	}
	return c.w.Write(p)
}

func (c *compressWriter) WriteHeader(statusCode int) {
	if statusCode >= 200 && statusCode < 300 {
		// Проверяем Content-Type для сжатия
		contentType := c.w.Header().Get("Content-Type")
		if shouldCompress(contentType) {
			c.w.Header().Set("Content-Encoding", "gzip")
			c.compress = true
			c.zw = gzip.NewWriter(c.w)
		}
	}
	c.w.WriteHeader(statusCode)
}

// shouldCompress проверяет, следует ли сжимать контент данного типа
func shouldCompress(contentType string) bool {
	return strings.Contains(contentType, "application/json") ||
		   strings.Contains(contentType, "text/html")
}

// Close закрывает gzip.Writer и досылает все данные из буфера.
func (c *compressWriter) Close() error {
	if c.compress && c.zw != nil {
		return c.zw.Close()
	}
	return nil
}

// compressReader реализует интерфейс io.ReadCloser и позволяет прозрачно для сервера
// декомпрессировать получаемые от клиента данные
type compressReader struct {
	r  io.ReadCloser
	zr *gzip.Reader
}

func newCompressReader(r io.ReadCloser) (*compressReader, error) {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}

	return &compressReader{
		r:  r,
		zr: zr,
	}, nil
}

func (c compressReader) Read(p []byte) (n int, err error) {
	return c.zr.Read(p)
}

func (c *compressReader) Close() error {
	if err := c.r.Close(); err != nil {
		return err
	}
	return c.zr.Close()
}

// GzipMiddleware возвращает middleware для поддержки gzip сжатия
func GzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// по умолчанию устанавливаем оригинальный http.ResponseWriter как тот,
		// который будем передавать следующей функции
		ow := w

		// проверяем, что клиент умеет получать от сервера сжатые данные в формате gzip
		acceptEncoding := r.Header.Get("Accept-Encoding")
		supportsGzip := strings.Contains(acceptEncoding, "gzip")
		if supportsGzip {
			// оборачиваем оригинальный http.ResponseWriter новым с поддержкой сжатия
			cw := newCompressWriter(w)
			// меняем оригинальный http.ResponseWriter на новый
			ow = cw
			// не забываем отправить клиенту все сжатые данные после завершения middleware
			defer cw.Close()
		}

		// проверяем, что клиент отправил серверу сжатые данные в формате gzip
		contentEncoding := r.Header.Get("Content-Encoding")
		sendsGzip := strings.Contains(contentEncoding, "gzip")
		if sendsGzip {
			// оборачиваем тело запроса в io.Reader с поддержкой декомпрессии
			cr, err := newCompressReader(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			// меняем тело запроса на новое
			r.Body = cr
			defer cr.Close()
		}

		// передаём управление следующему хендлеру
		next.ServeHTTP(ow, r)
	})
}
