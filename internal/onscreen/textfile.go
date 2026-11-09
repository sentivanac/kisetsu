package onscreen

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Text from a file (ADR 0051): Kisetsu shows the content of one text file
// the user chose, and never runs anything. Whatever writes the file (the
// user's own cron job, a script) is outside Kisetsu. The file is read with
// guards, because its content ends up on the desktop:
//
//   - only a regular file, also behind a symlink, so a FIFO or a device
//     cannot block or flood the agent;
//   - on Linux, only a file the user owns that no one else can write, so
//     another user cannot put text on this user's desktop;
//   - at most maxTextBytes, valid UTF-8;
//   - control characters (also terminal color codes) and characters that
//     change the direction of text are removed, so the text shows as it
//     is;
//   - at most maxTextLines lines; the composer shortens long ones.
const (
	maxTextBytes = 4096
	maxTextLines = 10
)

// textFileProblem is a reason not to show the file's content; the
// wallpaper shows the message instead.
type textFileProblem struct{ msg string }

func (p *textFileProblem) Error() string { return p.msg }

func problem(format string, args ...any) error {
	return &textFileProblem{msg: fmt.Sprintf(format, args...)}
}

// readTextFile returns the lines to draw from the file at path.
func readTextFile(path string) ([]string, error) {
	if path == "" {
		return nil, problem("No text file chosen")
	}
	f, err := openText(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, problem("Text file not found")
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, problem("Not a text file")
	}
	if err := checkOwner(fi); err != nil {
		return nil, err
	}
	b, err := io.ReadAll(io.LimitReader(f, maxTextBytes))
	if err != nil {
		return nil, err
	}
	if len(b) == maxTextBytes {
		// The file goes on; drop what may be half a character or line.
		if i := strings.LastIndexByte(string(b), '\n'); i >= 0 {
			b = b[:i]
		}
	}
	if !utf8.Valid(b) {
		return nil, problem("The text file is not UTF-8 text")
	}
	return textLines(string(b)), nil
}

// textLines cleans the text and splits it into at most maxTextLines lines,
// without empty lines at the start and end.
func textLines(s string) []string {
	s = clean(s)
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	for len(lines) > 0 && lines[0] == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > maxTextLines {
		lines = lines[:maxTextLines]
	}
	return lines
}

// clean removes terminal escape sequences, control characters and
// characters that change the direction of text, and turns tabs into
// spaces and Windows line ends into plain ones.
func clean(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == '\x1b':
			i += escapeLength(s[i:])
			continue
		case r == '\n':
			b.WriteByte('\n')
		case r == '\t':
			b.WriteString("    ")
		case unicode.IsControl(r), isDirectional(r), r == 0xfeff:
			// Dropped, \r among them.
		default:
			b.WriteRune(r)
		}
		i += size
	}
	return b.String()
}

// escapeLength is the length of the terminal escape sequence at the start
// of s, such as "\x1b[1;31m": ESC, "[", parameters, and a final letter.
// Other escapes drop ESC and the character after it.
func escapeLength(s string) int {
	if len(s) < 2 {
		return len(s)
	}
	if s[1] != '[' {
		return 2
	}
	for i := 2; i < len(s); i++ {
		if c := s[i]; c >= 0x40 && c <= 0x7e {
			return i + 1
		}
	}
	return len(s)
}

// isDirectional reports the Unicode characters that reorder text, which
// can make it look different from what it is.
func isDirectional(r rune) bool {
	switch {
	case r == 0x061c, r == 0x200e, r == 0x200f: // Arabic letter mark, LRM, RLM
		return true
	case r >= 0x202a && r <= 0x202e: // embeddings and overrides
		return true
	case r >= 0x2066 && r <= 0x2069: // isolates
		return true
	}
	return false
}

// openText opens path for reading without blocking on a FIFO.
func openText(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|openNonBlock, 0)
}
