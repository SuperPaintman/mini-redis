//> stage "Redis Protocol"
package radish

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"sync"
)

var (
	ErrBulkLength      = &Error{"ERR", "Protocol error: invalid bulk length"}
	ErrMultibulkLength = &Error{"ERR", "Protocol error: invalid multibulk length"}
	ErrIntegerValue    = &Error{"ERR", "Protocol error: invalid integer value"}

	errValue             = errors.New("invalid value")
	errLineLimitExceeded = errors.New("line limit exceeded")
)

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

func (c *Command) reset() {
	*c = Command{
		Raw:  c.Raw[:0],
		Args: c.Args[:0],
	}
}

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

// ReadCommand reads and returns a Command from the underlying reader.
//
// The returned Command might be reused, the client should not store or modify
// it or its fields.
func (r *Reader) ReadCommand() (cmd *Command, err error) {
	cmd = newCommand()
	defer func() {
		if err != nil {
			commandPool.Put(cmd)
			cmd = nil
		}
	}()

next:
	// We don't support paint text commands now.
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

	if diff := arrayLength - cap(cmd.Args); diff > 0 {
		// Grow the Args slice.
		cmd.Args = append(cmd.Args, make([]Arg, diff)...)[:len(cmd.Args)]
	}

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

	return cmd, err
}

// ReadSimpleString reads and returns a RESP simple string from the underlying
// reader.
func (r *Reader) ReadSimpleString() (string, error) {
	cmd := newCommand()
	defer commandPool.Put(cmd)

	line, err := r.readLine(DataTypeSimpleString, 0, cmd)
	return string(line), err
}

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

// ReadString reads and returns a RESP bulk string from the underlying reader.
func (r *Reader) ReadString() (s string, null bool, err error) {
	cmd := newCommand()
	defer commandPool.Put(cmd)

	b, null, err := r.readBulk(cmd)
	return string(b), null, err
}

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

func hasTerminator(b []byte) bool {
	return len(b) > 1 && b[len(b)-2] == '\r' && b[len(b)-1] == '\n'
}
