package radish_test

import (
	"fmt"
	"log"
	"strings"

	"github.com/SuperPaintman/mini-redis/001_redis_protocol/radish"
)

func ExampleWriter() {
	var output strings.Builder
	writer := radish.NewWriter(&output)

	// Simple strings.
	_ = writer.WriteSimpleString("OK")
	if err := writer.Flush(); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Simple string: %q\n", output.String())

	// Errors.
	output.Reset()
	writer.Reset(&output)
	_ = writer.WriteError(&radish.Error{Kind: "ERR", Msg: "unknown command 'GO'"})
	if err := writer.Flush(); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Error: %q\n", output.String())

	// Arrays.
	output.Reset()
	writer.Reset(&output)
	_ = writer.WriteArray(3)
	_ = writer.WriteString("SET")
	_ = writer.WriteString("mykey")
	_ = writer.WriteString("myvalue")
	if err := writer.Flush(); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Array: %q\n", output.String())

	// Output:
	// Simple string: "+OK\r\n"
	// Error: "-ERR unknown command 'GO'\r\n"
	// Array: "*3\r\n$3\r\nSET\r\n$5\r\nmykey\r\n$7\r\nmyvalue\r\n"
}
