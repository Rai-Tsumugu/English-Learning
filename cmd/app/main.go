package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Rai-Tsumugu/English-Learning/internal/config"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	switch cmd {
	case "serve":
		if err := runServe(ctx, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "serve: %v\n", err)
			os.Exit(1)
		}
	case "migrate":
		if err := runMigrate(ctx, cfg, os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
			os.Exit(1)
		}
	case "ingest":
		if err := runIngest(ctx, cfg, os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "ingest: %v\n", err)
			os.Exit(1)
		}
	case "pregenerate":
		if err := runPregenerate(ctx, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "pregenerate: %v\n", err)
			os.Exit(1)
		}
	case "login":
		if err := runLogin(ctx, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "login: %v\n", err)
			os.Exit(1)
		}
	case "logout":
		if err := runLogout(ctx, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "logout: %v\n", err)
			os.Exit(1)
		}
	case "whoami":
		if err := runWhoami(ctx, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "whoami: %v\n", err)
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: app <serve|migrate|ingest|pregenerate|login|logout|whoami> [args...]")
}
