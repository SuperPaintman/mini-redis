package radish

import (
	"bufio"
	"io"
	"strconv"
)

// DataType represents a RESP data type.
// It is also used as the first byte for RESP representations.
type DataType byte

const (
	DataTypeSimpleString DataType = '+'
	DataTypeError        DataType = '-'
	DataTypeInteger      DataType = ':'
	DataTypeBulkString   DataType = '$'
	DataTypeArray        DataType = '*'
)

// Writer implements buffering for an io.Writer object.
//
// After all data has been written, the client should call the Flush method to
// guarantee all data has been forwarded to the underlying io.Writer
type Writer struct {
	smallbuf []byte // A buffer for small values (e.g. results of strconv.AppendInt).
	w        *bufio.Writer
}

// NewWriter returns a new Writer writing RESP data types.
func NewWriter(w io.Writer) *Writer {
	const smallbufSize = 20 // Length of the string form of the int64.

	return &Writer{
		smallbuf: make([]byte, 0, smallbufSize),
		w:        bufio.NewWriter(w),
	}
}

func (w *Writer) Reset(wr io.Writer) {
	w.smallbuf = w.smallbuf[:0]
	w.w.Reset(wr)
}

// WriteSimpleString writes a RESP bulk string.
func (w *Writer) WriteSimpleString(s string) error {
	w.writeType(DataTypeSimpleString)
	w.writeString(s)
	return w.writeTerminator()
}

// WriteError writes a RESP error.
func (w *Writer) WriteError(s string) error {
	w.writeType(DataTypeError)
	w.writeString(s)
	return w.writeTerminator()
}

// WriteInt writes a RESP integer.
func (w *Writer) WriteInt(i int) error {
	return w.WriteInt64(int64(i))
}

// WriteInt32 writes a RESP integer.
func (w *Writer) WriteInt32(i int32) error {
	return w.WriteInt64(int64(i))
}

// WriteInt64 writes a RESP integer.
func (w *Writer) WriteInt64(i int64) error {
	w.writeType(DataTypeInteger)
	w.writeInt(i)
	return w.writeTerminator()
}

// WriteUint writes a RESP integer.
func (w *Writer) WriteUint(i uint) error {
	return w.WriteUint64(uint64(i))
}

// WriteUint32 writes a RESP integer.
func (w *Writer) WriteUint32(i uint32) error {
	return w.WriteUint64(uint64(i))
}

// WriteUint64 writes a RESP integer.
func (w *Writer) WriteUint64(i uint64) error {
	w.writeType(DataTypeInteger)
	w.writeUint(i)
	return w.writeTerminator()
}

// WriteString writes a RESP bulk string.
func (w *Writer) WriteString(s string) error {
	w.writePrefix(byte(DataTypeBulkString), len(s))
	w.w.WriteString(s)
	return w.writeTerminator()
}

// WriteBytes writes a RESP bulk string.
func (w *Writer) WriteBytes(b []byte) error {
	w.writePrefix(byte(DataTypeBulkString), len(b))
	w.w.Write(b)
	return w.writeTerminator()
}

// WriteBytes writes the RESP null.
func (w *Writer) WriteNull() error {
	_, err := w.w.WriteString("$-1\r\n")
	return err
}

// WriteBytes writes a RESP array type of n elements.
func (w *Writer) WriteArray(n int) error {
	return w.writePrefix(byte(DataTypeArray), n)
}

func (w *Writer) writeType(t DataType) error {
	return w.w.WriteByte(byte(t))
}

func (w *Writer) writeTerminator() error {
	_, err := w.w.WriteString("\r\n")
	return err
}

func (w *Writer) writeInt(i int64) error {
	if i >= 0 && i <= 9 {
		return w.w.WriteByte(byte('0' + i))
	}

	w.smallbuf = w.smallbuf[:0]
	w.smallbuf = strconv.AppendInt(w.smallbuf, i, 10)
	_, err := w.w.Write(w.smallbuf)
	return err
}

func (w *Writer) writeUint(i uint64) error {
	if i <= 9 {
		return w.w.WriteByte(byte('0' + i))
	}

	w.smallbuf = w.smallbuf[:0]
	w.smallbuf = strconv.AppendUint(w.smallbuf, i, 10)
	_, err := w.w.Write(w.smallbuf)
	return err
}

func (w *Writer) writePrefix(prefix byte, n int) error {
	w.w.WriteByte(prefix)
	w.writeInt(int64(n))
	return w.writeTerminator()
}

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

// Flush writes any buffered data to the underlying io.Writer.
func (w *Writer) Flush() error {
	return w.w.Flush()
}
