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

	errInvalidLength = errors.New("invalid length")
)

const (
	initialCommandBufSize  = 1024 // 1KB
	initialCommandArgsSize = 4    // More than enough for most of the commands.
)

type Arg []byte

func (a Arg) Bytes() []byte {
	b := make([]byte, len(a))
	copy(b, a)
	return b
}

type Command struct {
	Raw  []byte
	Args []Arg
}

var commandPool sync.Pool

func newCommand() *Command {
	cmd, ok := commandPool.Get().(*Command)
	if ok {
		cmd.reset()
	} else {
		cmd = &Command{
			Raw:  make([]byte, 0, initialCommandBufSize),
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

func (c *Command) grow(n int) {
	// Enough space for reading.
	if n <= cap(c.Raw)-len(c.Raw) {
		c.Raw = c.Raw[:len(c.Raw)+n]
		return
	}

	// It might be slower than `growSlice` from `buffer` package. But the compiler
	// should recognize this pattern in the future.
	//
	// See: https://github.com/golang/go/blob/go1.19/src/bytes/buffer.go#L226-L242
	// See: https://go-review.googlesource.com/c/go/+/370578
	// See: https://godbolt.org/z/8P7ns9ana
	c.Raw = append(c.Raw, make([]byte, n)...)
}

type CommandReader struct {
	r *bufio.Reader
}

func NewCommandReader(r io.Reader) *CommandReader {
	return &CommandReader{
		r: bufio.NewReader(r),
	}
}

func (cr *CommandReader) Reset(r io.Reader) {
	cr.r.Reset(r)
}

func (cr *CommandReader) ReadCommand() (cmd *Command, err error) {
	cmd = newCommand()
	defer func() {
		if err != nil {
			commandPool.Put(cmd)
			cmd = nil
		}
	}()

	// We don't support paint text commands now.
	// Just try to parse the input as a array.
	numberOfElements, err := cr.readDataTypeSize(DataTypeArray, cmd)
	if err != nil {
		if err == errInvalidLength {
			return cmd, ErrMultibulkLength
		}

		return cmd, err
	}

	if diff := numberOfElements - cap(cmd.Args); diff > 0 {
		// Grow the Args slice.
		cmd.Args = append(cmd.Args, make([]Arg, diff)...)[:len(cmd.Args)]
	}

	// Parse elements.
	for i := 0; i < numberOfElements; i++ {
		arg, err := cr.readBulkString(cmd)
		if err != nil {
			return cmd, err
		}

		cmd.Args = append(cmd.Args, arg)
	}

	return cmd, err
}

func (cr *CommandReader) readBulkString(cmd *Command) ([]byte, error) {
	// Parse a bulk string length.
	bulkLength, err := cr.readDataTypeSize(DataTypeBulkString, cmd)
	if err != nil {
		if err == errInvalidLength {
			return nil, ErrBulkLength
		}

		return nil, err
	}

	// Parse the bulk string content.
	start := len(cmd.Raw)
	si := len(cmd.Raw)
	remain := bulkLength + 2

	cmd.grow(remain)

	for remain > 0 {
		n, err := cr.r.Read(cmd.Raw[si:])
		if err != nil {
			return nil, err
		}
		remain -= n
		si += n
	}

	if cmd.Raw[len(cmd.Raw)-2] != '\r' || cmd.Raw[len(cmd.Raw)-1] != '\n' {
		return nil, ErrBulkLength
	}

	return cmd.Raw[start : len(cmd.Raw)-2], nil
}

func (cr *CommandReader) readDataTypeSize(dt DataType, cmd *Command) (int, error) {
	// Length of the string form of the int64 + <marker><CR><LF>.
	const maxLength = 20 + 3

	start := len(cmd.Raw)

	var length int
	for length < maxLength {
		frag, err := cr.r.ReadSlice('\n')
		length += len(frag)

		if err == nil { // Got the final fragment.
			cmd.Raw = append(cmd.Raw, frag...)
			break
		}
		if err != bufio.ErrBufferFull { // Unexpected error.
			return 0, err
		}

		cmd.Raw = append(cmd.Raw, frag...)
	}

	// Validate the line.
	if ch := cmd.Raw[start]; ch != byte(dt) {
		return 0, &Error{"ERR", "expected '" + string(dt) + "', got '" + string(ch) + "'"}
	}

	if cmd.Raw[len(cmd.Raw)-2] != '\r' || cmd.Raw[len(cmd.Raw)-1] != '\n' {
		return 0, errInvalidLength
	}

	// Parse the line as an integer.
	n, err := parseInt(cmd.Raw[start+1 : len(cmd.Raw)-2])
	if err != nil {
		return 0, err
	}

	return n, nil
}

func parseInt(b []byte) (n int, err error) {
	if len(b) == 0 {
		return 0, errInvalidLength
	}

	var negative bool

	if b[0] == '-' {
		negative = true
		if len(b) < 1 {
			return 0, errInvalidLength
		}

		b = b[1:]
	}

	for _, ch := range b {
		ch -= '0'
		if ch > 9 {
			return 0, errInvalidLength
		}
		n = n*10 + int(ch)
	}

	if negative {
		n = -1 * n
	}

	return n, nil
}
