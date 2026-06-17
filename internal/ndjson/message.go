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

	// Incoming (PTY only): write Text to the child's stdin.
	TypeInput = "input"

	// Incoming (PTY only): resize the PTY window to Rows x Cols.
	TypeResize = "resize"

	// Incoming (daemon only): create a new session.
	//   ID, Command, Options are set.
	TypeCreate = "create"

	// Incoming (daemon only): destroy an existing session.
	//   ID is set.
	TypeDestroy = "destroy"

	// Incoming (daemon only): list active sessions.
	TypeList = "list"

	// Outgoing (daemon only): emitted when a session is created.
	//   ID, PID are set.
	TypeCreated = "created"

	// Outgoing (daemon only): emitted when a session is destroyed.
	//   ID, ExitCode are set.
	TypeDestroyed = "destroyed"

	// Outgoing (daemon only): list of active sessions.
	//   Sessions is set.
	TypeSessions = "sessions"
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
//	Type=input     Text is set; the bytes are written to the
//	                child's stdin (PTY only).
//	Type=resize    Rows and Cols are set; the PTY window is resized.
//	Type=create    ID, Command are set. (daemon only)
//	Type=destroy   ID is set. (daemon only)
//	Type=list      (daemon only)
//
// Outgoing (daemon only):
//	Type=created   ID, PID are set.
//	Type=destroyed ID, ExitCode are set.
//	Type=sessions  Sessions is a list of {id, pid}.
//
// Numeric fields (PID, ExitCode, DurationMS, Rows, Cols) are
// pointers so a genuine 0 is distinguishable from "field not set".
// String fields use omitempty (the zero value is the empty string,
// which we treat as absent).
type Message struct {
	Type       string         `json:"type"`
	ID         string         `json:"id,omitempty"`
	Stream     string         `json:"stream,omitempty"`
	Text       string         `json:"text,omitempty"`
	Command    string         `json:"command,omitempty"`
	PID        *int           `json:"pid,omitempty"`
	ExitCode   *int           `json:"exit_code,omitempty"`
	DurationMS *int64         `json:"duration_ms,omitempty"`
	Err        string         `json:"err,omitempty"`
	Stdout     string         `json:"stdout,omitempty"`
	Stderr     string         `json:"stderr,omitempty"`
	Rows       *int           `json:"rows,omitempty"`
	Cols       *int           `json:"cols,omitempty"`
	Sessions   []SessionInfo  `json:"sessions,omitempty"`
}

// SessionInfo is a summary of an active session, used by the
// daemon's "list" response.
type SessionInfo struct {
	ID  string `json:"id"`
	PID int    `json:"pid"`
}
