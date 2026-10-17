package main

import (
	"bufio"
	"io"
	"os"
	"strings"
	"sync"
)

type aof struct {
	mu   sync.Mutex
	file *os.File
}

func openAOF(path string) (*aof, error) {
	// create if missing, read and write, always append at the end
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	return &aof{file: f}, nil
}

// Write appends one command to the file and flushes and flushes it to disk
func (a *aof) write(args []string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, err := a.file.WriteString(arrayReply(args)); err != nil {
		return err
	}
	return a.file.Sync()
}

// replay runs every logged command against the Store.
// It returns how many commands it ran
func (a *aof) replay(db *Store) (int, error) {
	reader := bufio.NewReader(a.file)
	count := 0
	for {
		args, err := readCommand(reader)
		if err != nil {
			if err == io.EOF {
				return count, nil
			}
			return count, err
		}
		execute(db, args)
		count++
	}
}

var writeCommands = map[string]bool{
	"SET": true, "DEL": true, "EXPIRE": true, "INCR": true, "DECR": true,
	"HSET": true, "LPUSH": true, "RPUSH": true,
}

func isWrite(args []string) bool {
	return len(args) > 0 && writeCommands[strings.ToUpper(args[0])]
}
