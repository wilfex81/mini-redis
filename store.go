package main

import (
	"errors"
	"path"
	"strconv"
	"sync"
	"time"
)

var (
	errNotInt    = errors.New("value is not an integer or out of range")
	errWrongType = errors.New("WRONGTYPE Operation against a key holding the wrong kind of value")
)

type valueKind int

const (
	kindString valueKind = iota
	kindHash
	kindList
)

// entry is one key's value, its type, and its optional expiry
type entry struct {
	kind    valueKind
	str     string
	hash    map[string]string
	list    []string
	expires time.Time // zero value means no expiry
}

func (e *entry) expired() bool {
	return !e.expires.IsZero() && time.Now().After(e.expires)
}

type Store struct {
	mu   sync.RWMutex
	data map[string]*entry
}

func newStore() *Store {
	s := &Store{data: make(map[string]*entry)}
	go s.cleanup()
	return s
}

// live returns the entry, or nil if the key is missing or expired.
// The caller must hold the lock.
func (s *Store) live(key string) *entry {
	e, ok := s.data[key]
	if !ok || e.expired() {
		return nil
	}
	return e
}

// liveW is live for writers: it also removes an expired entry.
// The caller must hold the write lock.
func (s *Store) liveW(key string) *entry {
	e := s.live(key)
	if e == nil {
		delete(s.data, key)
	}
	return e
}

// ---- strings ----

// set replaces whatever the key held. A ttl of 0 means no expiry.
func (s *Store) set(key, value string, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := &entry{kind: kindString, str: value}
	if ttl > 0 {
		e.expires = time.Now().Add(ttl)
	}
	s.data[key] = e
}

func (s *Store) get(key string) (string, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e := s.live(key)
	if e == nil {
		return "", false, nil
	}
	if e.kind != kindString {
		return "", false, errWrongType
	}
	return e.str, true, nil
}

func (s *Store) incr(key string, delta int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.liveW(key)
	if e == nil {
		s.data[key] = &entry{kind: kindString, str: strconv.Itoa(delta)}
		return delta, nil
	}
	if e.kind != kindString {
		return 0, errWrongType
	}
	n, err := strconv.Atoi(e.str)
	if err != nil {
		return 0, errNotInt
	}
	n += delta
	e.str = strconv.Itoa(n)
	return n, nil
}

// ---- any type ----

func (s *Store) del(keys ...string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, k := range keys {
		if s.live(k) != nil {
			count++
		}
		delete(s.data, k)
	}
	return count
}

func (s *Store) exists(keys ...string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, k := range keys {
		if s.live(k) != nil {
			count++
		}
	}
	return count
}

func (s *Store) keys(pattern string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := []string{}
	for k, e := range s.data {
		if e.expired() {
			continue
		}
		if ok, _ := path.Match(pattern, k); ok {
			result = append(result, k)
		}
	}
	return result
}

func (s *Store) expire(key string, seconds int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.liveW(key)
	if e == nil {
		return false
	}
	e.expires = time.Now().Add(time.Duration(seconds) * time.Second)
	return true
}

// ttl returns seconds left, -1 if no expiry, -2 if the key is missing
func (s *Store) ttl(key string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e := s.live(key)
	if e == nil {
		return -2
	}
	if e.expires.IsZero() {
		return -1
	}
	return int(time.Until(e.expires).Round(time.Second).Seconds())
}

// ---- hashes ----

func (s *Store) hset(key string, pairs ...string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.liveW(key)
	if e == nil {
		e = &entry{kind: kindHash, hash: make(map[string]string)}
		s.data[key] = e
	}
	if e.kind != kindHash {
		return 0, errWrongType
	}
	added := 0
	for i := 0; i < len(pairs); i += 2 {
		if _, exists := e.hash[pairs[i]]; !exists {
			added++
		}
		e.hash[pairs[i]] = pairs[i+1]
	}
	return added, nil
}

func (s *Store) hget(key, field string) (string, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e := s.live(key)
	if e == nil {
		return "", false, nil
	}
	if e.kind != kindHash {
		return "", false, errWrongType
	}
	v, ok := e.hash[field]
	return v, ok, nil
}

func (s *Store) hgetall(key string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := []string{}
	e := s.live(key)
	if e == nil {
		return result, nil
	}
	if e.kind != kindHash {
		return nil, errWrongType
	}
	for f, v := range e.hash {
		result = append(result, f, v)
	}
	return result, nil
}

// ---- lists ----

// pushTo finds or creates a list. The caller must hold the write lock.
func (s *Store) pushTo(key string) (*entry, error) {
	e := s.liveW(key)
	if e == nil {
		e = &entry{kind: kindList}
		s.data[key] = e
	}
	if e.kind != kindList {
		return nil, errWrongType
	}
	return e, nil
}

func (s *Store) lpush(key string, vals ...string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, err := s.pushTo(key)
	if err != nil {
		return 0, err
	}
	for _, v := range vals {
		e.list = append([]string{v}, e.list...)
	}
	return len(e.list), nil
}

func (s *Store) rpush(key string, vals ...string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, err := s.pushTo(key)
	if err != nil {
		return 0, err
	}
	e.list = append(e.list, vals...)
	return len(e.list), nil
}

func (s *Store) lrange(key string, start, stop int) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e := s.live(key)
	if e == nil {
		return []string{}, nil
	}
	if e.kind != kindList {
		return nil, errWrongType
	}
	n := len(e.list)
	if start < 0 {
		start += n
	}
	if stop < 0 {
		stop += n
	}
	if start < 0 {
		start = 0
	}
	if stop >= n {
		stop = n - 1
	}
	if start > stop {
		return []string{}, nil
	}
	out := make([]string, stop-start+1)
	copy(out, e.list[start:stop+1])
	return out, nil
}

// cleanup removes expired keys once per second
func (s *Store) cleanup() {
	ticker := time.NewTicker(time.Second)
	for range ticker.C {
		s.mu.Lock()
		for k, e := range s.data {
			if e.expired() {
				delete(s.data, k)
			}
		}
		s.mu.Unlock()
	}
}
