package main

import (
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

	buf := make([]byte, 1024)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return
		}

		conn.Write(buf[:n])
	}
}
