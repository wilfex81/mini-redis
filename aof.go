package main

import (
	"bufio"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

type aof struct {
	file *os.File
	ch   chan string
	done chan struct{}
}

func openAOF(path string) (*aof, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
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

func (a *aof) write(args []string) {
	a.ch <- arrayReply(args)
}

func (a *aof) close() {
	close(a.ch)
	<-a.done
	a.file.Close()
}

// loadAOF runs every logged command against the store.
// A missing file is fine, it just means a fresh start.
func loadAOF(path string, db *Store) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	defer f.Close()

	reader := bufio.NewReader(f)
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

// compactAOF writes the current state to a temp file,
// then swaps it over the old file in one atomic step
func compactAOF(path string, db *Store) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	for _, cmd := range db.snapshot() {
		if _, err := w.WriteString(arrayReply(cmd)); err != nil {
			f.Close()
			return err
		}
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// relative expiries become absolute, so a restart does not restart the clock
func journalForm(args []string) [][]string {
	switch strings.ToUpper(args[0]) {
	case "EXPIRE":
		secs, _ := strconv.Atoi(args[2])
		return [][]string{{"PEXPIREAT", args[1], absMillis(secs)}}
	case "SET":
		if len(args) == 5 { // SET key value EX seconds
			secs, _ := strconv.Atoi(args[4])
			return [][]string{
				{"SET", args[1], args[2]},
				{"PEXPIREAT", args[1], absMillis(secs)},
			}
		}
	}
	return [][]string{args}
}

// absMillis returns "now plus secs" as unix milliseconds, in text form
func absMillis(secs int) string {
	t := time.Now().Add(time.Duration(secs) * time.Second)
	return strconv.FormatInt(t.UnixMilli(), 10)
}

var writeCommands = map[string]bool{
	"SET": true, "DEL": true, "EXPIRE": true, "INCR": true, "DECR": true,
	"HSET": true, "LPUSH": true, "RPUSH": true,
}

func isWrite(args []string) bool {
	return len(args) > 0 && writeCommands[strings.ToUpper(args[0])]
}
