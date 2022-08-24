package radish_test

import (
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/SuperPaintman/mini-redis/001_redis_protocol/radish"
)

func ExampleCommandReader() {
	input := strings.NewReader(
		"*3\r\n" +
			"$3\r\n" +
			"SET\r\n" +
			"$5\r\n" +
			"mykey\r\n" +
			"$7\r\n" +
			"myvalue\r\n",
	)
	commandReader := radish.NewCommandReader(input)

	for {
		cmd, err := commandReader.ReadCommand()
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatal(err)
		}

		fmt.Printf("Raw: %q\n", cmd.Raw)

		fmt.Printf("Args:\n")
		for i, arg := range cmd.Args {
			fmt.Printf("%d. %q\n", i, arg)
		}
	}

	// Output:
	// Raw: "*3\r\n$3\r\nSET\r\n$5\r\nmykey\r\n$7\r\nmyvalue\r\n"
	// Args:
	// 0. "SET"
	// 1. "mykey"
	// 2. "myvalue"
}

func ExampleCommandReader_pipelining() {
	input := strings.NewReader(
		"*2\r\n" +
			"$3\r\n" +
			"GET\r\n" +
			"$5\r\n" +
			"mykey\r\n" +
			"*3\r\n" +
			"$3\r\n" +
			"SET\r\n" +
			"$5\r\n" +
			"mykey\r\n" +
			"$7\r\n" +
			"myvalue\r\n",
	)
	commandReader := radish.NewCommandReader(input)

	for {
		cmd, err := commandReader.ReadCommand()
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatal(err)
		}

		fmt.Printf("Raw: %q\n", cmd.Raw)

		fmt.Printf("Args:\n")
		for i, arg := range cmd.Args {
			fmt.Printf("%d. %q\n", i, arg)
		}

		fmt.Printf("\n")
	}

	// Output:
	// Raw: "*2\r\n$3\r\nGET\r\n$5\r\nmykey\r\n"
	// Args:
	// 0. "GET"
	// 1. "mykey"
	//
	// Raw: "*3\r\n$3\r\nSET\r\n$5\r\nmykey\r\n$7\r\nmyvalue\r\n"
	// Args:
	// 0. "SET"
	// 1. "mykey"
	// 2. "myvalue"
}
