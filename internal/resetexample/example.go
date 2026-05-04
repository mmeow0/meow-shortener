package resetexample

// Inner — вспомогательная структура с собственным методом Reset().
// generate:reset
type Inner struct {
	Value int
	Label string
}

// generate:reset
type ResetableStruct struct {
	// примитивы
	I   int
	F   float64
	B   bool
	Str string

	// указатели на примитивы — сбрасываются по значению (не nil)
	StrP *string
	IP   *int

	// слайс — обрезается до нулевой длины, базовый массив сохраняется
	S []int

	// мап — очищается через clear
	M map[string]string

	// вложенная структура-значение с Reset() — вызывается Reset()
	Nested Inner

	// указатель на структуру с Reset() — вызывается Reset() если не nil
	Child *ResetableStruct

	// указатель на слайс
	SP *[]int

	// указатель на мап
	MP *map[string]string
}

// generate:reset
type Counter struct {
	Count   int64
	Name    string
	Tags    []string
	Buckets map[string]int
	Active  bool
	Factor  float64
}
