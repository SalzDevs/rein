// Package ndjson implements the wire protocol used by the rein CLI.
//
// Messages are newline-delimited JSON objects, one per line, read
// from stdin and written to stdout. The protocol is deliberately
// minimal so any language with a JSON parser and a subprocess API
// can use it.
package ndjson

// Message types. The "type" field discriminates the message kind
// in both directions (incoming and outgoing).
const (
	// Outgoing: emitted when the process has been started.
	TypeStarted = "started"

	// Outgoing: emitted for each line of process output.
	TypeLine = "line"

	// Outgoing: emitted when the process has exited.
	TypeExit = "exit"

	// Outgoing: emitted on error before the process started.
	TypeError = "error"

	// Incoming: ask rein to stop the running process.
	TypeStop = "stop"
)

// Message is a single NDJSON frame in the rein protocol.
//
// Fields are populated according to the Type:
//
//	Type=started   PID is set.
//	Type=line      Stream ("stdout" or "stderr") and Text are set.
//	Type=exit      ExitCode, DurationMS, Stdout, Stderr, Err are set.
//	Type=error     Err is set.
//	Type=stop      No fields; signals that the caller wants the
//	                process stopped.
//
// Numeric fields (PID, ExitCode, DurationMS) are pointers so a
// genuine 0 is distinguishable from "field not set". String
// fields use omitempty (the zero value is the empty string, which
// we treat as absent).
type Message struct {
	Type       string `json:"type"`
	Stream     string `json:"stream,omitempty"`
	Text       string `json:"text,omitempty"`
	PID        *int   `json:"pid,omitempty"`
	ExitCode   *int   `json:"exit_code,omitempty"`
	DurationMS *int64 `json:"duration_ms,omitempty"`
	Err        string `json:"err,omitempty"`
	Stdout     string `json:"stdout,omitempty"`
	Stderr     string `json:"stderr,omitempty"`
}
