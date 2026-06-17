package ndjson

import (
	"bytes"
	"io"
	"testing"
)

func TestEncoderDecoderRoundTrip(t *testing.T) {
	i := func(v int) *int { return &v }
	i64 := func(v int64) *int64 { return &v }
	cases := []Message{
		{Type: TypeStarted, PID: i(1234)},
		{Type: TypeLine, Stream: "stdout", Text: "hello"},
		{Type: TypeLine, Stream: "stderr", Text: "warning"},
		{Type: TypeExit, ExitCode: i(0), DurationMS: i64(1234)},
		{Type: TypeExit, ExitCode: i(1), Err: "command exited with code 1"},
		{Type: TypeError, Err: "failed to start"},
		{Type: TypeStop},
	}

	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	for i := range cases {
		if err := enc.Encode(&cases[i]); err != nil {
			t.Fatalf("encode %d: %v", i, err)
		}
	}

	dec := NewDecoder(&buf)
	for i := range cases {
		got, err := dec.Decode()
		if err != nil {
			t.Fatalf("decode %d: %v", i, err)
		}
		if got.Type != cases[i].Type {
			t.Errorf("case %d: type = %q, want %q", i, got.Type, cases[i].Type)
		}
		if !ptrEq(got.PID, cases[i].PID) {
			t.Errorf("case %d: pid mismatch", i)
		}
		if got.Stream != cases[i].Stream {
			t.Errorf("case %d: stream = %q, want %q", i, got.Stream, cases[i].Stream)
		}
		if got.Text != cases[i].Text {
			t.Errorf("case %d: text = %q, want %q", i, got.Text, cases[i].Text)
		}
		if !ptrEq(got.ExitCode, cases[i].ExitCode) {
			t.Errorf("case %d: exit_code mismatch", i)
		}
		if !ptrEq64(got.DurationMS, cases[i].DurationMS) {
			t.Errorf("case %d: duration_ms mismatch", i)
		}
		if got.Err != cases[i].Err {
			t.Errorf("case %d: err = %q, want %q", i, got.Err, cases[i].Err)
		}
	}
}

func ptrEq(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func ptrEq64(a, b *int64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func TestDecoderEOF(t *testing.T) {
	dec := NewDecoder(bytes.NewReader(nil))
	_, err := dec.Decode()
	if err != io.EOF {
		t.Errorf("expected io.EOF, got: %v", err)
	}
}

func TestDecoderInvalidJSON(t *testing.T) {
	dec := NewDecoder(bytes.NewReader([]byte("not json\n")))
	_, err := dec.Decode()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestEncoderOmitsEmptyFields(t *testing.T) {
	i := func(v int) *int { return &v }
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.Encode(&Message{Type: TypeStarted, PID: i(42)}); err != nil {
		t.Fatalf("encode: %v", err)
	}
	got := buf.String()
	// Should not contain "stream", "text", "exit_code", etc.
	for _, field := range []string{`"stream"`, `"text"`, `"exit_code"`, `"duration_ms"`, `"err"`, `"stdout"`, `"stderr"`} {
		if bytes.Contains([]byte(got), []byte(field)) {
			t.Errorf("expected %s to be omitted, got: %s", field, got)
		}
	}
	if !bytes.Contains([]byte(got), []byte(`"type":"started"`)) {
		t.Errorf("expected type:started in output, got: %s", got)
	}
	if !bytes.Contains([]byte(got), []byte(`"pid":42`)) {
		t.Errorf("expected pid:42 in output, got: %s", got)
	}
}
