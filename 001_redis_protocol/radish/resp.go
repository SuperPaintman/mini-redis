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

// AppendSimpleString appends the RESP simple string form of the string s
// to dst and returns the extended buffer.
func AppendSimpleString(dst []byte, s string) []byte {
	dst = appendType(dst, DataTypeSimpleString)
	dst = appendString(dst, s)
	dst = appendTerminator(dst)
	return dst
}

// AppendError appends the RESP error form of the string s
// to dst and returns the extended buffer.
func AppendError(dst []byte, s string) []byte {
	dst = appendType(dst, DataTypeError)
	dst = appendString(dst, s)
	dst = appendTerminator(dst)
	return dst
}

// AppendInt appends the RESP integer from the integer i
// to dst and returns the extended buffer.
func AppendInt(dst []byte, i int) []byte {
	return AppendInt64(dst, int64(i))
}

// AppendInt32 appends the RESP integer from the 32-bit integer i
// to dst and returns the extended buffer.
func AppendInt32(dst []byte, i int32) []byte {
	return AppendInt64(dst, int64(i))
}

// AppendInt64 appends the RESP integer from the 64-bit integer i
// to dst and returns the extended buffer.
func AppendInt64(dst []byte, i int64) []byte {
	if i >= 0 && i <= 9 {
		return appendSmall(dst, byte(DataTypeInteger), int(i))
	}
	dst = appendType(dst, DataTypeInteger)
	dst = strconv.AppendInt(dst, i, 10)
	dst = appendTerminator(dst)
	return dst
}

// AppendUint appends the RESP integer from the unsigned integer i
// to dst and returns the extended buffer.
func AppendUint(dst []byte, i uint) []byte {
	return AppendUint64(dst, uint64(i))
}

// AppendUint32 appends the RESP integer from the 32-bit unsigned integer i
// to dst and returns the extended buffer.
func AppendUint32(dst []byte, i uint32) []byte {
	return AppendUint64(dst, uint64(i))
}

// AppendUint64 appends the RESP integer from the 64-bit unsigned integer i
// to dst and returns the extended buffer.
func AppendUint64(dst []byte, i uint64) []byte {
	if i <= 9 {
		return appendSmall(dst, byte(DataTypeInteger), int(i))
	}
	dst = appendType(dst, DataTypeInteger)
	dst = strconv.AppendUint(dst, i, 10)
	dst = appendTerminator(dst)
	return dst
}

// AppendString appends the RESP bulk string form of the string s
// to dst and returns the extended buffer.
func AppendString(dst []byte, s string) []byte {
	dst = appendPrefix(dst, byte(DataTypeBulkString), len(s))
	dst = append(dst, s...)
	dst = appendTerminator(dst)
	return dst
}

// AppendBytes appends the RESP bulk string form of the bytes slice b
// to dst and returns the extended buffer.
func AppendBytes(dst []byte, b []byte) []byte {
	dst = appendPrefix(dst, byte(DataTypeBulkString), len(b))
	dst = append(dst, b...)
	dst = appendTerminator(dst)
	return dst
}

// AppendNull appends the RESP null to dst and returns the extended buffer.
func AppendNull(dst []byte) []byte {
	return append(dst, byte(DataTypeBulkString), '-', '1', '\r', '\n')
}

// AppendNull appends the RESP array type of n elements to dst and returns the
// extended buffer.
func AppendArray(dst []byte, n int) []byte {
	return appendPrefix(dst, byte(DataTypeArray), n)
}

func appendType(dst []byte, t DataType) []byte {
	return append(dst, byte(t))
}

func appendTerminator(dst []byte) []byte {
	return append(dst, '\r', '\n')
}

func appendPrefix(dst []byte, prefix byte, n int) []byte {
	if n >= 0 && n <= 9 {
		return appendSmall(dst, prefix, n)
	}
	dst = append(dst, prefix)
	dst = strconv.AppendInt(dst, int64(n), 10)
	dst = appendTerminator(dst)
	return dst
}

