//> stage "Redis Protocol"
//> snippet writer
package radish

import (
	"bufio"
	"io"

	//^ remove-lines: before=1
	//> snippet writer-import-strconv
	"strconv"
	//< snippet writer-import-strconv
)

//> snippet writer-data-types
// DataType represents a RESP data type.
// It is also used as the first byte for RESP representations.
type DataType byte

const (
	DataTypeSimpleString DataType = '+'
	DataTypeError        DataType = '-'
	DataTypeInteger      DataType = ':'
	DataTypeBulkString   DataType = '$'
	DataTypeArray        DataType = '*'
	//> snippet writer-data-type-null
	DataTypeNull DataType = '_'

//^ remove-lines: before=1
//< snippet writer-data-type-null
)

//< snippet writer-data-types
//^ remove-lines: after=1

//> snippet writer-error
// Error represents a RESP error.
//
// If a read error or any other non-standard RESP error occurs, the actual
// error is returned.
type Error struct {
	Kind string // Optional kind of the error (default ERR). e.g. ERR, WRONGTYPE, WRONGPASS, etc.
	Msg  string
}

func (e *Error) Error() string {
	kind := e.Kind
	if kind == "" {
		kind = "<nil>"
	}
	msg := e.Msg
	if msg == "" {
		msg = "<nil>"
	}
	return "radish: " + kind + " " + msg
}

//< snippet writer-error
//^ remove-lines: after=1

// Writer implements buffering for an io.Writer object.
//
// After all data has been written, the client should call the Flush method to
// guarantee all data has been forwarded to the underlying io.Writer
type Writer struct {
	w *bufio.Writer
	//> snippet writer-writer-smallbuf-field
	smallbuf []byte // A buffer for small values (e.g. results of strconv.AppendInt).
	//< snippet writer-writer-smallbuf-field
}

// NewWriter returns a new Writer writing RESP data types.
func NewWriter(w io.Writer) *Writer {
	//> snippet writer-writer-smallbuf-size
	// Length of the string form of the int64.
	const smallbufSize = len("-9223372036854775808")

	//< snippet writer-writer-smallbuf-size
	return &Writer{
		w: bufio.NewWriter(w),
		//> snippet writer-writer-smallbuf-init
		smallbuf: make([]byte, 0, smallbufSize),
		//< snippet writer-writer-smallbuf-init
	}
}

// Reset discards any unflushed buffered data, and resets w to write
// its output to wr.
func (w *Writer) Reset(wr io.Writer) {
	w.w.Reset(wr)
}

// Flush writes any buffered data to the underlying io.Writer.
func (w *Writer) Flush() error {
	return w.w.Flush()
}

//> snippet writer-write-simple-string
// WriteSimpleString writes a RESP simple string.
func (w *Writer) WriteSimpleString(s string) error {
	_ = w.writeType(DataTypeSimpleString)
	_ = w.writeString(s)
	return w.writeTerminator()
}

//< snippet writer-write-simple-string
//^ remove-lines: after=1

//> snippet writer-write-error
// WriteError writes a RESP error.
func (w *Writer) WriteError(e *Error) error {
	return w.WriteRawError(e.Kind, e.Msg)
}

// WriteRawError writes the kind and string msg as a RESP error.
func (w *Writer) WriteRawError(kind string, msg string) error {
	if kind == "" {
		kind = "ERR"
	}
	_ = w.writeType(DataTypeError)
	_ = w.writeString(kind)
	if msg != "" {
		_ = w.w.WriteByte(' ')
		_ = w.writeString(msg)
	}
	return w.writeTerminator()
}

//< snippet writer-write-error
//^ remove-lines: after=1

//> snippet writer-write-ints
// WriteInt writes a RESP integer.
func (w *Writer) WriteInt(i int) error {
	return w.WriteInt64(int64(i))
}

// WriteInt32 writes a RESP 32-bit integer.
func (w *Writer) WriteInt32(i int32) error {
	return w.WriteInt64(int64(i))
}

// WriteInt64 writes a RESP 64-bit integer.
func (w *Writer) WriteInt64(i int64) error {
	_ = w.writeType(DataTypeInteger)
	_ = w.writeInt(i)
	return w.writeTerminator()
}

