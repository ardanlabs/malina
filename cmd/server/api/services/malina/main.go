package main

import (
	"context"
	"log"

	"github.com/ardanlabs/malina/cmd/server/api/services/malina/runner"
)

func main() {
	if err := runner.Run(context.Background(), runner.DefaultConfig()); err != nil {
		log.Fatal(err)
	}
}
