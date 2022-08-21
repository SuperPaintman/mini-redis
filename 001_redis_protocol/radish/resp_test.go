package radish

import (
	"bytes"
	"testing"
)

func testAppend(t testing.TB, name string, want []byte, f func([]byte) []byte) {
	t.Helper()

	buf := make([]byte, 64)
	got := f(buf[:0])

	if buf[:1][0] == 0 {
		t.Errorf("%s(): buf was not modified: buf = %q, got = %q",
			name,
			buf[:64],
			got,
		)
	}

	if !bytes.Equal(want, got) {
		t.Errorf("%s() = %q, want %q",
			name,
			got,
			want,
		)
	}
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

func TestAppendSimpleString(t *testing.T) {
	for _, tc := range testSimpleStrings {
		t.Run(tc.name, func(t *testing.T) {
			testAppend(t, "AppendSimpleString", tc.want, func(b []byte) []byte {
				return AppendSimpleString(b, tc.s)
			})
		})
	}
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

func TestAppendError(t *testing.T) {
	for _, tc := range testErrors {
		t.Run(tc.name, func(t *testing.T) {
			testAppend(t, "AppendError", tc.want, func(b []byte) []byte {
				return AppendError(b, tc.s)
			})
		})
	}
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

func TestAppendInt(t *testing.T) {
	for _, tc := range testInts {
		t.Run(tc.name, func(t *testing.T) {
			testAppend(t, "AppendInt", tc.want, func(b []byte) []byte {
				return AppendInt(b, int(tc.i))
			})
		})
	}
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

func TestAppendInt32(t *testing.T) {
	for _, tc := range testInts {
		t.Run(tc.name, func(t *testing.T) {
			testAppend(t, "AppendInt32", tc.want, func(b []byte) []byte {
				return AppendInt32(b, int32(tc.i))
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
func TestAppendInt64(t *testing.T) {
	for _, tc := range testInts {
		t.Run(tc.name, func(t *testing.T) {
			testAppend(t, "AppendInt64", tc.want, func(b []byte) []byte {
				return AppendInt64(b, tc.i)
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

func TestAppendUint(t *testing.T) {
	for _, tc := range testUints {
		t.Run(tc.name, func(t *testing.T) {
			testAppend(t, "AppendUint", tc.want, func(b []byte) []byte {
				return AppendUint(b, uint(tc.i))
			})
		})
	}
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

func TestAppendUint32(t *testing.T) {
	for _, tc := range testUints {
		t.Run(tc.name, func(t *testing.T) {
			testAppend(t, "AppendUint32", tc.want, func(b []byte) []byte {
				return AppendUint32(b, uint32(tc.i))
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

func TestAppendUint64(t *testing.T) {
	for _, tc := range testUints {
		t.Run(tc.name, func(t *testing.T) {
			testAppend(t, "AppendUint64", tc.want, func(b []byte) []byte {
				return AppendUint64(b, tc.i)
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

func TestAppendString(t *testing.T) {
	for _, tc := range testBulkStrings {
		t.Run(tc.name, func(t *testing.T) {
			testAppend(t, "AppendString", tc.want, func(b []byte) []byte {
				return AppendString(b, string(tc.b))
			})
		})
	}
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

func TestAppendBytes(t *testing.T) {
	for _, tc := range testBulkStrings {
		t.Run(tc.name, func(t *testing.T) {
			testAppend(t, "AppendBytes", tc.want, func(b []byte) []byte {
				return AppendBytes(b, tc.b)
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

func TestAppendNull(t *testing.T) {
	want := []byte("$-1\r\n")
	testAppend(t, "AppendNull", want, func(b []byte) []byte {
		return AppendNull(b)
	})
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

func TestAppendArray(t *testing.T) {
	for _, tc := range testArrays {
		t.Run(tc.name, func(t *testing.T) {
			testAppend(t, "AppendArray", tc.want, func(b []byte) []byte {
				return AppendArray(b, tc.n)
			})
		})
	}
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

var appendIntRes []byte

func BenchmarkAppendInt(b *testing.B) {
	var buf []byte

	for i := 0; i < b.N; i++ {
		buf = AppendInt(buf, i)
		buf = buf[:0]
	}

	appendIntRes = buf
}
