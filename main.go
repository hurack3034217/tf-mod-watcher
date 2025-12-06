package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/hurack/go-mod-watcher/pkg/cli"
)

func main() {
	if err := cli.NewApp().Run(context.Background(), os.Args); err != nil {
		slog.Error("CLI execution failed", "error", err)
		os.Exit(1)
	}
}
