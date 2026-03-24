package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"monks.co/apps/tagtime/node"
	"monks.co/pkg/database"
	"monks.co/pkg/env"
	"monks.co/pkg/gzip"
	"monks.co/pkg/meta"
	"monks.co/pkg/reqlog"
	"monks.co/pkg/serve"
	"monks.co/pkg/sigctx"
	"monks.co/pkg/tailnet"
)

func main() {
	if err := run(); err != nil {
		if !errors.Is(err, context.Canceled) {
			slog.Error("fatal", "error", err.Error(), "app.name", meta.AppName())
		}
		reqlog.Shutdown()
		os.Exit(1)
	}
}

func run() error {
	reqlog.SetupLogging()

	ctx := sigctx.New()

	config, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	if config.DBPath == "" {
		config.DBPath = env.InMonksData("tagtime.db")
	}

	if err := tailnet.WaitReady(ctx); err != nil {
		return fmt.Errorf("tailnet: %w", err)
	}

	repl, err := database.StartReplication(ctx, config.DBPath)
	if err != nil {
		return fmt.Errorf("starting replication: %w", err)
	}
	defer repl.Close()

	n, err := node.NewNode(ctx, config)
	if err != nil {
		return fmt.Errorf("creating node: %w", err)
	}
	defer n.Close()

	mux := serve.NewMux()
	mux.Handle("/", n.Handler())

	if err := tailnet.ListenAndServe(ctx, reqlog.Middleware().ModifyHandler(gzip.Middleware(mux))); err != nil {
		return err
	}

	return nil
}

func loadConfig() (node.Config, error) {
	configPath := os.Getenv("TAGTIME_CONFIG")
	if configPath == "" {
		configPath = env.InMonksData("tagtime.json")
	}
	configData, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		slog.Info("no config file found, using defaults", "path", configPath)
		return node.DefaultConfig(), nil
	}
	if err != nil {
		return node.Config{}, fmt.Errorf("reading config %s: %w", configPath, err)
	}
	config, err := node.ParseConfig(configData)
	if err != nil {
		return node.Config{}, fmt.Errorf("parsing config: %w", err)
	}
	return config, nil
}
