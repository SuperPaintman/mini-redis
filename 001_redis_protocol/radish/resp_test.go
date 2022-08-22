package radish

import (
	"bytes"
	"testing"
)

var longString string // ~16KB

func init() {
	for i := 0; i < 16*1024; i++ {
		longString += "very-"
	}

	longString += "long-string"
}

func testWriter(t testing.TB, name string, want []byte, f func(*Writer) error) {
	t.Helper()

	var buf bytes.Buffer
	writer := NewWriter(&buf)

	if err := f(writer); err != nil {
		t.Fatalf("%s(): unexpected error: %v", name, err)
	}

	if err := writer.Flush(); err != nil {
		t.Fatalf("Flush(): unexpected error: %v", err)
	}

	got := buf.Bytes()
	if !bytes.Equal(want, got) {
		t.Errorf("%s() = %q, want %q",
			name,
			got,
			want,
		)
	}
}

var testSimpleStrings = []struct {
	name       string
	s          string
	want       []byte
	wantUnsafe []byte
}{
	{
		name: "empty",
		s:    "",
		want: []byte("+\r\n"),
	},
	{
		name: "string",
		s:    "SET",
		want: []byte("+SET\r\n"),
	},
	{
		name:       "with newlines",
		s:          "hello\n\nfrom\rredis\t!",
		want:       []byte("+hello\\n\\nfrom\\rredis\t!\r\n"),
		wantUnsafe: []byte("+hello\n\nfrom\rredis\t!\r\n"),
	},
}

func TestWriter_WriteSimpleString(t *testing.T) {
	for _, tc := range testSimpleStrings {
		t.Run(tc.name, func(t *testing.T) {
			testWriter(t, "WriteSimpleString", tc.want, func(w *Writer) error {
				return w.WriteSimpleString(tc.s)
			})
		})
	}
}

var testErrors = []struct {
	name       string
	s          string
	want       []byte
	wantUnsafe []byte
}{
	{
		name: "empty",
		s:    "",
		want: []byte("-\r\n"),
	},
	{
		name: "error",
		s:    "ERR Protocol error: expected '$', got ' '",
		want: []byte("-ERR Protocol error: expected '$', got ' '\r\n"),
	},
	{
		name:       "with newlines",
		s:          "ERR\n\nBroken\rerror\t!",
		want:       []byte("-ERR\\n\\nBroken\\rerror\t!\r\n"),
		wantUnsafe: []byte("-ERR\n\nBroken\rerror\t!\r\n"),
	},
}

func TestWriter_WriteError(t *testing.T) {
	for _, tc := range testErrors {
		t.Run(tc.name, func(t *testing.T) {
			testWriter(t, "WriteError", tc.want, func(w *Writer) error {
				return w.WriteError(tc.s)
			})
		})
	}
}

var testInts = []struct {
	name string
	i    int64
	want []byte
}{
	{
		name: "zero",
		i:    0,
		want: []byte(":0\r\n"),
	},
	{
		name: "positive",
		i:    1337,
		want: []byte(":1337\r\n"),
	},
	{
		name: "negative",
		i:    -1337,
		want: []byte(":-1337\r\n"),
	},
	{
		name: "small",
		i:    7,
		want: []byte(":7\r\n"),
	},
}

func TestWriter_WriteInt(t *testing.T) {
	for _, tc := range testInts {
		t.Run(tc.name, func(t *testing.T) {
			testWriter(t, "WriteInt", tc.want, func(w *Writer) error {
				return w.WriteInt(int(tc.i))
			})
		})
	}
}

func TestWriter_WriteInt32(t *testing.T) {
	for _, tc := range testInts {
		t.Run(tc.name, func(t *testing.T) {
			testWriter(t, "WriteInt32", tc.want, func(w *Writer) error {
				return w.WriteInt32(int32(tc.i))
			})
		})
	}
}

func TestWriter_WriteInt64(t *testing.T) {
	for _, tc := range testInts {
		t.Run(tc.name, func(t *testing.T) {
			testWriter(t, "WriteInt64", tc.want, func(w *Writer) error {
				return w.WriteInt64(tc.i)
			})
		})
	}
}

