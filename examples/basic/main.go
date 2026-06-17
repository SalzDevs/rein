// Example: basic — run a one-shot command and print the result.
//
// Usage: go run ./examples/basic
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/SalzDevs/rein"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := rein.Run(ctx, "echo hello from rein && date",
		rein.WithTimeout(10*time.Second),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	fmt.Printf("exit code:    %d\n", result.ExitCode)
	fmt.Printf("duration:     %s\n", result.Duration)
	fmt.Printf("stdout:       %q\n", result.Stdout)
	fmt.Printf("stderr:       %q\n", result.Stderr)

	if result.Err != nil {
		fmt.Printf("error:        %v\n", result.Err)
	}
}
