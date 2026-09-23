//go:build !windows

package main

import (
	"log"
)

func alert(title, msg string) {
	log.Printf("%s: %s", title, msg)
}

func fatalf(format string, args ...any) {
	log.Fatalf(format, args...)
}
