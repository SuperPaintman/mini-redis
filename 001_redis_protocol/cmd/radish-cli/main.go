package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/SuperPaintman/mini-redis/001_redis_protocol/radish"
)

var (
	hostname = flag.String("h", "127.0.0.1", "server hostname")
	port     = flag.Int("p", 6379, "server port")
)

func main() {
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		log.Fatal("REPL is not implemented yet")
	}

	address := fmt.Sprintf("%s:%d", *hostname, *port)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		log.Fatalf("Could not connect to Radish: %s", err)
	}
	defer conn.Close()

	writer := radish.NewWriter(conn)

	_ = writer.WriteArray(len(args))
	for _, arg := range args {
		_ = writer.WriteString(arg)
	}
	if err := writer.Flush(); err != nil {
		log.Fatalf("Could not write a command: %s", err)
	}

	reader := radish.NewReader(conn)

	readReaponse(reader, "")
}

func readReaponse(reader *radish.Reader, prefix string) {
	dt, v, err := reader.ReadAny()
	if err != nil {
		log.Fatalf("Could not read the response: %s", err)
	}

	fmt.Print(prefix)

	switch dt {
	case radish.DataTypeSimpleString:
		fmt.Printf("%s\n", v.(string))

	case radish.DataTypeError:
		e := v.(*radish.Error)
		fmt.Printf("(error) %s %s\n", e.Kind, e.Msg)

	case radish.DataTypeInteger:
		fmt.Printf("(integer) %d\n", v.(int))

	case radish.DataTypeBulkString:
		fmt.Printf("%q\n", v.(string))

	case radish.DataTypeArray:
		length := v.(int)
		if length == 0 {
			fmt.Print("(empty array)\n")
		} else {
			for i := 0; i < length; i++ {
				p := ""
				if i != 0 {
					// TODO
					p = strings.Repeat(" ", len(prefix))
				}

				readReaponse(reader, fmt.Sprintf("%s%d) ", p, i+1))
			}
		}

	case radish.DataTypeNull:
		fmt.Print("(nil)\n")

	default:
		log.Fatalf("Unknown data type: %q", dt)
	}
}
