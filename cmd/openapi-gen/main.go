package main

import (
	"flag"
	"fmt"
	"os"

	kickopenapi "github.com/egekocabas/kick-sim/internal/openapi"
)

func main() {
	output := flag.String("output", "web/openapi.json", "OpenAPI output path")
	flag.Parse()
	data, err := kickopenapi.Generate()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(*output, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write OpenAPI document: %v\n", err)
		os.Exit(1)
	}
}
