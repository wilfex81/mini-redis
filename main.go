package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

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
			s.journal.write(args)
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

	n, err := journal.replay(db)
	if err != nil {
		fmt.Println("replay stopped early:", err)
	}
	fmt.Println("replayed", n, "commands")

	addr := "tcp://:6380"

	// on CTRL+c, stop the server cleanly
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		gnet.Stop(ctx, addr)
	}()

	srv := &server{db: db, journal: journal}
	if err := gnet.Run(srv, addr, gnet.WithMulticore(true)); err != nil {
		fmt.Println("server error:", err)
	}

	journal.close() // flush the last buffered writes before exiting
	fmt.Println("shutdown complete")
}
