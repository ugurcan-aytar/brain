package commands

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// withSignalCancel wraps ctx with a cancel that fires on SIGINT. The caller
// must defer the returned cleanup to unregister the handler and stop the
// goroutine — otherwise Ctrl+C in a later request would still cancel this
// one's context.
//
// Chat installs its own fine-grained handlers around the streaming segment,
// so this helper is only used by the one-shot commands (ask, search).
func withSignalCancel(parent context.Context) (context.Context, func()) {
	ctx, cancel := context.WithCancel(parent)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT)

	done := make(chan struct{})
	go func() {
		defer close(done)
		select {
		case <-ctx.Done():
		case <-sigs:
			cancel()
		}
	}()

	return ctx, func() {
		signal.Stop(sigs)
		cancel()
		<-done
	}
}
