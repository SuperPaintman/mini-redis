//> stage "Redis Protocol"
//> snippet reader
package radish

import (
	"bufio"
	//> snippet reader-import-errors
	"errors"
	//< snippet reader-import-errors
	//> snippet reader-import-fmt
	"fmt"
	//< snippet reader-import-fmt
	"io"
	//> snippet reader-import-sync
	"sync"
	//< snippet reader-import-sync
)

//> snippet reader-errors
var (
	ErrMultibulkLength = &Error{"ERR", "Protocol error: invalid multibulk length"}
	//> snippet reader-error-bulk-length
	ErrBulkLength = &Error{"ERR", "Protocol error: invalid bulk length"}
	//< snippet reader-error-bulk-length
	//> snippet reader-error-integer-value
	ErrIntegerValue = &Error{"ERR", "Protocol error: invalid integer value"}
	//< snippet reader-error-integer-value

	errValue = errors.New("invalid value")
	//> snippet reader-error-line-limit-exceeded
	errLineLimitExceeded = errors.New("line limit exceeded")
	//< snippet reader-error-line-limit-exceeded
)

//< snippet reader-errors
//^ remove-lines: after=1

//> snippet reader-command
const (
	initialCommandRawSize  = 1024 // 1KB
	initialCommandArgsSize = 4    // More than enough for most of the commands.
)

// Arg represents a byte slice of the raw input.
type Arg []byte

// Bytes creates a new copy of the underlying byte slice and returns it.
func (a Arg) Bytes() []byte { return append([]byte(nil), a...) }

// Command represents a RESP command.
//
// After each reading, the Command can be reused, the client should not store
// or modify the Command or any of its fields without a full copy.
type Command struct {
	// Raw contains all bytes of the command, including all prefixes and "\r\n".
	Raw []byte
	// Args are bytes slices of the Raw witout "\r\n".
	Args []Arg
}

//> snippet reader-command-pool
var commandPool sync.Pool

func newCommand() *Command {
	cmd, ok := commandPool.Get().(*Command)
	if ok {
		cmd.reset()
	} else {
		cmd = &Command{
			Raw:  make([]byte, 0, initialCommandRawSize),
			Args: make([]Arg, 0, initialCommandArgsSize),
		}
	}
	return cmd
}

//< snippet reader-command-pool
//^ remove-lines: after=1

func (c *Command) reset() {
	*c = Command{
		Raw:  c.Raw[:0],
		Args: c.Args[:0],
	}
}

//> snippet reader-command-grow
// grow allocates extra bytes to the Raw if necessary and increases
// the length of the Raw by n bytes.
func (c *Command) grow(n int) {
	// Enough space for reading.
	if n <= cap(c.Raw)-len(c.Raw) {
		c.Raw = c.Raw[:len(c.Raw)+n]
		return
	}

	// It might be slower than growSlice from the buffer package. But the compiler
	// should recognize this pattern in the future.
	//
	// See: https://github.com/golang/go/blob/go1.19/src/bytes/buffer.go#L226-L242
	// See: https://go-review.googlesource.com/c/go/+/370578
	// See: https://godbolt.org/z/8P7ns9ana
	c.Raw = append(c.Raw, make([]byte, n)...)
}

//< snippet reader-command-grow
//^ remove-lines: after=1

//< snippet reader-command
//^ remove-lines: after=1

// Reader implements a RESP reader.
type Reader struct {
	r *bufio.Reader
}

// NewReader returns a new Reader.
func NewReader(rd io.Reader) *Reader {
	return &Reader{
		r: bufio.NewReader(rd),
	}
}

// Reset discards any buffered data and switches the reader to read from rd.
func (r *Reader) Reset(rd io.Reader) {
	r.r.Reset(rd)
}

