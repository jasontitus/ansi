package ansi

import (
	"bytes"
	"io"
	"unicode/utf8"
)

type Writer struct {
	Forward io.Writer

	ansi       bool
	ansiseq    bytes.Buffer
	lastseq    bytes.Buffer
	seqchanged bool
	runeBuf    []byte
	err        error
}

// Write is used to write content to the ANSI buffer.
// A forwarding error is retained; subsequent writes return the same error.
func (w *Writer) Write(b []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	sequenceStart := -w.ansiseq.Len()
	for i, c := range string(b) {
		if c == Marker {
			// ANSI escape sequence
			if w.ansiseq.Len() == 0 {
				sequenceStart = i
			}
			w.ansi = true
			w.seqchanged = true
			_, _ = w.ansiseq.WriteRune(c)
		} else if w.ansi {
			_, _ = w.ansiseq.WriteRune(c)
			if IsTerminator(c) {
				// ANSI sequence terminated
				w.ansi = false

				if bytes.HasSuffix(w.ansiseq.Bytes(), []byte("[0m")) {
					// reset sequence
					w.lastseq.Reset()
					w.seqchanged = false
				} else if c == 'm' {
					// color code
					_, _ = w.lastseq.Write(w.ansiseq.Bytes())
				}

				n, err := w.ansiseq.WriteTo(w.Forward)
				if err != nil {
					w.err = err
					written := sequenceStart + int(n)
					if written < 0 {
						written = 0
					}
					_, size := utf8.DecodeRune(b[i:])
					if written > i+size {
						written = i + size
					}
					return written, err
				}
			}
		} else {
			n, err := w.writeRune(c)
			if err != nil {
				w.err = err
				_, size := utf8.DecodeRune(b[i:])
				if n > size {
					n = size
				}
				return i + n, err
			}
		}
	}

	return len(b), nil
}

func (w *Writer) writeRune(r rune) (int, error) {
	if w.runeBuf == nil {
		w.runeBuf = make([]byte, utf8.UTFMax)
	}
	n := utf8.EncodeRune(w.runeBuf, r)
	written, err := w.Forward.Write(w.runeBuf[:n])
	if err == nil && written < n {
		err = io.ErrShortWrite
	}
	return written, err
}

func (w *Writer) LastSequence() string {
	return w.lastseq.String()
}

func (w *Writer) ResetAnsi() {
	if w.err != nil || !w.seqchanged {
		return
	}
	n, err := w.Forward.Write([]byte("\x1b[0m"))
	if err == nil && n < len("\x1b[0m") {
		err = io.ErrShortWrite
	}
	w.err = err
}

func (w *Writer) RestoreAnsi() {
	if w.err != nil {
		return
	}
	n, err := w.Forward.Write(w.lastseq.Bytes())
	if err == nil && n < w.lastseq.Len() {
		err = io.ErrShortWrite
	}
	w.err = err
}
