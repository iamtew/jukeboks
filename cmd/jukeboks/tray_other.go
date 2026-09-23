//go:build !windows

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func attachParentConsole() {}

func waitForQuit() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	stopHTTP()
}
