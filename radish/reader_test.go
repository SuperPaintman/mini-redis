//> stage "Redis Protocol"
package radish

import (
	"bytes"
	"io"
	"reflect"
	"testing"
)

func buildRawCommand(t testing.TB, args []Arg) []byte {
	t.Helper()

	var buf bytes.Buffer
	writer := NewWriter(&buf)

	_ = writer.WriteArray(len(args))
	for _, arg := range args {
		_ = writer.WriteBytes(arg)
	}

	if err := writer.Flush(); err != nil {
		t.Fatalf("unexpected error: failed to flush the writer to the buffer: %v", err)
	}

	return buf.Bytes()
}

func TestReader_ReadCommand(t *testing.T) {
	type testCase struct {
		name  string
		input []byte
		want  []*Command
	}

	buildTestCase := func(t testing.TB, name string, commands [][]Arg) testCase {
		t.Helper()

		var (
			input []byte
			want  []*Command
		)
		for _, args := range commands {
			raw := buildRawCommand(t, args)

			input = append(input, raw...)
			want = append(want, &Command{
				Raw:  raw,
				Args: args,
			})
		}

		return testCase{name, input, want}
	}

	tt := []testCase{
		buildTestCase(t, "ping", [][]Arg{
			{
				Arg("PING"),
			},
		}),
		buildTestCase(t, "set", [][]Arg{
			{
				Arg("SET"),
				Arg("test-key"),
				Arg("test-value"),
			},
		}),
		buildTestCase(t, "pipeline", [][]Arg{
			{
				Arg("SET"),
				Arg("test-key"),
				Arg("test-value"),
			},
			{
				Arg("PING"),
			},
			{
				Arg("SET"),
				Arg("test-key"),
				Arg("test-value"),
			},
			{
				Arg("PING"),
			},
		}),
		buildTestCase(t, "long input", [][]Arg{
			{
				Arg("SET"),
				Arg(longString),
				Arg("test-value"),
			},
		}),
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			input := bytes.NewBuffer(tc.input)
			reader := NewReader(input)

			for i := 0; i < len(tc.want); i++ {
				got, err := reader.ReadCommand()
				if err != nil {
					t.Fatalf("ReadCommand() #%d returned unexpected error: %v", i, err)
				}

				want := tc.want[i]
				if !bytes.Equal(got.Raw, want.Raw) {
					t.Errorf("ReadCommand() #%d raw = %q, want %q",
						i,
						got.Raw,
						want.Raw,
					)
				}

				if len(got.Args) != len(want.Args) {
					t.Errorf("ReadCommand() #%d number of args = %d, want %d",
						i,
						len(got.Args),
						len(want.Args),
					)
				} else {
					for j := 0; j < len(want.Args); j++ {
						if !bytes.Equal(got.Args[j], want.Args[j]) {
							t.Errorf("ReadCommand() #%d arg[%d] = %q, want %q",
								i,
								j,
								got.Args[j],
								want.Args[j],
							)
						}
					}
				}
			}

			next, err := reader.ReadCommand()
			if err != io.EOF {
				t.Fatalf("ReadCommand() error = %v, want %v", err, io.EOF)
			}

			if next != nil {
				t.Errorf("ReadCommand() returned an extra command: %q", next.Raw)
			}
		})
	}
}

