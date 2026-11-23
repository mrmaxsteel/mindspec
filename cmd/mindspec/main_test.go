package main

import (
	"fmt"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	code := m.Run()
	if mindspecTestBinary.dir != "" {
		if err := os.RemoveAll(mindspecTestBinary.dir); err != nil {
			fmt.Fprintf(os.Stderr, "remove mindspec test binary: %v\n", err)
			if code == 0 {
				code = 1
			}
		}
	}
	os.Exit(code)
}
