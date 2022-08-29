//> stage "Redis Protocol"
//> snippet radish-cli
package main

import (
	"flag"
	"fmt"

	//^ remove-lines: before=1, after=1

	//> snippet radish-cli-import-ioutil: uncomment-lines
	// "io/ioutil"
	//< snippet radish-cli-import-ioutil
	//> snippet radish-cli-import-ioutil-remove replaces radish-cli-import-ioutil
	//< snippet radish-cli-import-ioutil-remove
	"log"
	//> snippet radish-cli-read-response-array-import-math
	"math"
	//< snippet radish-cli-read-response-array-import-math
	"net"
	//> snippet radish-cli-read-response-array-import-str
	"strconv"
	"strings"

	//^ remove-lines: before=1
	//< snippet radish-cli-read-response-array-import-str
	//> snippet radish-cli-radish-import

	"github.com/SuperPaintman/mini-redis/radish"
	//< snippet radish-cli-radish-import
)

var (
	hostname = flag.String("h", "127.0.0.1", "server hostname")
	port     = flag.Int("p", 6379, "server port")
)

func main() {
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		log.Fatal("Interactive mode is not implemented yet")
	}

	address := fmt.Sprintf("%s:%d", *hostname, *port)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		log.Fatalf("Could not connect to Radish: %s", err)
	}
	defer conn.Close()
	//> snippet radish-cli-writer

	writer := radish.NewWriter(conn)

	_ = writer.WriteArray(len(args))
	for _, arg := range args {
		_ = writer.WriteString(arg)
	}
	if err := writer.Flush(); err != nil {
		log.Fatalf("Could not write a command: %s", err)
	}
	//< snippet radish-cli-writer
	//> snippet radish-cli-readall: uncomment-lines
	//
	// if err := conn.(*net.TCPConn).CloseWrite(); err != nil {
	// 	log.Fatalf("Could not close the TCP writer: %s", err)
	// }
	//
	// res, err := ioutil.ReadAll(conn)
	// if err != nil {
	// 	log.Fatalf("Could not read the response: %s", err)
	// }
	//
	// fmt.Printf("%q\n", res)
	//< snippet radish-cli-readall
	//> snippet radish-cli-reader replaces radish-cli-readall

	reader := radish.NewReader(conn)
	readResponse(reader, "")
	//< snippet radish-cli-reader
}

//^ remove-lines: before=1
//< snippet radish-cli

//> snippet radish-cli-read-response
func readResponse(reader *radish.Reader, indent string) {
	dt, v, err := reader.ReadAny()
	if err != nil {
		log.Fatalf("Could not read the response: %s", err)
	}

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

	case radish.DataTypeNull:
		fmt.Print("(nil)\n")

	//> snippet radish-cli-read-response-array
	case radish.DataTypeArray:
		length := v.(int)
		if length == 0 {
			fmt.Print("(empty array)\n")
		} else {
			prefixWidth := int(math.Log10(float64(length))) + 1
			prefixFormat := "%" + strconv.Itoa(prefixWidth) + "d) " // "%2d"-like.
			nextIndent := indent + strings.Repeat(" ", prefixWidth+len(") "))

			for i := 0; i < length; i++ {
				if i != 0 {
					fmt.Print(indent)
				}
				fmt.Printf(prefixFormat, i+1)

				readResponse(reader, nextIndent)
			}
		}

	//< snippet radish-cli-read-response-array
	default:
		log.Fatalf("Unknown data type: %q", dt)
	}
}

//^ remove-lines: before=1
//< snippet radish-cli-read-response
