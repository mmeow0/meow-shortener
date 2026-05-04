package pool_test

import (
	"testing"

	"github.com/mmeow0/meow-shortener/pkg/pool"
)

// item — тестовый тип с методом Reset().
type item struct {
	value  int
	labels []string
	data   map[string]string
}

func (it *item) Reset() {
	it.value = 0
	it.labels = it.labels[:0]
	clear(it.data)
}

func TestPool_GetPut(t *testing.T) {
	p := pool.New(func() *item {
		return &item{
			labels: make([]string, 0, 8),
			data:   make(map[string]string),
		}
	})

	// Получаем объект, заполняем и возвращаем в пул.
	obj := p.Get()
	obj.value = 42
	obj.labels = append(obj.labels, "a", "b")
	obj.data["key"] = "value"
	p.Put(obj)

	// Следующий Get должен вернуть тот же объект, но уже сброшенный.
	obj2 := p.Get()
	if obj2.value != 0 {
		t.Errorf("ожидалось value=0, получено %d", obj2.value)
	}
	if len(obj2.labels) != 0 {
		t.Errorf("ожидалось len(labels)=0, получено %d", len(obj2.labels))
	}
	if len(obj2.data) != 0 {
		t.Errorf("ожидалось len(data)=0, получено %d", len(obj2.data))
	}
}

func TestPool_ResetCalledOnPut(t *testing.T) {
	resetCount := 0

	type tracked struct {
		onReset func()
	}
	_ = resetCount

	// Убеждаемся, что Put вызывает Reset.
	p := pool.New(func() *item { return &item{data: make(map[string]string)} })

	obj := p.Get()
	obj.value = 99
	p.Put(obj)

	got := p.Get()
	if got.value != 0 {
		t.Errorf("Reset не был вызван при Put: value=%d", got.value)
	}
}

func BenchmarkPool(b *testing.B) {
	p := pool.New(func() *item {
		return &item{
			labels: make([]string, 0, 8),
			data:   make(map[string]string),
		}
	})

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			obj := p.Get()
			obj.value = 1
			obj.labels = append(obj.labels, "x")
			p.Put(obj)
		}
	})
}
