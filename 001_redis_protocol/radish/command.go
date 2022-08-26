package radish

import (
	"bufio"
	"errors"
	"io"
	"sync"
)

var (
	ErrBulkLength      = &Error{"ERR", "Protocol error: invalid bulk length"}
	ErrMultibulkLength = &Error{"ERR", "Protocol error: invalid multibulk length"}

	errLength        = errors.New("invalid length")
	errUnexpectedEOL = errors.New("unexpected EOL")
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

// CommandReader implements a RESP command reader.
type CommandReader struct {
	r *bufio.Reader
}

// NewCommandReader returns a new CommandReader.
func NewCommandReader(r io.Reader) *CommandReader {
	return &CommandReader{
		r: bufio.NewReader(r),
	}
}

// Reset discards any buffered data and switches the reader to read from r.
func (cr *CommandReader) Reset(r io.Reader) {
	cr.r.Reset(r)
}

func (cr *CommandReader) ReadAny() (dt DataType, v interface{}, err error) {
	first, err := cr.r.ReadByte()
	if err == nil {
		err = cr.r.UnreadByte()
	}
	if err != nil {
		return DataTypeNull, nil, err
	}

	dt = DataType(first)
	switch dt {
	case DataTypeSimpleString:
		v, err = cr.ReadSimpleString()

	case DataTypeError:
		v, err = cr.ReadError()

	case DataTypeInteger:
		v, err = cr.ReadInt()

	case DataTypeBulkString:
		var null bool
		v, null, err = cr.ReadString()
		if null {
			v = nil
			dt = DataTypeNull
		}

	case DataTypeArray:
		v, err = cr.ReadArray()

	// DataTypeNull

	default:
		return DataTypeNull, nil, errors.New("TODO")
	}

	return dt, v, err
}

func (cr *CommandReader) ReadSimpleString() (string, error) {
	cmd := newCommand()
	defer commandPool.Put(cmd)

	line, err := cr.readLine(DataTypeSimpleString, 0, cmd)
	// TODO
	return string(line), err
}

func (cr *CommandReader) ReadError() (*Error, error) {
	cmd := newCommand()
	defer commandPool.Put(cmd)

	line, err := cr.readLine(DataTypeError, 0, cmd)
	if err != nil {
		return nil, err
	}

	spacePos := -1
	for i, ch := range line {
		if ch == ' ' {
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

func (cr *CommandReader) ReadInt() (int, error) {
	cmd := newCommand()
	defer commandPool.Put(cmd)

	i, err := cr.readDataTypeLength(DataTypeInteger, cmd)
	// TODO
	return i, err
}

func (cr *CommandReader) ReadString() (s string, null bool, err error) {
	cmd := newCommand()
	defer commandPool.Put(cmd)

	b, null, err := cr.readBulk(cmd)
	return string(b), null, err
}

func (cr *CommandReader) ReadArray() (int, error) {
	cmd := newCommand()
	defer commandPool.Put(cmd)

	n, err := cr.readDataTypeLength(DataTypeArray, cmd)
	if err == errLength {
		err = ErrMultibulkLength
	}
	return n, err
}

// ReadCommand reads and returns a Command from the underlying reader.
//
// The returned Command might be reused, the client should not store or modify
// it or its fields.
func (cr *CommandReader) ReadCommand() (cmd *Command, err error) {
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
	arrayLength, err := cr.readDataTypeLength(DataTypeArray, cmd)
	if err != nil {
		if err == errLength {
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
		arg, null, err := cr.readBulk(cmd)
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

// readBulk reads a full RESP bulk string (with the prefix and final "\r\n")
// and returns only the content.
//
// It uses the cmd as a buffer and puts all read command bytes into the
// Raw.
func (cr *CommandReader) readBulk(cmd *Command) (bulk []byte, null bool, err error) {
	// Parse a bulk string length.
	bulkLength, err := cr.readDataTypeLength(DataTypeBulkString, cmd)
	if err != nil {
		if err == errLength {
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
	remain := bulkLength + 2 // bulkLength + <CR><LF>

	cmd.grow(remain)

	for remain > 0 {
		n, err := cr.r.Read(cmd.Raw[si:])
		if err != nil {
			return nil, false, err
		}
		remain -= n
		si += n
	}

	if cmd.Raw[len(cmd.Raw)-2] != '\r' || cmd.Raw[len(cmd.Raw)-1] != '\n' {
		return nil, false, ErrBulkLength
	}

	return cmd.Raw[start : len(cmd.Raw)-2], false, nil
}

func (cr *CommandReader) readLine(dt DataType, limit int, cmd *Command) ([]byte, error) {
	start := len(cmd.Raw)

	var length int
	for limit <= 0 || length < limit {
		frag, err := cr.r.ReadSlice('\n')
		length += len(frag)

		if err == nil { // Got the final fragment.
			cmd.Raw = append(cmd.Raw, frag...)
			break
		}
		if err != bufio.ErrBufferFull { // Unexpected error.
			return nil, err
		}

		cmd.Raw = append(cmd.Raw, frag...)
	}

	if ch := cmd.Raw[start]; ch != byte(dt) {
		return nil, &Error{"ERR", "expected '" + string(dt) + "', got '" + string(ch) + "'"}
	}

	if cmd.Raw[len(cmd.Raw)-2] != '\r' || cmd.Raw[len(cmd.Raw)-1] != '\n' {
		return nil, errUnexpectedEOL
	}

	return cmd.Raw[start+1 : len(cmd.Raw)-2], nil
}

// readDataTypeLength reads a length of the data type.
//
// It checks the data type prefix and returns an error if it does not
// match the dt.
//
// It uses the cmd as a buffer and puts all read command bytes into the
// Raw.
func (cr *CommandReader) readDataTypeLength(dt DataType, cmd *Command) (int, error) {
	// Length of the string form of the int64 + <marker><CR><LF>.
	const maxLength = 20 + 3

	line, err := cr.readLine(dt, maxLength, cmd)
	if err != nil {
		if err == errUnexpectedEOL {
			return 0, errLength
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

func parseInt(b []byte) (n int, err error) {
	if len(b) == 0 {
		return 0, errLength
	}

	var negative bool

	if b[0] == '-' {
		negative = true
		if len(b) < 1 {
			return 0, errLength
		}

		b = b[1:]
	}

	for _, ch := range b {
		ch -= '0'
		if ch > 9 {
			return 0, errLength
		}
		n = n*10 + int(ch)
	}

	if negative {
		n = -1 * n
	}

	return n, nil
}
