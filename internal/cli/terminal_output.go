package cli

import (
	"io"
	"unicode"
	"unicode/utf8"
)

type terminalSafeWriter struct{ io.Writer }

func (w terminalSafeWriter) Write(p []byte) (int, error) {
	safe := append([]byte(nil), p...)
	for i := 0; i < len(safe); {
		r, size := utf8.DecodeRune(safe[i:])
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			for j := 0; j < size; j++ {
				safe[i+j] = ' '
			}
		}
		i += size
	}
	return w.Writer.Write(safe)
}
