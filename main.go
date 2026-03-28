package main

import (
	"os"

	"github.com/fingerprint/notetools/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
