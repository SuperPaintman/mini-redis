package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
)

func main() {
	f, err := os.OpenFile("cmd/radish-cli/main.go", os.O_RDONLY, 0)
	if err != nil {
		log.Fatalf("Could not open a file: %s", err)
	}
	defer f.Close()

	parse := NewFileParser(f)

	file, err := parse.Parse()
	if err != nil {
		log.Fatalf("Could not parse the file: %s", err)
	}

	file.Enable("Redis Protocol")
	file.Enable("radish-cli")
	file.Enable("radish-cli-writer")
	file.Enable("radish-cli-import-radish")
	file.Enable("radish-cli-readall")
	file.Enable("radish-cli-import-ioutil")
	file.Enable("radish-cli-reader")
	file.Enable("radish-cli-import-ioutil-remove")
	file.Enable("radish-cli-read-response")
	file.Enable("radish-cli-read-response-array")
	file.Enable("radish-cli-read-response-array-import-math")
	file.Enable("radish-cli-read-response-array-import-str")

	start, end, ok := file.SnippetLines("radish-cli-read-response-array")
	fmt.Printf("start = %d | end = %d | ok = %v\n", start, end, ok)

	var source bytes.Buffer
	var lineID int
	for _, line := range file.lines {
		if line.endTag {
			continue
		}

		if line.tag != "" {
			if tag, ok := file.tags[line.tag]; !ok || !tag.Enabled() {
				continue
			}
		}

		if ok && lineID >= start && lineID < end {
			fmt.Fprintf(&source, "%4d > ", lineID+1)
		} else {
			fmt.Fprintf(&source, "%4d | ", lineID+1)
		}
		lineID++

		_, _ = source.Write(line.text)
	}
	res := source.Bytes()

	// res, err = format.Source(res)
	// if err != nil {
	// 	log.Fatalf("Could not format the source code: %s", err)
	// }

	fmt.Printf("%s", res)

}

type tag interface {
	Name() string
	Enabled() bool
}

type tagState struct {
	enabled bool
	name    string
}

func (t *tagState) Name() string { return t.name }

func (t *tagState) Enabled() bool { return t.enabled }

type tagSnippet struct {
	enabled              bool
	name                 string
	replaces             string
	uncommentLines       bool
	uncommentLinesPrefix string
}

func (t *tagSnippet) Name() string { return t.name }

func (t *tagSnippet) Enabled() bool { return t.enabled }

type line struct {
	tag    string
	endTag bool
	text   []byte
}

type File struct {
	stack []tag
	tags  map[string]tag
	lines []line
}

func (f *File) Enable(name string) {
	tag, ok := f.tags[name]
	if !ok {
		return
	}

	switch t := tag.(type) {
	case *tagState:
		t.enabled = true

	case *tagSnippet:
		t.enabled = true
		if t.replaces != "" {
			f.Disable(t.replaces)
		}

	default:
		panic("unknown tag type")
	}
}

func (f *File) Disable(name string) {
	tag, ok := f.tags[name]
	if !ok {
		return
	}

	switch t := tag.(type) {
	case *tagState:
		t.enabled = false

	case *tagSnippet:
		t.enabled = false
		if t.replaces != "" {
			f.Enable(t.replaces)
		}

	default:
		panic("unknown tag type")
	}
}

func (f *File) SnippetLines(name string) (start, end int, ok bool) {
	start = -1
	end = -1

	var (
		i      int
		lineID int
	)

	for ; i < len(f.lines); i++ {
		l := f.lines[i]

		if l.tag != "" {
			if tag, ok := f.tags[l.tag]; !ok || !tag.Enabled() {
				continue
			}
		}

		if l.endTag {
			continue
		}

		lineID++

		if l.tag == name {
			start = lineID - 1
			break
		}
	}

	if start == -1 {
		return -1, -1, false
	}

	for ; i < len(f.lines); i++ {
		l := f.lines[i]

		if l.tag != "" {
			if tag, ok := f.tags[l.tag]; !ok || !tag.Enabled() {
				continue
			}
		}

		if !l.endTag {
			lineID++
			continue
		}

		if l.tag == name {
			end = lineID - 1
			break
		}
	}

	if end == -1 {
		return -1, -1, false
	}

	return start, end, true
}

