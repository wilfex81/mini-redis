# mini-redis

A small Redis server written in Go. It speaks the real Redis protocol (RESP), so `redis-cli` and `redis-benchmark` work against it unchanged.

I built it in two parts to learn Go and server design:

- **Part 1:** standard library only. One goroutine per connection. Kept at the git tag `stdlib-version`.
- **Part 2 (current code):** the network layer replaced with [gnet](https://github.com/panjf2000/gnet), an event loop framework. Store, commands and persistence are unchanged.

## Quick start

```bash
go build -o mini-redis .
./mini-redis
```

```bash
redis-cli -p 6380 SET name wilfex
redis-cli -p 6380 GET name
```

Flags:

| Flag | Default | Meaning |
|------|---------|---------|
| `--port`, `-p` | `6380` | Port to listen on |
| `--aof` | `appendonly.aof` | Path of the append-only file |

Port 6380 avoids a clash with a real Redis on 6379.

## Docker

Run the published image:

```bash
docker run -d --name mini-redis -p 6380:6380 -v mini-redis-data:/data ghcr.io/wilfex81/mini-redis:latest
redis-cli -p 6380 PING
```

Or build it yourself with Compose:

```bash
docker compose up -d
```

The image is about 13 MB: a multi-stage build with a static Go binary on Alpine, running as a non-root user. The append-only file lives in the `/data` volume, so data survives removing and recreating the container. `docker stop` sends SIGTERM, and the server flushes the log before it exits.

A GitHub Actions workflow runs `go vet` and `go test -race` on every push to `main`, then publishes the image to GitHub Container Registry. A version tag such as `v0.1.0` publishes a matching image tag.

## Commands

| Group | Commands |
|-------|----------|
| Connection | `PING`, `ECHO` |
| Strings | `SET` (with `EX`), `GET`, `INCR`, `DECR` |
| Keys | `DEL`, `EXISTS`, `KEYS` (glob patterns), `EXPIRE`, `PEXPIREAT`, `TTL` |
| Hashes | `HSET`, `HGET`, `HGETALL` |
| Lists | `LPUSH`, `RPUSH`, `LRANGE` |

Each key holds one type. Using a command on the wrong type returns `WRONGTYPE`, as in Redis. `SET` overwrites a key of any type. Expiry works on every type.

## How it works

```
main.go       cobra flags, startup, graceful shutdown, gnet event handler
resp.go       RESP parser (partial-input aware) and the blocking reader used for the AOF
commands.go   command dispatch and reply encoding
store.go      typed in-memory keyspace behind one RWMutex
aof.go        append-only file: background writer, replay, compaction

Dockerfile, docker-compose.yml   container image and local run
.github/workflows/docker.yml     CI: vet, race tests, publish image
```

**Event loop.** gnet calls `OnTraffic` with whatever bytes arrived. `parseCommand` returns "incomplete" for a partial command and the bytes stay buffered until more arrive. Several commands in one read are handled in order. The parser is tested with every possible split point of a valid command.

**Store.** One `map[string]*entry`. Each entry holds its type, its value and its own expiry time. A single `sync.RWMutex` guards the map, and `go test -race` passes. Expired keys are hidden on read and removed by a once-per-second cleanup goroutine.

**Persistence.** Every successful write command is encoded as RESP and sent to a background goroutine through a channel. That goroutine buffers writes and calls `fsync` once per second (the same tradeoff as Redis `appendfsync everysec`). Ctrl+C or SIGTERM stops the server and flushes the buffer first.

On startup the log is replayed, then rewritten as the shortest set of commands that rebuilds the current state, and swapped in with an atomic rename. Expiry is logged as an absolute timestamp (`PEXPIREAT`), so a restart does not restart TTL clocks, and keys that expire while the server is down stay gone.

## Tests

```bash
go test -race -cover
```

Covers the parser (complete, partial, byte by byte, malformed, oversized), every command, wrong-type errors, expiry on every type, a snapshot round trip, and concurrent access under the race detector. The same checks run in CI on every push.

## Benchmarks

`redis-benchmark`, 100,000 requests, 50 clients, client and server on the same machine:

```bash
redis-benchmark -p 6380 -t set,get -n 100000 -c 50 -q          # one command per round trip
redis-benchmark -p 6380 -t set,get -n 100000 -c 50 -P 16 -q    # 16 pipelined commands
```

| Mode | SET | GET |
|------|-----|-----|
| No pipelining | 24,295 req/s | 21,477 req/s |
| Pipelined (`-P 16`) | 123,767 req/s | 134,203 req/s |

Without pipelining the limit is network round trips, not the server. SET stays close to GET with pipelining because the disk sync runs off the event loop. These numbers are from running the server natively. Inside Docker, expect lower numbers because of container networking.

## Known limits

- A crash (`kill -9`) can lose up to one second of writes. A clean shutdown loses none.
- The log only shrinks at startup. Live rewriting while the server runs is not implemented.
- One global lock. Fine at this scale, a sharded store would scale better.
- `KEYS` scans every key. `LPUSH` copies the list.
- No `MULTI`/`EXEC`, Pub/Sub, `AUTH`, or `CONFIG`.
- The published image is built for linux/amd64 only. On Apple Silicon it runs under emulation.
- Absolute expiry timestamps assume the system clock does not jump while the server is down.