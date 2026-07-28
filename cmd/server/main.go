package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"cpa-usage/internal/app"
)

func main() {
	envFile := flag.String("env", "", "path to env file")
	flag.Parse()

	application, err := app.NewWithOptions(app.Options{EnvFile: *envFile})
	if err != nil {
		log.Fatalf("initialize app: %v", err)
	}
	defer application.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := application.Run(ctx); err != nil {
		log.Printf("run app: %v", err)
		if closeErr := application.Close(); closeErr != nil {
			log.Printf("close app: %v", closeErr)
		}
		os.Exit(1)
	}
}