func (f *File) doEnabledLines(fn func(l *line) (next bool)) {
	for _, line := range f.lines {
		if line.tag == "" {
			if tag, ok := f.tags[line.tag]; !ok || !tag.Enabled() {
				continue
			}
		}

		if !fn(&line) {
			break
		}
	}
}

func (f *File) topTag() (tag, bool) {
	if len(f.stack) == 0 {
		return nil, false
	}

	return f.stack[len(f.stack)-1], true
}

func (f *File) popTag() (tag, bool) {
	v, ok := f.topTag()
	if !ok {
		return nil, false
	}

	f.stack = f.stack[:len(f.stack)-1]
	return v, true
}

type FileParser struct {
	r *bufio.Reader
}

func NewFileParser(rd io.Reader) *FileParser {
	return &FileParser{
		r: bufio.NewReader(rd),
	}
}

func (p *FileParser) Reset(rd io.Reader) {
	p.r.Reset(rd)
}

func (p *FileParser) Parse() (*File, error) {
	file := &File{
		tags: make(map[string]tag),
	}

	var (
		removeAfter int
	)

	for {
		l, err := p.r.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}

			return nil, err
		}

		if removeAfter > 0 {
			removeAfter--
			continue
		}

		marker, start := isMagicLine(l)
		if marker == magicMarkerNone {
			// Just a line.
			tag, ok := file.topTag()
			if ok {
				if t, ok := tag.(*tagSnippet); ok && t.uncommentLines {
					l = uncommentLine(l)
				}

				file.lines = append(file.lines, line{
					tag:  tag.Name(),
					text: l,
				})
			} else {
				file.lines = append(file.lines, line{
					text: l,
				})
			}

			continue
		}

		switch marker {
		case magicMarkerStart:
			kind, args, options, err := parseMagicLine(l, start)
			if err != nil {
				return nil, err
			}

			switch string(kind) {
			case "stage":
				if len(args) != 1 {
					return nil, fmt.Errorf("wrong arity: %s", kind)
				}

				state := &tagState{
					name: string(args[0]),
				}

				file.stack = append(file.stack, state)
				file.tags[state.name] = state

			case "snippet":
				snippet := &tagSnippet{}
				switch len(args) {
				case 1:
					snippet.name = string(args[0])

				case 3:
					snippet.name = string(args[0])

					if string(args[1]) != "replaces" {
						return nil, fmt.Errorf("expected \"replaces\", got %q", args[1])
					}

					snippet.replaces = string(args[2])

				default:
					return nil, fmt.Errorf("wrong arity: %s", kind)
				}

				if v, ok := options["uncomment-lines"]; ok {
					snippet.uncommentLines = true
					if len(v) > 0 {
						snippet.uncommentLinesPrefix = string(v)
					}
				}

				file.stack = append(file.stack, snippet)
				file.tags[snippet.name] = snippet

			default:
				return nil, fmt.Errorf("unknown %q kind: %s", marker, kind)
			}

		case magicMarkerEnd:
			kind, name, err := parseMagicLineEnd(l, start)
			if err != nil {
				return nil, err
			}

			switch string(kind) {
			case "stage":
				prev, ok := file.popTag()
				if !ok {
					return nil, fmt.Errorf("tags stack is empty")
				}

				p, ok := prev.(*tagState)
				if !ok {
					return nil, fmt.Errorf("prev tag is not a state")
				}

				if p.name != string(name) {
					return nil, fmt.Errorf("prev tag has different name: %q != %q", p.name, name)
				}

				file.lines = append(file.lines, line{
					endTag: true,
					tag:    string(name),
				})

			case "snippet":
				prev, ok := file.popTag()
				if !ok {
					return nil, fmt.Errorf("tags stack is empty")
				}

				p, ok := prev.(*tagSnippet)
				if !ok {
					return nil, fmt.Errorf("prev tag is not a snippet")
				}

				if p.name != string(name) {
					return nil, fmt.Errorf("prev tag has different name: %q != %q", p.name, name)
				}

				file.lines = append(file.lines, line{
					endTag: true,
					tag:    string(name),
				})

			default:
				return nil, fmt.Errorf("unknown %q kind: %s", marker, kind)
			}

		case magicMarkerLine:
			kind, args, options, err := parseMagicLine(l, start)
			if err != nil {
				return nil, err
			}

			switch string(kind) {
			case "remove-lines":
				if len(args) != 0 {
					return nil, fmt.Errorf("wrong arity: %s", kind)
				}

				if v, ok := options["before"]; ok {
					value, err := strconv.Atoi(string(v))
					if err != nil {
						return nil, err
					}

					file.lines = file.lines[:len(file.lines)-value]
				}

				if v, ok := options["after"]; ok {
					value, err := strconv.Atoi(string(v))
					if err != nil {
						return nil, err
					}

					removeAfter += value
				}

			default:
				return nil, fmt.Errorf("unknown %q kind: %s", marker, kind)
			}

		default:
			panic("unknown marker: " + string(marker))
		}
	}

	for {
		prev, ok := file.popTag()
		if !ok {
			break
		}

		file.lines = append(file.lines, line{
			endTag: true,
			tag:    prev.Name(),
		})
	}

	return file, nil
}

