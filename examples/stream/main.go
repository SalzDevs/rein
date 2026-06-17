// Example: stream — start a long-running command and stream its
// output line-by-line, with an idle timeout.
//
// Usage: go run ./examples/stream
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SalzDevs/rein"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// SIGINT (Ctrl-C) cancels the context.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Fprintln(os.Stderr, "\ninterrupted, stopping...")
		cancel()
	}()

	session, err := rein.Start(ctx, "bash -c 'for i in 1 2 3 4 5; do echo tick $i; sleep 0.3; done; sleep 60'",
		rein.WithIdleTimeout(2*time.Second), // kill if no output for 2s
		rein.WithGracefulTimeout(500*time.Millisecond),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "start failed:", err)
		os.Exit(1)
	}
	defer session.Stop()

	fmt.Println("--- streaming output (Ctrl-C to stop) ---")
	for line := range session.Lines() {
		fmt.Printf("[%s] %s\n", line.Stream, line.Text)
	}
	fmt.Println("--- stream closed ---")

	result, _ := session.Wait()
	fmt.Printf("\nexit code: %d\n", result.ExitCode)
	if result.Err != nil {
		fmt.Printf("error: %v\n", result.Err)
	}
}
