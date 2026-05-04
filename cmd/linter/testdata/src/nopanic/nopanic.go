package nopanic

import "errors"

// Foo возвращает ошибку вместо вызова panic.
func Foo() error {
	return errors.New("something went wrong")
}
