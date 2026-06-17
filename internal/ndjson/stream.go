package ndjson

import (
	"bufio"
	"encoding/json"
	"io"
)

// Encoder writes NDJSON messages to an underlying writer. Each
// Encode call writes one JSON object followed by a newline.
type Encoder struct {
	w   io.Writer
	enc *json.Encoder
}

// NewEncoder returns a new Encoder that writes to w.
func NewEncoder(w io.Writer) *Encoder {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return &Encoder{w: w, enc: enc}
}

// Encode writes m as a single NDJSON frame (one line of JSON
// followed by a newline).
func (e *Encoder) Encode(m *Message) error {
	return e.enc.Encode(m)
}

// Decoder reads NDJSON messages from an underlying reader, one
// per line.
type Decoder struct {
	sc *bufio.Scanner
}

// NewDecoder returns a new Decoder that reads from r.
//
// The default line length limit is 16 MB. Long lines (rare in
// practice — NDJSON messages should be small) are rejected.
func NewDecoder(r io.Reader) *Decoder {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
	return &Decoder{sc: sc}
}

// Decode reads the next NDJSON message. Returns io.EOF when the
// underlying reader returns io.EOF (e.g. the writer closed the
// stream).
func (d *Decoder) Decode() (*Message, error) {
	if !d.sc.Scan() {
		if err := d.sc.Err(); err != nil {
			return nil, err
		}
		return nil, io.EOF
	}
	m := &Message{}
	if err := json.Unmarshal(d.sc.Bytes(), m); err != nil {
		return nil, err
	}
	return m, nil
}
