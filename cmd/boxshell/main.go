package main

import (
	"boxshell/internal/auth"
	"boxshell/internal/boxapi"
	"boxshell/internal/shell"
	"context"
	"fmt"
	"os"
)

func main() {
	ctx := context.Background()

	httpClient, err := auth.NewClient(ctx)
	if err != nil {
		fmt.Fprint(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	boxClient := boxapi.NewClient(httpClient)

	if err := shell.Run(ctx, boxClient); err != nil {
		fmt.Fprint(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
