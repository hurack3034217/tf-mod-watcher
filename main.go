package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/hurack3034217/tf-mod-watcher/pkg/cli"
)

func main() {
	if err := cli.NewApp(os.Stdout).Run(context.Background(), os.Args); err != nil {
		slog.Error("CLI execution failed", "error", err)
		os.Exit(1)
	}
}