type magicMarker byte

const (
	magicMarkerNone  magicMarker = 0
	magicMarkerStart magicMarker = '>'
	magicMarkerEnd   magicMarker = '<'
	magicMarkerLine  magicMarker = '^'
)

func isMagicLine(line []byte) (marker magicMarker, start int) {
	start, _ = skipWhitespaces(line, 0)

	if start >= len(line)-2 || line[start] != '/' || line[start+1] != '/' {
		return magicMarkerNone, 0
	}

	switch marker = magicMarker(line[start+2]); marker {
	case magicMarkerStart, magicMarkerEnd, magicMarkerLine:
		start, _ = skipWhitespaces(line, start+3)
		return marker, start

	default:
		return magicMarkerNone, 0
	}
}

func skipWhitespaces(line []byte, start int) (idx, size int) {
	end := start
loop:
	for ; end < len(line); end++ {
		switch line[end] {
		case ' ', '\t':
			// Eat.

		default:
			break loop
		}
	}

	return end, end - start
}

func expectWhitespaces(line []byte, start int) (int, error) {
	end, size := skipWhitespaces(line, start)
	if size <= 0 {
		return start, fmt.Errorf("expected at least one whitespace")
	}

	return end, nil
}

func expectEOL(line []byte, start int) error {
	if start >= len(line) {
		return fmt.Errorf("expected EOL, got <nil>")
	}
	if start != len(line)-1 || line[start] != '\n' {
		return fmt.Errorf("expected EOL, got %q", line[start])
	}
	return nil
}

func parseMagicLine(line []byte, start int) (kind []byte, args [][]byte, options map[string][]byte, err error) {
	start, _ = skipWhitespaces(line, start)
	if start >= len(line) {
		panic("reached EOL")
	}

	// Parse the kind.
	kind, start, err = parseString(line, start)
	if err != nil {
		return nil, nil, nil, err
	}

	options = make(map[string][]byte)

	// Parse args.
	for {
		var size int
		start, size = skipWhitespaces(line, start)
		if start >= len(line) || size <= 0 {
			break
		}

		ch := line[start]
		if ch == ':' || ch == '\n' {
			break
		}

		var arg []byte
		arg, start, err = parseString(line, start)
		if err != nil {
			return nil, nil, nil, err
		}

		args = append(args, arg)
	}

	// Parse options.
	if start >= len(line) {
		return kind, args, options, nil
	}

	if line[start] == ':' {
		start++

		var foundOption bool
		for start < len(line) {
			start, _ = skipWhitespaces(line, start)

			if start >= len(line) {
				break
			}

			ch := line[start]
			if ch == '\n' {
				break
			}

			if foundOption {
				if ch != ',' {
					return nil, nil, nil, fmt.Errorf("expected ',', got %q", ch)
				}
				start++

				start, _ = skipWhitespaces(line, start)
			}

			var key, value []byte
			key, value, start, err = parseOption(line, start)
			if err != nil {
				return nil, nil, nil, err
			}

			options[string(key)] = value
			foundOption = true
		}
	}

	// Expect the EOL.
	start, _ = skipWhitespaces(line, start)

	err = expectEOL(line, start)
	if err != nil {
		return nil, nil, nil, err
	}

	return kind, args, options, nil
}

