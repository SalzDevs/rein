// Example: overflow — start a process that produces more output
// than the buffer can hold, and watch rein drop old lines while
// keeping the most recent output.
//
// Usage: go run ./examples/overflow
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/SalzDevs/rein"
)

func main() {
	// Produce 200 lines rapidly. With a buffer of 8 and the
	// DropOldest policy, the consumer will see the last 8
	// lines (approximately) while the older ones are dropped.
	session, err := rein.Start(context.Background(),
		`for i in $(seq 1 200); do echo "line-$i"; done`,
		rein.WithLineBuffer(8),
		rein.WithOverflowPolicy(rein.PolicyDropOldest),
		rein.WithTimeout(10*time.Second),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "start failed:", err)
		os.Exit(1)
	}
	defer session.Stop()

	// Drain the channel.
	for line := range session.Lines() {
		fmt.Println(line.Text)
	}

	// Wait for the process to finish.
	session.Wait()

	// Report how many lines were dropped.
	dropped := session.Drops()
	fmt.Fprintf(os.Stderr, "\n%d lines were dropped due to overflow\n", dropped)
}
