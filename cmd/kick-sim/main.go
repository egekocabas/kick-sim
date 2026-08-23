package main

import (
	"os"

	"github.com/egekocabas/kick-sim/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