//> snippet reader-read-command
// ReadCommand reads and returns a Command from the underlying reader.
//
// The returned Command might be reused, the client should not store or modify
// it or its fields.
func (r *Reader) ReadCommand() (cmd *Command, err error) {
	//> snippet reader-read-command-cmd: uncomment-lines
	// cmd = &Command{
	// 	Raw:  make([]byte, 0, initialCommandRawSize),
	// 	Args: make([]Arg, 0, initialCommandArgsSize),
	// }
	//
	//< snippet reader-read-command-cmd
	//> snippet reader-read-command-from-pool replaces reader-read-command-cmd
	cmd = newCommand()
	defer func() {
		if err != nil {
			commandPool.Put(cmd)
			cmd = nil
		}
	}()

	//< snippet reader-read-command-from-pool
	//> snippet reader-read-command-array-length
next:
	// We don't support plain text commands now.
	// Just try to parse the input as a array.
	arrayLength, err := r.readValue(DataTypeArray, cmd)
	if err != nil {
		if err == errValue {
			return cmd, ErrMultibulkLength
		}

		return cmd, err
	}
	// Skip empty commands and read the next one.
	if arrayLength <= 0 {
		cmd.reset()
		goto next
	}

	//< snippet reader-read-command-array-length
	//> snippet reader-read-command-preallocate-args
	if diff := arrayLength - cap(cmd.Args); diff > 0 {
		// Grow the Args slice.
		cmd.Args = append(cmd.Args, make([]Arg, diff)...)[:len(cmd.Args)]
	}

	//< snippet reader-read-command-preallocate-args
	//> snippet reader-read-command-parse-elements
	// Parse elements.
	for i := 0; i < arrayLength; i++ {
		arg, null, err := r.readBulk(cmd)
		if err != nil {
			return cmd, err
		}
		if null {
			return cmd, ErrBulkLength
		}

		cmd.Args = append(cmd.Args, arg)
	}

	//< snippet reader-read-command-parse-elements
	return cmd, err
}

//< snippet reader-read-command
//^ remove-lines: after=1

//> snippet reader-read-simple-string
// ReadSimpleString reads and returns a RESP simple string from the underlying
// reader.
func (r *Reader) ReadSimpleString() (string, error) {
	cmd := newCommand()
	defer commandPool.Put(cmd)

	line, err := r.readLine(DataTypeSimpleString, 0, cmd)
	return string(line), err
}

//< snippet reader-read-simple-string
//^ remove-lines: after=1

//> snippet reader-read-error
// ReadError reads and returns a RESP error from the underlying reader.
//
// The first word after the "-", up to the first space or newline, is parsed
// as the kind of error returned.
func (r *Reader) ReadError() (*Error, error) {
	cmd := newCommand()
	defer commandPool.Put(cmd)

	line, err := r.readLine(DataTypeError, 0, cmd)
	if err != nil {
		return nil, err
	}

	spacePos := -1
	for i, ch := range line {
		if ch == ' ' || ch == '\n' {
			spacePos = i
			break
		}
	}

	e := &Error{}
	if spacePos == -1 {
		e.Kind = string(line)
	} else {
		e.Kind = string(line[:spacePos])
		e.Msg = string(line[spacePos+1:])
	}

	return e, nil
}

//< snippet reader-read-error
//^ remove-lines: after=1

//> snippet reader-read-integer
// ReadInteger reads and returns a RESP integer from the underlying reader.
func (r *Reader) ReadInteger() (int, error) {
	cmd := newCommand()
	defer commandPool.Put(cmd)

	i, err := r.readValue(DataTypeInteger, cmd)
	if err == errValue {
		err = ErrIntegerValue
	}
	return i, err
}

//< snippet reader-read-integer
//^ remove-lines: after=1

//> snippet reader-read-string
// ReadString reads and returns a RESP bulk string from the underlying reader.
func (r *Reader) ReadString() (s string, null bool, err error) {
	cmd := newCommand()
	defer commandPool.Put(cmd)

	b, null, err := r.readBulk(cmd)
	return string(b), null, err
}

//< snippet reader-read-string
//^ remove-lines: after=1

//> snippet reader-read-array
// ReadArray reads and returns the length of a RESP array from the underlying
// reader.
func (r *Reader) ReadArray() (length int, err error) {
	cmd := newCommand()
	defer commandPool.Put(cmd)

	n, err := r.readValue(DataTypeArray, cmd)
	if err == errValue {
		err = ErrMultibulkLength
	}
	return n, err
}

//< snippet reader-read-array
//^ remove-lines: after=1

//> snippet reader-read-any
// ReadAny reads and returns a RESP type and its value from the underlying
// reader.
//
// The data type is determined by the first byte.
func (r *Reader) ReadAny() (dt DataType, v interface{}, err error) {
	first, err := r.r.ReadByte()
	if err == nil {
		err = r.r.UnreadByte()
	}
	if err != nil {
		return DataTypeNull, nil, err
	}

	dt = DataType(first)
	switch dt {
	case DataTypeSimpleString:
		v, err = r.ReadSimpleString()

	case DataTypeError:
		v, err = r.ReadError()

	case DataTypeInteger:
		v, err = r.ReadInteger()

	case DataTypeBulkString:
		var null bool
		v, null, err = r.ReadString()
		if null {
			v = nil
			dt = DataTypeNull
		}

	case DataTypeArray:
		v, err = r.ReadArray()

	// DataTypeNull is an internal data type. Nulls are handled by
	// DataTypeBulkString.

	default:
		return DataTypeNull, nil, &Error{"ERR", fmt.Sprintf("Protocol error, got %q as reply type byte", string(dt))}
	}

	return dt, v, err
}

