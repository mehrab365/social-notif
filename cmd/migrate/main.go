package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"social-notif/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.RunMigrate(ctx); err != nil {
		panic(err)
	}
}
