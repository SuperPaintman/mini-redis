package radish

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func buildRawCommand(t testing.TB, args []Arg) []byte {
	t.Helper()

	var buf bytes.Buffer
	cr := NewWriter(&buf)

	_ = cr.WriteArray(len(args))
	for _, arg := range args {
		_ = cr.WriteBytes(arg)
	}

	if err := cr.Flush(); err != nil {
		t.Fatalf("unexpected error: failed to flush the writer to the buffer: %v", err)
	}

	return buf.Bytes()
}

func TestCommandReader_ReadCommand(t *testing.T) {
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
			reader := NewCommandReader(input)

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

			rest, err := reader.ReadCommand()
			if !errors.Is(err, io.EOF) {
				t.Fatalf("ReadCommand() error = %v, want %v", err, io.EOF)
			}

			if rest != nil {
				t.Errorf("ReadCommand() returned an extra command: %q", rest.Raw)
			}
		})
	}
}

var readCommandRes *Command

func BenchmarkCommandReader_ReadCommand(b *testing.B) {
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
			reader := NewCommandReader(input)

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
