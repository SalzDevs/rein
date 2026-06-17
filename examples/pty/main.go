// Example: pty — run an interactive command (here, a small
// shell session that prompts for input) and drive it via PTY.
//
// This example sends a question to a tiny awk script and prints
// the response. In a real use case, this pattern is how an
// agent would respond to sudo's password prompt, ssh-add's
// passphrase prompt, or any other interactive CLI.
//
// Usage: go run ./examples/pty
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/SalzDevs/rein"
)

func main() {
	// The command asks for input and prints it back. The PTY
	// is required so the command sees a TTY.
	command := `awk 'BEGIN { printf "What is your name? "; getline name < "/dev/stdin"; printf "Hello, %s!\n", name }'`

	session, err := rein.Start(nil, command,
		rein.WithPTY(),
		rein.WithTimeout(5*time.Second),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "start failed:", err)
		os.Exit(1)
	}
	defer session.Stop()

	// Read the prompt.
	lines := readNLines(session.Lines(), 1, 2*time.Second)
	for _, l := range lines {
		fmt.Print("agent saw: ", l.Text)
	}

	// Send the answer.
	if _, err := session.Write([]byte("rein\n")); err != nil {
		fmt.Fprintln(os.Stderr, "write failed:", err)
		os.Exit(1)
	}

	// Read the response.
	lines = readNLines(session.Lines(), 1, 2*time.Second)
	for _, l := range lines {
		fmt.Println("agent saw:", l.Text)
	}
}

func readNLines(ch <-chan rein.Line, n int, timeout time.Duration) []rein.Line {
	out := make([]rein.Line, 0, n)
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for i := 0; i < n; i++ {
		select {
		case l, ok := <-ch:
			if !ok {
				return out
			}
			out = append(out, l)
		case <-timer.C:
			return out
		}
	}
	return out
}
