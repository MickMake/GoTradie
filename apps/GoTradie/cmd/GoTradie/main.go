package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/MickMake/GoTradie/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(app.App{}.Run(ctx, os.Args[1:]))
}
