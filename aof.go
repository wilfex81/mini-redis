package main

import (
	"bufio"
	"io"
	"os"
	"strings"
	"time"
)

type aof struct {
	file *os.File
	ch   chan string
	done chan struct{}
}

func openAOF(path string) (*aof, error) {
	// create if missing, read and write, always append at the end
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	a := &aof{
		file: f,
		ch:   make(chan string, 4096),
		done: make(chan struct{}),
	}
	go a.loop()
	return a, nil
}

// Only section that writes to file
func (a *aof) loop() {
	defer close(a.done)
	w := bufio.NewWriter(a.file)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	flush := func() {
		w.Flush()
		a.file.Sync()
	}

	for {
		select {
		case cmd, ok := <-a.ch:
			if !ok {
				flush()
				return
			}
			w.WriteString(cmd)
		case <-ticker.C:
			flush()
		}
	}
}

// Write queues a command. It returns right away
func (a *aof) write(args []string) {
	a.ch <- arrayReply(args)
}

// flushes everything and stops the writer
func (a *aof) close() {
	close(a.ch)
	<-a.done
	a.file.Close()
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
