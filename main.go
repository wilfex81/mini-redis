package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/panjf2000/gnet/v2"
)

type server struct {
	gnet.BuiltinEventEngine // empty versions of every callback
	db                      *Store
	journal                 *aof
}

// runs once when the server starts
func (s *server) OnBoot(eng gnet.Engine) gnet.Action {
	fmt.Println("Listening on :6380")
	return gnet.None
}

// runs whenever a client sends bytes
func (s *server) OnTraffic(c gnet.Conn) gnet.Action {
	buf, _ := c.Peek(c.InboundBuffered()) // look at all bytes without removing them
	pos := 0

	for pos < len(buf) {
		args, used, err := parseCommand(buf[pos:])
		if err != nil {
			c.Write([]byte(errorReply("protocol error")))
			return gnet.Close
		}
		if used == 0 {
			break
		}
		pos += used

		reply := execute(s.db, args)
		if isWrite(args) && !strings.HasPrefix(reply, "-") {
			if err := s.journal.write(args); err != nil {
				fmt.Println("aof write error:", err)
			}
		}
		c.Write([]byte(reply))
	}
	c.Discard(pos) //remove only one bite we fully handled
	return gnet.None
}

func main() {
	db := newStore()

	journal, err := openAOF("appendonly.aof")
	if err != nil {
		log.Fatal("aof error: ", err)
	}
	defer journal.file.Close()

	n, err := journal.replay(db)
	if err != nil {
		fmt.Println("replay stopped early:", err)
	}
	fmt.Println("replayed", n, "commands")

	srv := &server{db: db, journal: journal}
	log.Fatal(gnet.Run(srv, "tcp://:6380", gnet.WithMulticore(true)))
}