//< snippet reader-read-any
//^ remove-lines: after=1

//> snippet reader-read-line
// readLine reads a full RESP line terminating with <CRLF> and starting with
// a given data type.
//
// It checks the first byte and returns an error if it does not match the dt.
//
// It uses the cmd as a buffer and puts all read bytes into the Raw.
func (r *Reader) readLine(dt DataType, limit int, cmd *Command) ([]byte, error) {
	start := len(cmd.Raw)

	var length int
	for limit <= 0 || length < limit {
		frag, err := r.r.ReadSlice('\n')
		length += len(frag)

		if err == nil { // Got the final fragment.
			cmd.Raw = append(cmd.Raw, frag...)

			if len(frag) < 2 || frag[len(frag)-2] != '\r' { // Not a <CRLF>
				continue
			}
			break
		}
		if err != bufio.ErrBufferFull { // Unexpected error.
			return nil, err
		}

		cmd.Raw = append(cmd.Raw, frag...)
	}

	if !hasTerminator(cmd.Raw) {
		return nil, errLineLimitExceeded
	}

	if ch := cmd.Raw[start]; ch != byte(dt) {
		return nil, &Error{"ERR", "expected '" + string(dt) + "', got '" + string(ch) + "'"}
	}

	return cmd.Raw[start+1 : len(cmd.Raw)-2], nil
}

//< snippet reader-read-line
//^ remove-lines: after=1

//> snippet reader-read-value
// readValue reads a value or a length of a given data type.
//
// It checks the first byte and returns an error if it does not match the dt.
//
// It uses the cmd as a buffer and puts all read bytes into the Raw.
func (r *Reader) readValue(dt DataType, cmd *Command) (int, error) {
	// Length of the string form of the <marker> + int64 + <CRLF>.
	const maxLength = len(":-9223372036854775808\r\n")

	line, err := r.readLine(dt, maxLength, cmd)
	if err != nil {
		if err == errLineLimitExceeded {
			err = errValue
		}
		return 0, err
	}

	// Parse the line as an integer.
	n, err := parseInt(line)
	if err != nil {
		return 0, err
	}

	return n, nil
}

//< snippet reader-read-value
//^ remove-lines: after=1

//> snippet reader-read-bulk
// readBulk reads a full RESP bulk string (with the prefix and the <CRLF>)
// and returns only the content.
//
// It uses the cmd as a buffer and puts all read bytes into the Raw.
func (r *Reader) readBulk(cmd *Command) (bulk []byte, null bool, err error) {
	// Parse a bulk string length.
	bulkLength, err := r.readValue(DataTypeBulkString, cmd)
	if err != nil {
		if err == errValue {
			err = ErrBulkLength
		}
		return nil, false, err
	}
	if bulkLength < 0 {
		return nil, true, nil
	}

	// Parse the bulk string content.
	start := len(cmd.Raw)
	si := len(cmd.Raw)

	const crlfLength = len("\r\n")
	remain := bulkLength + crlfLength

	cmd.grow(remain)

	for remain > 0 {
		n, err := r.r.Read(cmd.Raw[si:])
		if err != nil {
			return nil, false, err
		}
		remain -= n
		si += n
	}

	if !hasTerminator(cmd.Raw) {
		return nil, false, ErrBulkLength
	}

	return cmd.Raw[start : len(cmd.Raw)-2], false, nil
}

//< snippet reader-read-bulk
//^ remove-lines: after=1

//> snippet reader-parse-int
func parseInt(b []byte) (n int, err error) {
	if len(b) == 0 {
		return 0, errValue
	}

	var negative bool

	if b[0] == '-' {
		negative = true
		if len(b) < 1 {
			return 0, errValue
		}

		b = b[1:]
	}

	for _, ch := range b {
		ch -= '0'
		if ch > 9 {
			return 0, errValue
		}
		n = n*10 + int(ch)
	}

	if negative {
		n = -1 * n
	}

	return n, nil
}

//< snippet reader-parse-int
//^ remove-lines: after=1

//> snippet reader-has-terminator
func hasTerminator(b []byte) bool {
	return len(b) > 1 && b[len(b)-2] == '\r' && b[len(b)-1] == '\n'
}

//< snippet reader-has-terminator
//^ remove-lines: after=1

//< snippet reader
