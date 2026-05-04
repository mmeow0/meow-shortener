package exitoutsidemain

import (
	"log"
	"os"
)

// Shutdown вызывает log.Fatal и os.Exit вне пакета main — запрещено.
func Shutdown(err error) {
	log.Fatal(err) // want `вызов log\.Fatal вне функции main пакета main`
	os.Exit(1)     // want `вызов os\.Exit вне функции main пакета main`
}

// Warn демонстрирует запрещённый вызов log.Fatalf.
func Warn(msg string) {
	log.Fatalf("fatal: %s", msg) // want `вызов log\.Fatalf вне функции main пакета main`
}