func appendSmall(dst []byte, prefix byte, i int) []byte {
	return append(dst, prefix, byte('0'+i), '\r', '\n')
}

func appendString(dst []byte, s string) []byte {
	// It is better to do a double check than to just copy the string byte by
	// byte. But, of course, it would be better not to do it at all.
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\r', '\n':
			return appendEscapedString(dst, s)
		}
	}

	return append(dst, s...)
}

func appendEscapedString(dst []byte, s string) []byte {
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch ch {
		case '\r':
			dst = append(dst, '\\', 'r')

		case '\n':
			dst = append(dst, '\\', 'n')

		default:
			dst = append(dst, ch)
		}
	}

	return dst
}

const (
	defaultWriterBufSize = 4 * kibibyte
)

// Writer implements buffering for an io.Writer object.
//
// Writer provides wrappers for each Append function for easier use.
//
// After all data has been written, the client should call the Flush method to
// guarantee all data has been forwarded to the underlying io.Writer
type Writer struct {
	buf []byte
	w   *bufio.Writer
}

// NewWriter returns a new Writer writing RESP data types.
func NewWriter(w io.Writer) *Writer {
	return &Writer{
		buf: make([]byte, 0, defaultWriterBufSize),
		w:   bufio.NewWriter(w),
	}
}

// WriteSimpleString writes a RESP bulk string.
func (w *Writer) WriteSimpleString(s string) error {
	return w.write(func() { w.buf = AppendSimpleString(w.buf, s) })
}

// WriteError writes a RESP error.
func (w *Writer) WriteError(s string) error {
	return w.write(func() { w.buf = AppendError(w.buf, s) })
}

// WriteInt writes a RESP integer.
func (w *Writer) WriteInt(n int) error {
	return w.write(func() { w.buf = AppendInt(w.buf, n) })
}

// WriteInt32 writes a RESP integer.
func (w *Writer) WriteInt32(n int32) error {
	return w.write(func() { w.buf = AppendInt32(w.buf, n) })
}

// WriteInt64 writes a RESP integer.
func (w *Writer) WriteInt64(n int64) error {
	return w.write(func() { w.buf = AppendInt64(w.buf, n) })
}

// WriteUint writes a RESP integer.
func (w *Writer) WriteUint(n uint) error {
	return w.write(func() { w.buf = AppendUint(w.buf, n) })
}

// WriteUint32 writes a RESP integer.
func (w *Writer) WriteUint32(n uint32) error {
	return w.write(func() { w.buf = AppendUint32(w.buf, n) })
}

// WriteUint64 writes a RESP integer.
func (w *Writer) WriteUint64(n uint64) error {
	return w.write(func() { w.buf = AppendUint64(w.buf, n) })
}

// WriteString writes a RESP bulk string.
func (w *Writer) WriteString(s string) error {
	return w.write(func() { w.buf = AppendString(w.buf, s) })
}

// WriteBytes writes a RESP bulk string.
func (w *Writer) WriteBytes(b []byte) error {
	return w.write(func() { w.buf = AppendBytes(w.buf, b) })
}

// WriteBytes writes the RESP null.
func (w *Writer) WriteNull() error {
	return w.write(func() { w.buf = AppendNull(w.buf) })
}

// WriteBytes writes a RESP array type of n elements.
func (w *Writer) WriteArray(n int) error {
	return w.write(func() { w.buf = AppendArray(w.buf, n) })
}

// Flush writes any buffered data to the underlying io.Writer.
func (w *Writer) Flush() error {
	if len(w.buf) > 0 {
		_, err := w.w.Write(w.buf)
		w.buf = w.buf[:0]
		if err != nil {
			return err
		}
	}

	return w.w.Flush()
}

// write calls the function f, which should modify the buf, and then
// writes the buffered data to the bufio.Writer and resets the buf.
func (w *Writer) write(f func()) error {
	f()

	_, err := w.w.Write(w.buf)
	w.buf = w.buf[:0]
	return err
}
