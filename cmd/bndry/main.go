package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ofkm/bndry/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	bndryApp, err := app.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize bndry: %v\n", err)
		os.Exit(1)
	}

	if err := bndryApp.Run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "bndry: %v\n", err)
		os.Exit(1)
	}
}
