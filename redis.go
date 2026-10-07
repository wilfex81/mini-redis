package main

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)


func readLiner(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(line, "\r\n"), nil
}

func readCommand(r *bufio.Reader) ([]string, error) {
	line, err := readLiner(r)
	if err != nil {
		return  nil, err
	}
	if len(line) == 0 || line[0] != '*' {
		return nil, fmt.Errorf("expected array, got %q", line)
	}
	count, err := strconv.Atoi(line[1:])
	if err != nil {
		return nil, err
	}

	args := make([]string, 0, count)
	for i := 0; i < count; i++ {
		header, err := readLiner(r)
		if err != nil {
			return nil, err
		}
		if len(header) == 0 || header[0] != '$' {
			return  nil, fmt.Errorf("expected bulk string, got %q", header)
		}
		size , err := strconv.Atoi(header[1:])
		if err != nil {
			return nil, err
		}

		buf := make([]byte, size+2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		args = append(args, string(buf[:size]))
	}
	return args, nil
	
}