---
title: Welcome
draft: true
---

## Hello

```go
// HI
```

```redis
*2\r\n
$3\r\n
GET\r\n
$5\r\n
hello\r\n

$-1\r\n

+OK\r\n
:1337\r\n
+1337\r\n
-ERR Something went wrong\r\n


*2\r\n$3\r\nGET\r\n$5\r\nhello\r\n+OK\r\n*2\r\n$3\r\nGET\r\n$5\r\nhello\r\n
```

^snippet radish-cli

^snippet radish-cli-writer: before=1, after=1

^snippet radish-cli-import-radish: before=1, after=1

^snippet radish-cli-readall: before=2, after=1

^snippet radish-cli-import-ioutil: before=1, after=1

^snippet radish-cli-reader: before=2, after=1

^snippet radish-cli-import-ioutil-remove

^snippet radish-cli-read-response

^snippet radish-cli-read-response-array: before=3, after=2

## Test

^snippet radish-cli-read-response-array-import-math: before=1, after=1

## Test

^snippet radish-cli-read-response-array-import-str: before=1, after=2

## Writer

^snippet writer

^snippet writer-data-types

^snippet writer-write-simple-string

^snippet writer-write-type

^snippet writer-write-string

^snippet writer-write-terminator

^snippet writer-error

^snippet writer-write-error

^snippet writer-write-ints

^snippet writer-write-int

^snippet writer-import-strconv: before=1, after=1

^snippet writer-writer-smallbuf-field: before=2, after=1

^snippet writer-writer-smallbuf-size: before=1, after=1

^snippet writer-writer-smallbuf-init: before=2, after=2

^snippet writer-write-uints

^snippet writer-write-uint

^snippet writer-write-bulk

^snippet writer-write-prefix

^snippet writer-write-null

^snippet writer-write-array

## Reader

^snippet reader

^snippet reader-command

^snippet reader-read-command

^snippet reader-read-command-cmd: before=1, after=2

^snippet reader-read-command-array-length: before=3, after=2

^snippet reader-errors

^snippet reader-import-errors: before=1, after=1

^snippet reader-read-value

^snippet reader-error-line-limit-exceeded: before=1, after=1

^snippet reader-read-line

^snippet reader-has-terminator

^snippet reader-parse-int

^snippet reader-read-command-parse-elements: before=3, after=2

^snippet reader-error-bulk-length: before=1, after=2

^snippet reader-read-bulk

^snippet reader-command-grow

^snippet reader-command-pool

^snippet reader-import-sync: before=1, after=1

^snippet reader-read-command-from-pool: before=1, after=4

^snippet reader-read-command-preallocate-args: before=3, after=2

^snippet reader-read-simple-string

^snippet reader-read-error

^snippet reader-read-integer

^snippet reader-error-integer-value: before=1, after=2

^snippet reader-read-string

^snippet reader-read-array

^snippet reader-read-any

^snippet reader-import-fmt: before=1, after=1

^snippet writer-data-type-null: before=1, after=1
