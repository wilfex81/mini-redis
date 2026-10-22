package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCommandComplete(t *testing.T) {
	buf := []byte("*2\r\n$4\r\nECHO\r\n$2\r\nhi\r\n")
	args, used, err := parseCommand(buf)
	require.NoError(t, err)
	assert.Equal(t, []string{"ECHO", "hi"}, args)
	assert.Equal(t, len(buf), used)
}

func TestParseCommandTwoInOneBuffer(t *testing.T) {
	first := "*1\r\n$4\r\nPING\r\n"
	buf := []byte(first + "*1\r\n$4\r\nPING\r\n")
	args, used, err := parseCommand(buf)
	require.NoError(t, err)
	assert.Equal(t, []string{"PING"}, args)
	assert.Equal(t, len(first), used) // only the first command is consumed
}

func TestParseCommandIncomplete(t *testing.T) {
	cases := []string{
		"", "*", "*1", "*1\r\n", "*1\r\n$4", "*1\r\n$4\r\n",
		"*1\r\n$4\r\nPI", "*1\r\n$4\r\nPING", "*2\r\n$4\r\nECHO\r\n",
	}
	for _, c := range cases {
		args, used, err := parseCommand([]byte(c))
		assert.NoError(t, err, "input %q", c)
		assert.Equal(t, 0, used, "input %q", c)
		assert.Nil(t, args, "input %q", c)
	}
}

// every shorter prefix of a valid command must say "incomplete", never an error
func TestParseCommandByteByByte(t *testing.T) {
	full := []byte("*3\r\n$3\r\nSET\r\n$4\r\nname\r\n$6\r\nwilfex\r\n")
	for i := 0; i < len(full); i++ {
		_, used, err := parseCommand(full[:i])
		require.NoError(t, err, "prefix length %d", i)
		require.Equal(t, 0, used, "prefix length %d", i)
	}
	args, used, err := parseCommand(full)
	require.NoError(t, err)
	assert.Equal(t, []string{"SET", "name", "wilfex"}, args)
	assert.Equal(t, len(full), used)
}

func TestParseCommandErrors(t *testing.T) {
	cases := []string{
		"PING\r\n",             // not an array
		"*x\r\n",               // count is not a number
		"*-1\r\n",              // negative count
		"*99999999\r\n",        // absurd count
		"*1\r\nPING\r\n",       // missing $ header
		"*1\r\n$x\r\n",         // length is not a number
		"*1\r\n$-1\r\n\r\n",    // negative length
		"*1\r\n$999999999\r\n", // absurd length
	}
	for _, c := range cases {
		_, used, err := parseCommand([]byte(c))
		assert.Error(t, err, "input %q", c)
		assert.Equal(t, 0, used, "input %q", c)
	}
}

func TestParseCommandEmptyAndBinaryArgs(t *testing.T) {
	args, _, err := parseCommand([]byte("*2\r\n$3\r\nSET\r\n$0\r\n\r\n"))
	require.NoError(t, err)
	assert.Equal(t, []string{"SET", ""}, args)

	// a \r\n inside an argument is fine because we read by length
	args, _, err = parseCommand([]byte("*2\r\n$4\r\nECHO\r\n$4\r\na\r\nb\r\n"))
	require.NoError(t, err)
	assert.Equal(t, []string{"ECHO", "a\r\nb"}, args)
}