func parseMagicLineEnd(line []byte, start int) (kind, name []byte, err error) {
	start, _ = skipWhitespaces(line, start)
	if start >= len(line) {
		panic("reached EOL")
	}

	// Parse the kind.
	kind, start, err = parseString(line, start)
	if err != nil {
		return nil, nil, err
	}

	// Parse the name.
	start, err = expectWhitespaces(line, start)
	if err != nil {
		return nil, nil, err
	}

	name, start, err = parseString(line, start)
	if err != nil {
		return nil, nil, err
	}

	// Expect the EOL.
	start, _ = skipWhitespaces(line, start)

	err = expectEOL(line, start)
	if err != nil {
		return nil, nil, err
	}

	return kind, name, nil
}

func parseString(line []byte, start int) ([]byte, int, error) {
	if start >= len(line) {
		panic("reached EOL")
	}

	switch ch := line[start]; ch {
	case '"', '\'':
		return parseQuotedString(line, start, ch)

	default:
		return parseUnquotedString(line, start)
	}
}

func parseUnquotedString(line []byte, start int) ([]byte, int, error) {
	if start >= len(line) {
		panic("reached EOL")
	}

	end := start
	for ; end < len(line); end++ {
		ch := line[end]
		valid := (ch >= 'a' && ch <= 'z') ||
			(ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '1') ||
			ch == '-' ||
			ch == '_'

		if !valid {
			break
		}
	}

	if start == end {
		return nil, start, fmt.Errorf("empty string")
	}

	return line[start:end], end, nil
}

func parseQuotedString(line []byte, start int, q byte) ([]byte, int, error) {
	if start >= len(line) {
		panic("reached EOL")
	}

	if ch := line[start]; ch != q {
		return nil, start, fmt.Errorf("expected %q, got %q", q, ch)
	}
	end := start + 1

	// TODO(SuperPaintman): optimize it.
	var (
		buf     bytes.Buffer
		escaped bool
		closed  bool
	)
loop:
	for ; end < len(line); end++ {
		ch := line[end]
		switch ch {
		case '\n', '\r':
			return nil, start, fmt.Errorf("unexpected newline: %q", ch)

		case q:
			if !escaped {
				end++
				closed = true
				break loop
			}

		case '\\':
			if !escaped {
				escaped = true
				continue
			}
		}

		_ = buf.WriteByte(ch)
		escaped = false
	}

	if escaped || !closed {
		return nil, start, fmt.Errorf("unexpected end of the string")
	}

	if buf.Len() <= 0 {
		return nil, start, fmt.Errorf("empty string")
	}

	return buf.Bytes(), end, nil
}

func parseOption(line []byte, start int) (key, value []byte, end int, err error) {
	if start >= len(line) {
		panic("reached EOL")
	}
	end = start

	// Parse the key.
	key, end, err = parseUnquotedString(line, end)
	if err != nil {
		return nil, nil, start, err
	}

	// Parse the '='.
	end, _ = skipWhitespaces(line, end)

	if end >= len(line) || line[end] != '=' {
		return key, nil, end, nil
	}

	end++

	// Parse the value.
	end, _ = skipWhitespaces(line, end)

	value, end, err = parseString(line, end)
	if err != nil {
		return nil, nil, start, nil
	}

	return key, value, end, nil
}

func uncommentLine(line []byte) []byte {
	idx, _ := skipWhitespaces(line, 0)
	if idx >= len(line)-1 {
		return line
	}

	if line[idx] != '/' || line[idx+1] != '/' {
		return line
	}

	res := line[:idx]
	suffix := line[idx+2:]
	if len(suffix) > 0 && suffix[0] == ' ' {
		suffix = suffix[1:]
	}

	res = append(res, suffix...)

	return res
}
