package main

import (
	"fmt"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../.."); err != nil {
		fmt.Fprintf(os.Stderr, "use Daptin repository root: %v\n", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}
