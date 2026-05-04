package withpanic

// Foo демонстрирует запрещённое использование panic.
func Foo() {
	panic("something went wrong") // want `использование встроенной функции panic`
}

// Bar также использует panic с нестроковым аргументом.
func Bar(err error) {
	panic(err) // want `использование встроенной функции panic`
}
