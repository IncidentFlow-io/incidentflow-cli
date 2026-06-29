package main

import (
	"os"

	"github.com/incidentflow/incidentflow-cli/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
