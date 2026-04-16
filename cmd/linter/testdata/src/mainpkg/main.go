package main

import (
	"log"
	"os"
)

// helper вызывает log.Fatal и os.Exit вне main — это запрещено.
func helper() {
	log.Fatal("error in helper")  // want `вызов log\.Fatal вне функции main пакета main`
	os.Exit(2)                    // want `вызов os\.Exit вне функции main пакета main`
}

// main является единственной разрешённой точкой для log.Fatal и os.Exit.
func main() {
	log.Fatal("fatal error") // OK
	os.Exit(1)               // OK
}
