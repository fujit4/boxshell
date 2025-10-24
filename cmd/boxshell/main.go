package main

import (
	"boxshell/internal/auth"
	"boxshell/internal/boxapi"
	"fmt"

	"boxshell/internal/shell"
	"context"
	"log"
)

func main() {
	if err := run(); err !=nil {
		log.Fatalln(err)
	}
}

func run() error {
	ctx := context.Background()

	authedClient, err := auth.NewClient(ctx)
	if err != nil {
		return err
	}

	boxClient := boxapi.NewClient(authedClient)
	if err := shell.Run(ctx, boxClient); err != nil {
		return err
	}

	return nil
}
