package main

import (
	"fmt"
	"os"

	"rika-collector/internal/collector"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <config-file>\n", os.Args[0])
		os.Exit(2)
	}

	os.Exit(collector.Run(os.Args[1]))
}
