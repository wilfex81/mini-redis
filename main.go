package main

import (
	"bufio"
	"fmt"
	"net"
)

func main() {
	// Listen for TCP connection on port 6380
	listener, err := net.Listen("tcp", ":6380")
	if err != nil {
		fmt.Println("listen error:", err)
		return
	}

	defer listener.Close()
	fmt.Println("Listening on :6380")

	for {
		// Wait on client to connect
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("accept error:", err)
			continue
		}
		// Handle each client in its own goroutine
		go handle(conn)
	}
}

func handle(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	for {
		args, err := readCommand(reader)
		if err != nil {
			return
		}
		fmt.Println("got command:", args)
		conn.Write([]byte(execute(args)))
	}
}