var testUints = []struct {
	name string
	i    uint64
	want []byte
}{
	{
		name: "zero",
		i:    0,
		want: []byte(":0\r\n"),
	},
	{
		name: "positive",
		i:    1337,
		want: []byte(":1337\r\n"),
	},
	{
		name: "small",
		i:    7,
		want: []byte(":7\r\n"),
	},
}

func TestWriter_WriteUint(t *testing.T) {
	for _, tc := range testUints {
		t.Run(tc.name, func(t *testing.T) {
			testWriter(t, "WriteUint", tc.want, func(w *Writer) error {
				return w.WriteUint(uint(tc.i))
			})
		})
	}
}

func TestWriter_WriteUint32(t *testing.T) {
	for _, tc := range testUints {
		t.Run(tc.name, func(t *testing.T) {
			testWriter(t, "WriteUint32", tc.want, func(w *Writer) error {
				return w.WriteUint32(uint32(tc.i))
			})
		})
	}
}

func TestWriter_WriteUint64(t *testing.T) {
	for _, tc := range testUints {
		t.Run(tc.name, func(t *testing.T) {
			testWriter(t, "WriteUint64", tc.want, func(w *Writer) error {
				return w.WriteUint64(tc.i)
			})
		})
	}
}

var testBulkStrings = []struct {
	name string
	b    []byte
	want []byte
}{
	{
		name: "empty",
		b:    []byte(""),
		want: []byte("$0\r\n\r\n"),
	},
	{
		name: "string",
		b:    []byte("SET"),
		want: []byte("$3\r\nSET\r\n"),
	},
	{
		name: "with newlines",
		b:    []byte("hello\n\nfrom\rredis\t!"),
		want: []byte("$19\r\nhello\n\nfrom\rredis\t!\r\n"),
	},
}

func TestWriter_WriteString(t *testing.T) {
	for _, tc := range testBulkStrings {
		t.Run(tc.name, func(t *testing.T) {
			testWriter(t, "WriteString", tc.want, func(w *Writer) error {
				return w.WriteString(string(tc.b))
			})
		})
	}
}

func TestWriter_WriteBytes(t *testing.T) {
	for _, tc := range testBulkStrings {
		t.Run(tc.name, func(t *testing.T) {
			testWriter(t, "WriteBytes", tc.want, func(w *Writer) error {
				return w.WriteBytes(tc.b)
			})
		})
	}
}

func TestWriter_WriteNull(t *testing.T) {
	want := []byte("$-1\r\n")
	testWriter(t, "WriteNull", want, func(w *Writer) error {
		return w.WriteNull()
	})
}

var testArrays = []struct {
	name string
	n    int
	want []byte
}{
	{
		name: "empty",
		n:    0,
		want: []byte("*0\r\n"),
	},
	{
		name: "array",
		n:    1337,
		want: []byte("*1337\r\n"),
	},
	{
		name: "null array",
		n:    -1,
		want: []byte("*-1\r\n"),
	},
}

func TestWriter_WriteArray(t *testing.T) {
	for _, tc := range testArrays {
		t.Run(tc.name, func(t *testing.T) {
			testWriter(t, "WriteArray", tc.want, func(w *Writer) error {
				return w.WriteArray(tc.n)
			})
		})
	}
}

var writerRes []byte

func BenchmarkWriter(b *testing.B) {
	bt := []struct {
		name string
		s    string
	}{
		{"short", "test"},
		{"long", longString},
	}

	for _, bc := range bt {
		b.Run(bc.name, func(b *testing.B) {
			var buf bytes.Buffer
			writer := NewWriter(&buf)

			for i := 0; i < b.N; i++ {
				writer.WriteArray(3)
				writer.WriteString("SET")
				writer.WriteString("test")
				writer.WriteString(bc.s)
				writer.Flush()

				writerRes = buf.Bytes()

				buf.Reset()
				writer.Reset(&buf)
			}
		})
	}
}
