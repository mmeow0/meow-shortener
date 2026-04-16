package pool

import "sync"

// Resettable — ограничение для типов, обладающих методом Reset().
type Resettable interface {
	Reset()
}

// Pool — потокобезопасный пул объектов типа T.
// T должен реализовывать интерфейс Resettable, то есть обладать методом Reset().
type Pool[T Resettable] struct {
	p sync.Pool
}

// New создаёт и возвращает указатель на Pool.
// newFunc — фабричная функция, вызываемая пулом для создания нового объекта,
// когда в пуле не осталось свободных экземпляров.
func New[T Resettable](newFunc func() T) *Pool[T] {
	return &Pool[T]{
		p: sync.Pool{
			New: func() any {
				return newFunc()
			},
		},
	}
}

// Get возвращает объект из пула.
// Если пул пуст, вызывается фабричная функция, переданная в New.
func (p *Pool[T]) Get() T {
	return p.p.Get().(T)
}

// Put сбрасывает состояние объекта через Reset() и возвращает его в пул.
func (p *Pool[T]) Put(v T) {
	v.Reset()
	p.p.Put(v)
}