//< snippet writer-write-ints
//^ remove-lines: after=1

//> snippet writer-write-uints
// WriteUint writes a RESP unsigned integer.
func (w *Writer) WriteUint(i uint) error {
	return w.WriteUint64(uint64(i))
}

// WriteUint32 writes a RESP 32-bit unsigned integer.
func (w *Writer) WriteUint32(i uint32) error {
	return w.WriteUint64(uint64(i))
}

// WriteUint64 writes a RESP 64-bit unsigned integer.
func (w *Writer) WriteUint64(i uint64) error {
	_ = w.writeType(DataTypeInteger)
	_ = w.writeUint(i)
	return w.writeTerminator()
}

//< snippet writer-write-uints
//^ remove-lines: after=1

//> snippet writer-write-bulk
// WriteString writes a RESP bulk string.
func (w *Writer) WriteString(s string) error {
	_ = w.writePrefix(byte(DataTypeBulkString), len(s))
	_, _ = w.w.WriteString(s)
	return w.writeTerminator()
}

// WriteBytes writes a RESP bulk bytes.
func (w *Writer) WriteBytes(b []byte) error {
	_ = w.writePrefix(byte(DataTypeBulkString), len(b))
	_, _ = w.w.Write(b)
	return w.writeTerminator()
}

//< snippet writer-write-bulk
//^ remove-lines: after=1

//> snippet writer-write-null
// WriteBytes writes the RESP null.
func (w *Writer) WriteNull() error {
	_, err := w.w.WriteString("$-1\r\n")
	return err
}

//< snippet writer-write-null
//^ remove-lines: after=1

//> snippet writer-write-array
// WriteBytes writes a RESP array type of n elements.
func (w *Writer) WriteArray(n int) error {
	return w.writePrefix(byte(DataTypeArray), n)
}

//< snippet writer-write-array
//^ remove-lines: after=1

//> snippet writer-write-type
func (w *Writer) writeType(t DataType) error {
	return w.w.WriteByte(byte(t))
}

//< snippet writer-write-type
//^ remove-lines: after=1

//> snippet writer-write-terminator
func (w *Writer) writeTerminator() error {
	_, err := w.w.WriteString("\r\n")
	return err
}

//< snippet writer-write-terminator
//^ remove-lines: after=1

//> snippet writer-write-prefix
func (w *Writer) writePrefix(prefix byte, n int) error {
	_ = w.w.WriteByte(prefix)
	_ = w.writeInt(int64(n))
	return w.writeTerminator()
}

//< snippet writer-write-prefix
//^ remove-lines: after=1

//> snippet writer-write-string
func (w *Writer) writeString(s string) error {
	// It is better to do a double check than to just copy the string byte by
	// byte. But, of course, it would be better not to do it at all.
	for _, ch := range []byte(s) {
		switch ch {
		case '\r', '\n':
			return w.writeEscapedString(s)
		}
	}

	_, err := w.w.WriteString(s)
	return err
}

func (w *Writer) writeEscapedString(s string) error {
	var err error
	for _, ch := range []byte(s) {
		switch ch {
		case '\r':
			_, err = w.w.WriteString("\\r")

		case '\n':
			_, err = w.w.WriteString("\\n")

		default:
			err = w.w.WriteByte(ch)
		}
	}
	return err
}

//< snippet writer-write-string
//^ remove-lines: after=1

//> snippet writer-write-int
func (w *Writer) writeInt(i int64) error {
	if i >= 0 && i <= 9 {
		return w.w.WriteByte(byte('0' + i))
	}

	w.smallbuf = w.smallbuf[:0]
	w.smallbuf = strconv.AppendInt(w.smallbuf, i, 10)
	_, err := w.w.Write(w.smallbuf)
	return err
}

//< snippet writer-write-int
//^ remove-lines: after=1

//> snippet writer-write-uint
func (w *Writer) writeUint(i uint64) error {
	if i <= 9 {
		return w.w.WriteByte(byte('0' + i))
	}

	w.smallbuf = w.smallbuf[:0]
	w.smallbuf = strconv.AppendUint(w.smallbuf, i, 10)
	_, err := w.w.Write(w.smallbuf)
	return err
}

//< snippet writer-write-uint
//^ remove-lines: after=1

//< snippet writer
