package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
)

func main() {
	// Listen for TCP connection on port 6380
	listener, err := net.Listen("tcp", ":6380")
	if err != nil {
		fmt.Println("listen error:", err)
		return
	}
	defer listener.Close()

	db := newStore()

	journal, err := openAOF("appendonly.aof")
	if err != nil {
		fmt.Println("aof error:", err)
		return
	}
	defer journal.file.Close()

	n, err := journal.replay(db)
	if err != nil {
		fmt.Println("replay stopped early:", err)
	}
	fmt.Println("replayed", n, "commands")
	fmt.Println("Listening on :6380")

	for {
		// Wait on client to connect
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("accept error:", err)
			continue
		}
		// Handle each client in its own goroutine
		go handle(conn, db, journal)
	}
}

func handle(conn net.Conn, db *Store, journal *aof) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	for {
		args, err := readCommand(reader)
		if err != nil {
			return
		}
		reply := execute(db, args)
		if isWrite(args) && !strings.HasPrefix(reply, "-") {
			if err := journal.write(args); err != nil {
				fmt.Println("aof write error:", err)
			}
		}
		conn.Write([]byte(reply))
	}
}
