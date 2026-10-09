package main

import (
	"fmt"
	"strings"
)

func simpleString(s string) string {
	return "+" + s + "\r\n"
}
func errorReply(s string) string {
	return "-ERR " + s + "\r\n"
}
func bulkString(s string) string {
	return fmt.Sprintf("$%d\r\n%s\r\n", len(s), s)
}

func intReply(n int) string {
	return fmt.Sprintf(":%d\r\n", n)
}

const nilReply = "$-1\r\n"

func execute(args []string) string {
	if len(args) == 0 {
		return errorReply("empty command")
	}

	switch strings.ToUpper(args[0]) {
	case "PING":
		if len(args) > 1 {
			return bulkString(args[1])
		}
		return simpleString("PONG")

	case "ECHO":
		if len(args) != 2 {
			return errorReply("Wrong number of arguments for 'echo' command")
		}
		return bulkString(args[1])

	default:
		return errorReply("Unkown command '" + args[0] + "'")
	}
}
