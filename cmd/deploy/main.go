package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"monks.co/pkg/errlogger"
	"monks.co/pkg/sigctx"
)

func main() {
	startAt := time.Now()

	if err := run(); err != nil {
		errlogger.ReportPanic(err)
		fmt.Fprintln(os.Stderr, "failed after %s", time.Now().Sub(startAt))
		panic(err)
	}

	fmt.Fprintf(os.Stderr, "succeeded after %s\n", time.Now().Sub(startAt))
}

func run() error {
	ctx := sigctx.New()

	isRemote := flag.Bool("remote", false, "use remote builder")
	flag.Parse()

	if *isRemote {
		return remoteDeploy(ctx)
	} else {
		return localDeploy(ctx)
	}
}

func remoteDeploy(ctx context.Context) error {
	if err := execcmd(ctx, "fly", "deploy"); err != nil {
		return err
	}
	return nil
}

func localDeploy(ctx context.Context) error {
	version, err := bump()
	if err != nil {
		return err
	}

	tag := fmt.Sprintf("registry.fly.io/monks-go:monks-go-v%s", version)
	if err := execcmd(ctx, "docker", "build", "-t", tag, "--platform", "linux/amd64", "."); err != nil {
		return err
	}
	if err := execcmd(ctx, "docker", "push", tag); err != nil {
		return err
	}
	if err := execcmd(ctx, "fly", "deploy", "--image", tag); err != nil {
		return err
	}

	return nil
}

func execcmd(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stderr, cmd.Stdout = os.Stderr, os.Stdout
	return cmd.Run()
}

func bump() (string, error) {
	currentbs, err := os.ReadFile(".version")
	if err != nil {
		return "", err
	}
	currentint, err := strconv.ParseInt(strings.TrimSpace(string(currentbs)), 10, 64)
	if err != nil {
		return "", err
	}
	nextint := currentint + 1
	nextstr := fmt.Sprintf("%d", nextint)
	if err := os.WriteFile(".version", []byte(nextstr), 0644); err != nil {
		return "", err
	}
	return nextstr, nil
}
