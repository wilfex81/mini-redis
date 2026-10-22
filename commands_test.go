package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestExecute(t *testing.T) {
	db := newStore()
	steps := []struct {
		args []string
		want string
	}{
		{[]string{"PING"}, "+PONG\r\n"},
		{[]string{"ping", "hi"}, "$2\r\nhi\r\n"},
		{[]string{"ECHO", "yo"}, "$2\r\nyo\r\n"},
		{[]string{"SET", "a", "1"}, "+OK\r\n"},
		{[]string{"GET", "a"}, "$1\r\n1\r\n"},
		{[]string{"INCR", "a"}, ":2\r\n"},
		{[]string{"DECR", "a"}, ":1\r\n"},
		{[]string{"GET", "missing"}, "$-1\r\n"},
		{[]string{"EXISTS", "a", "missing"}, ":1\r\n"},
		{[]string{"DEL", "a", "missing"}, ":1\r\n"},
		{[]string{"EXISTS", "a"}, ":0\r\n"},
		{[]string{"RPUSH", "l", "x", "y"}, ":2\r\n"},
		{[]string{"LPUSH", "l", "z"}, ":3\r\n"},
		{[]string{"LRANGE", "l", "0", "-1"}, "*3\r\n$1\r\nz\r\n$1\r\nx\r\n$1\r\ny\r\n"},
		{[]string{"LRANGE", "l", "1", "1"}, "*1\r\n$1\r\nx\r\n"},
		{[]string{"HSET", "h", "f", "v"}, ":1\r\n"},
		{[]string{"HSET", "h", "f", "w"}, ":0\r\n"},
		{[]string{"HGET", "h", "f"}, "$1\r\nw\r\n"},
		{[]string{"HGET", "h", "nope"}, "$-1\r\n"},
		{[]string{"TTL", "h"}, ":-1\r\n"},
		{[]string{"TTL", "missing"}, ":-2\r\n"},
		{[]string{"EXPIRE", "h", "100"}, ":1\r\n"},
		{[]string{"EXPIRE", "missing", "100"}, ":0\r\n"},
	}
	for _, s := range steps {
		assert.Equal(t, s.want, execute(db, s.args), "command %v", s.args)
	}
}

func TestExecuteErrors(t *testing.T) {
	db := newStore()
	execute(db, []string{"SET", "s", "text"})

	assert.Contains(t, execute(db, []string{"GET"}), "wrong number of arguments")
	assert.Contains(t, execute(db, []string{"SET", "a"}), "wrong number of arguments")
	assert.Contains(t, execute(db, []string{"FOO"}), "unknown command")
	assert.Contains(t, execute(db, []string{"INCR", "s"}), "not an integer")
	assert.Contains(t, execute(db, []string{"SET", "k", "v", "EX", "abc"}), "invalid expire time")
	assert.Contains(t, execute(db, []string{}), "empty command")
}

func TestExecuteWrongType(t *testing.T) {
	db := newStore()
	execute(db, []string{"SET", "s", "v"})
	execute(db, []string{"HSET", "h", "f", "v"})
	execute(db, []string{"RPUSH", "l", "x"})

	bad := [][]string{
		{"HSET", "s", "f", "v"}, {"HGET", "s", "f"}, {"HGETALL", "s"},
		{"LPUSH", "s", "x"}, {"RPUSH", "s", "x"}, {"LRANGE", "s", "0", "-1"},
		{"GET", "h"}, {"INCR", "h"}, {"LPUSH", "h", "x"},
		{"GET", "l"}, {"HSET", "l", "f", "v"},
	}
	for _, cmd := range bad {
		assert.Contains(t, execute(db, cmd), "WRONGTYPE", "command %v", cmd)
	}

	// SET overwrites a key of any type
	assert.Equal(t, "+OK\r\n", execute(db, []string{"SET", "h", "now a string"}))
	assert.Equal(t, "$12\r\nnow a string\r\n", execute(db, []string{"GET", "h"}))
}

func TestExpiry(t *testing.T) {
	db := newStore()
	db.set("k", "v", 50*time.Millisecond)

	_, ok, _ := db.get("k")
	assert.True(t, ok)

	time.Sleep(100 * time.Millisecond)

	_, ok, _ = db.get("k")
	assert.False(t, ok)
	assert.Equal(t, -2, db.ttl("k"))
	assert.Equal(t, 0, db.exists("k"))
}

func TestExpireWorksOnEveryType(t *testing.T) {
	db := newStore()
	execute(db, []string{"HSET", "h", "f", "v"})
	execute(db, []string{"RPUSH", "l", "x"})
	assert.Equal(t, true, db.expireAt("h", time.Now().Add(-time.Second)))
	assert.Equal(t, true, db.expireAt("l", time.Now().Add(-time.Second)))
	assert.Equal(t, 0, db.exists("h", "l"))
}

// the compaction snapshot must rebuild an identical store
func TestSnapshotRoundTrip(t *testing.T) {
	src := newStore()
	execute(src, []string{"SET", "s", "v"})
	execute(src, []string{"INCR", "n"})
	execute(src, []string{"HSET", "h", "a", "1", "b", "2"})
	execute(src, []string{"RPUSH", "l", "x", "y", "z"})
	execute(src, []string{"SET", "t", "temp", "EX", "100"})

	dst := newStore()
	for _, cmd := range src.snapshot() {
		execute(dst, cmd)
	}

	for _, k := range []string{"s", "n", "t"} {
		want, _, _ := src.get(k)
		got, _, _ := dst.get(k)
		assert.Equal(t, want, got, "key %s", k)
	}
	wantHash, _ := src.hgetall("h")
	gotHash, _ := dst.hgetall("h")
	assert.ElementsMatch(t, wantHash, gotHash)

	wantList, _ := src.lrange("l", 0, -1)
	gotList, _ := dst.lrange("l", 0, -1)
	assert.Equal(t, wantList, gotList)

	assert.Equal(t, -1, dst.ttl("s"))
	assert.Greater(t, dst.ttl("t"), 90)
}
