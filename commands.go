package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func simpleString(s string) string {
	return "+" + s + "\r\n"
}
func errorReply(s string) string {
	return "-ERR " + s + "\r\n"
}
func bulkString(s string) string {
	return fmt.Sprintf("$%d\r\n%s\r\n", len(s), s)
}

func intReply(n int) string {
	return fmt.Sprintf(":%d\r\n", n)
}

func arrayReply(items []string) string {
	out := fmt.Sprintf("*%d\r\n", len(items))
	for _, it := range items {
		out += bulkString(it)
	}
	return out
}

const nilReply = "$-1\r\n"

func execute(s *Store, args []string) string {
	if len(args) == 0 {
		return errorReply("empty command")
	}

	switch strings.ToUpper(args[0]) {
	case "PING":
		if len(args) > 1 {
			return bulkString(args[1])
		}
		return simpleString("PONG")

	case "ECHO":
		if len(args) != 2 {
			return errorReply("Wrong number of arguments for 'echo' command")
		}
		return bulkString(args[1])

	case "SET":
		if len(args) != 3 && len(args) != 5 {
			return errorReply("wrong number of arguments for 'set' command")
		}
		var ttl time.Duration
		if len(args) == 5 {
			if strings.ToUpper(args[3]) != "EX" {
				return errorReply("syntax error")
			}
			secs, err := strconv.Atoi(args[4])
			if err != nil || secs <= 0 {
				return errorReply("invalid expire time in 'set' command")
			}
			ttl = time.Duration(secs) * time.Second
		}
		s.set(args[1], args[2], ttl)
		return simpleString("OK")

	case "EXPIRE":
		if len(args) != 3 {
			return errorReply("wrong number of arguments for 'expire' command")
		}
		secs, err := strconv.Atoi(args[2])
		if err != nil {
			return errorReply("value is not an integer or out of range")
		}
		if s.expire(args[1], secs) {
			return intReply(1)
		}
		return intReply(0)

	case "TTL":
		if len(args) != 2 {
			return errorReply("wrong number of arguments for 'ttl' command")
		}
		return intReply(s.ttl(args[1]))

	case "GET":
		if len(args) != 2 {
			return errorReply("wrong number of arguments for 'get' command")
		}
		v, ok := s.get(args[1])
		if !ok {
			return nilReply
		}
		return bulkString(v)

	case "DEL":
		if len(args) < 2 {
			return errorReply("wrong number of arguments for 'del' command")
		}
		return intReply(s.del(args[1:]...))

	case "INCR":
		if len(args) != 2 {
			return errorReply("wrong number of arguments for 'incr' command")
		}
		n, err := s.incr(args[1], 1)
		if err != nil {
			return errorReply(err.Error())
		}
		return intReply(n)

	case "DECR":
		if len(args) != 2 {
			return errorReply("wrong number of arguments for 'decr' command")
		}
		n, err := s.incr(args[1], -1)
		if err != nil {
			return errorReply(err.Error())
		}
		return intReply(n)

	case "EXISTS":
		if len(args) < 2 {
			return errorReply("wrong number of arguments for 'exists' command")
		}
		return intReply(s.exists(args[1:]...))

	case "KEYS":
		if len(args) != 2 {
			return errorReply("wrong number of arguments for 'keys' command")
		}
		return arrayReply(s.keys(args[1]))

	case "HSET":
		if len(args) < 4 || len(args)%2 != 0 {
			return errorReply("wrong number of arguments for 'hset' command")
		}
		return intReply(s.hset(args[1], args[2:]...))
	case "HGET":
		if len(args) != 3 {
			return errorReply("wrong number of arguments for 'hget' command")
		}
		v, ok := s.hget(args[1], args[2])
		if !ok {
			return nilReply
		}
		return bulkString(v)

	case "HGETALL":
		if len(args) != 2 {
			return errorReply("wrong number of arguments for 'hgetall' command")
		}
		return arrayReply(s.hgetall(args[1]))

	case "LPUSH":
		if len(args) < 3 {
			return errorReply("wrong number of arguments for 'lpush' command")
		}
		return intReply(s.lpush(args[1], args[2:]...))

	case "RPUSH":
		if len(args) < 3 {
			return errorReply("wrong number of arguments for 'rpush' command")
		}
		return intReply(s.rpush(args[1], args[2:]...))

	case "LRANGE":
		if len(args) != 4 {
			return errorReply("wrong number of arguments for 'lrange' command")
		}
		start, err1 := strconv.Atoi(args[2])
		stop, err2 := strconv.Atoi(args[3])
		if err1 != nil || err2 != nil {
			return errorReply("value is not an integer or out of range")
		}
		return arrayReply(s.lrange(args[1], start, stop))

	default:
		return errorReply("Unkown command '" + args[0] + "'")
	}
}
