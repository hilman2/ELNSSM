package main

import (
	"os"

	"github.com/hilman2/ELNSSM/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
