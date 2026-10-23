package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/panjf2000/gnet/v2"
	"github.com/spf13/cobra"
)

type server struct {
	gnet.BuiltinEventEngine
	addr    string
	db      *Store
	journal *aof
}

func (s *server) OnBoot(eng gnet.Engine) gnet.Action {
	fmt.Println("Listening on", s.addr)
	return gnet.None
}

func (s *server) OnTraffic(c gnet.Conn) gnet.Action {
	buf, _ := c.Peek(c.InboundBuffered())
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
			for _, cmd := range journalForm(args) {
				s.journal.write(cmd)
			}
		}
		c.Write([]byte(reply))
	}

	c.Discard(pos)
	return gnet.None
}

func main() {
	var port int
	var aofPath string

	cmd := &cobra.Command{
		Use:   "mini-redis",
		Short: "A small Redis server written in Go",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(port, aofPath)
		},
	}
	cmd.Flags().IntVarP(&port, "port", "p", 6380, "port to listen on")
	cmd.Flags().StringVar(&aofPath, "aof", "appendonly.aof", "path of the append-only file")

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(port int, aofPath string) error {
	db := newStore()

	n, err := loadAOF(aofPath, db)
	if err != nil {
		fmt.Println("replay stopped early:", err)
	} else if err := compactAOF(aofPath, db); err != nil {
		fmt.Println("compact error:", err)
	}
	fmt.Println("replayed", n, "commands")

	journal, err := openAOF(aofPath)
	if err != nil {
		return fmt.Errorf("aof error: %w", err)
	}

	addr := fmt.Sprintf("tcp://:%d", port)

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		gnet.Stop(ctx, addr)
	}()

	srv := &server{addr: addr, db: db, journal: journal}
	runErr := gnet.Run(srv, addr, gnet.WithMulticore(true))

	journal.close() // flush buffered writes whether we stopped cleanly or not
	if runErr != nil {
		return runErr
	}
	fmt.Println("shutdown complete")
	return nil
}
