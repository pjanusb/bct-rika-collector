package main

import (
	"fmt"
	"os"

	"rika-collector/internal/collector"
)

var version = "unknown"

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println(version)
		return
	}

	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <config-file>\n       %s --version|-v\n", os.Args[0], os.Args[0])
		os.Exit(2)
	}

	os.Exit(collector.Run(os.Args[1], version))
}