func TestReader_ReadAny(t *testing.T) {
	tt := []struct {
		name         string
		input        []byte
		wantDataType DataType
		wantValue    interface{}
	}{
		{
			name:         "simple string",
			input:        []byte("+OK\r\n"),
			wantDataType: DataTypeSimpleString,
			wantValue:    "OK",
		},
		{
			name:         "simple string with newlines",
			input:        []byte("+OK\n \r\r\n"),
			wantDataType: DataTypeSimpleString,
			wantValue:    "OK\n \r",
		},
		{
			name:         "empty simple string",
			input:        []byte("+\r\n"),
			wantDataType: DataTypeSimpleString,
			wantValue:    "",
		},
		{
			name:         "error",
			input:        []byte("-ERR unknown command 'GO'\r\n"),
			wantDataType: DataTypeError,
			wantValue:    &Error{"ERR", "unknown command 'GO'"},
		},
		{
			name:         "error with newlines",
			input:        []byte("-ERR unknown\r command\n 'GO'\n\r\n"),
			wantDataType: DataTypeError,
			wantValue:    &Error{"ERR", "unknown\r command\n 'GO'\n"},
		},
		{
			name:         "error without msg",
			input:        []byte("-ERR\r\n"),
			wantDataType: DataTypeError,
			wantValue:    &Error{"ERR", ""},
		},
		{
			name:         "empty error",
			input:        []byte("-\r\n"),
			wantDataType: DataTypeError,
			wantValue:    &Error{"", ""},
		},
		{
			name:         "integer",
			input:        []byte(":1337\r\n"),
			wantDataType: DataTypeInteger,
			wantValue:    1337,
		},
		{
			name:         "negative integer",
			input:        []byte(":-1337\r\n"),
			wantDataType: DataTypeInteger,
			wantValue:    -1337,
		},
		{
			name:         "bulk string",
			input:        []byte("$11\r\nhello world\r\n"),
			wantDataType: DataTypeBulkString,
			wantValue:    "hello world",
		},
		{
			name:         "empty bulk string",
			input:        []byte("$0\r\n\r\n"),
			wantDataType: DataTypeBulkString,
			wantValue:    "",
		},
		{
			name:         "null",
			input:        []byte("$-1\r\n"),
			wantDataType: DataTypeNull,
			wantValue:    nil,
		},
		{
			name:         "array",
			input:        []byte("*10\r\n"),
			wantDataType: DataTypeArray,
			wantValue:    10,
		},
		{
			name:         "null array",
			input:        []byte("*-1\r\n"),
			wantDataType: DataTypeArray,
			wantValue:    -1,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			input := bytes.NewBuffer(tc.input)
			reader := NewReader(input)

			dt, v, err := reader.ReadAny()
			if err != nil {
				t.Fatalf("ReadAny() returned unexpected error: %v", err)
			}

			if dt != tc.wantDataType {
				t.Errorf("ReadCommand() data type = %q, want %q",
					dt,
					tc.wantDataType,
				)
			}

			if !reflect.DeepEqual(v, tc.wantValue) {
				t.Errorf("ReadCommand() value = %#v, want %#v",
					v,
					tc.wantValue,
				)
			}

			nextDataType, nextValue, err := reader.ReadAny()
			if err != io.EOF {
				t.Fatalf("ReadAny() error = %v, want %v", err, io.EOF)
			}

			if nextDataType != DataTypeNull {
				t.Errorf("ReadAny() returned an extra data type: %q", nextDataType)
			}

			if nextValue != nil {
				t.Errorf("ReadAny() returned an extra value: %#v", nextValue)
			}
		})
	}
}

var readCommandRes *Command

func BenchmarkReader_ReadCommand(b *testing.B) {
	bt := []struct {
		name  string
		input []byte
	}{
		{
			name: "short",
			input: buildRawCommand(b, []Arg{
				Arg("SET"),
				Arg("test-key"),
				Arg("test-value"),
			}),
		},
		{
			name: "long",
			input: buildRawCommand(b, []Arg{
				Arg("SET"),
				Arg(longString),
				Arg("test-value"),
			}),
		},
	}

	for _, bc := range bt {
		b.Run(bc.name, func(b *testing.B) {
			input := bytes.NewReader(bc.input)
			reader := NewReader(input)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				cmd, err := reader.ReadCommand()
				if err != nil {
					b.Fatal(err)
				}

				readCommandRes = cmd

				input.Reset(bc.input)
				reader.Reset(input)

				commandPool.Put(cmd)
			}
		})
	}
}
