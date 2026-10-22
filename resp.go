package main

import (
	"bytes"
	"fmt"
	"strconv"
)

const (
	maxArgs    = 1024
	maxBulkLen = 64 * 1024 * 1024
)

func lineAt(buf []byte, pos int) (string, int, bool) {
	i := bytes.Index(buf[pos:], []byte("\r\n"))
	if i < 0 {
		return "", 0, false
	}
	return string(buf[pos : pos+i]), pos + i + 2, true
}

// Tries to read one full command from buf.
func parseCommand(buf []byte) ([]string, int, error) {
	line, pos, ok := lineAt(buf, 0)
	if !ok {
		return nil, 0, nil
	}
	if len(line) == 0 || line[0] != '*' {
		return nil, 0, fmt.Errorf("expected array, got %q", line)
	}
	count, err := strconv.Atoi(line[1:])
	if err != nil {
		return nil, 0, err
	}
	if count < 0 || count > maxArgs {
		return nil, 0, fmt.Errorf("invalid argument count %d", count)
	}

	args := make([]string, 0, count)
	for i := 0; i < count; i++ {
		header, next, ok := lineAt(buf, pos)
		if !ok {
			return nil, 0, nil
		}
		if len(header) == 0 || header[0] != '$' {
			return nil, 0, fmt.Errorf("expected bulk string, got %q", header)
		}
		size, err := strconv.Atoi(header[1:])
		if err != nil {
			return nil, 0, err
		}
		if size < 0 || size > maxBulkLen {
			return nil, 0, fmt.Errorf("invalid bulk length %d", size)
		}
		if len(buf) < next+size+2 {
			return nil, 0, nil // argument not fully arrived yet
		}
		args = append(args, string(buf[next:next+size]))
		pos = next + size + 2
	}
	return args, pos, nil
}
