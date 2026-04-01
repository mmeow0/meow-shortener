package repository

import (
	"fmt"
	"testing"

	"github.com/mmeow0/meow-shortener/internal/model"
)

// BenchmarkInMemorySave — запись в in-memory хранилище (новый short_id на итерацию).
func BenchmarkInMemorySave(b *testing.B) {
	r := NewInMemoryURLRepository()
	b.ReportAllocs()

	i := 0
	for b.Loop() {
		u := &model.URL{
			UUID:        fmt.Sprintf("00000000-0000-4000-8000-%012d", i),
			ShortURL:    fmt.Sprintf("s%07d", i),
			OriginalURL: "https://example.com/page",
			UserID:      "user-bench",
		}
		if err := r.Save(u); err != nil {
			b.Fatal(err)
		}
		i++
	}
}

// BenchmarkInMemoryFindByID — поиск по короткому идентификатору при заполненном хранилище.
func BenchmarkInMemoryFindByID(b *testing.B) {
	r := NewInMemoryURLRepository()
	for i := range 1000 {
		_ = r.Save(&model.URL{
			UUID:        fmt.Sprintf("00000000-0000-4000-8000-%012d", i),
			ShortURL:    fmt.Sprintf("k%05d", i),
			OriginalURL: "https://example.com/x",
			UserID:      "u1",
		})
	}
	target := "k00100"

	b.ReportAllocs()
	for b.Loop() {
		_, _ = r.FindByID(target)
	}
}
