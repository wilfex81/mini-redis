package main

import (
	"errors"
	"path"
	"strconv"
	"sync"
	"time"
)

var errNotInt = errors.New("value is not an integer or out of range")

type Store struct {
	mu      sync.RWMutex
	data    map[string]string
	hashes  map[string]map[string]string
	lists   map[string][]string
	expires map[string]time.Time
}

func newStore() *Store {
	s := &Store{
		data:    make(map[string]string),
		hashes:  make(map[string]map[string]string),
		lists:   make(map[string][]string),
		expires: make(map[string]time.Time),
	}
	go s.cleanup()
	return s
}

// adds delta to a key. A missing key counts as 0
func (s *Store) incr(key string, delta int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.expired(key) {
		delete(s.data, key)
		delete(s.expires, key)
	}

	n := 0
	if v, ok := s.data[key]; ok {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return 0, errNotInt
		}
		n = parsed
	}
	n += delta
	s.data[key] = strconv.Itoa(n)
	return n, nil
}

// counts how many of the given keys are present
func (s *Store) exists(keys ...string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, k := range keys {
		_, inData := s.data[k]
		_, inHash := s.hashes[k]
		_, inList := s.lists[k]
		if (inData && !s.expired(k)) || inHash || inList {
			count++
		}
	}
	return count
}

// Keys returns all live keys matching a global pattern like "user:*"
func (s *Store) keys(pattern string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := []string{}
	for k := range s.data {
		if s.expired(k) {
			continue
		}
		if ok, _ := path.Match(pattern, k); ok {
			result = append(result, k)
		}
	}
	for k := range s.hashes {
		if ok, _ := path.Match(pattern, k); ok {
			result = append(result, k)
		}
	}
	for k := range s.lists {
		if ok, _ := path.Match(pattern, k); ok {
			result = append(result, k)
		}
	}
	return result
}

func (s *Store) hset(key string, pairs ...string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	h, ok := s.hashes[key]
	if !ok {
		h = make(map[string]string)
		s.hashes[key] = h
	}
	added := 0
	for i := 0; i < len(pairs); i += 2 {
		if _, exists := h[pairs[i]]; !exists {
			added++
		}
		h[pairs[i]] = pairs[i+1]
	}
	return added
}

func (s *Store) hget(key, field string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.hashes[key][field]
	return v, ok
}

func (s *Store) hgetall(key string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := []string{}
	for f, v := range s.hashes[key] {
		result = append(result, f, v)
	}
	return result
}

func (s *Store) lpush(key string, vals ...string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	l := s.lists[key]
	for _, v := range vals {
		l = append([]string{v}, l...)
	}
	s.lists[key] = l
	return len(l)
}

func (s *Store) rpush(key string, vals ...string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lists[key] = append(s.lists[key], vals...)
	return len(s.lists[key])
}

func (s *Store) lrange(key string, start, stop int) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	l := s.lists[key]
	n := len(l)
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
		return []string{}
	}
	out := make([]string, stop-start+1)
	copy(out, l[start:stop+1]) // copy so callers never touch our slice
	return out
}

// expired reports whether a key is past its expiry.
// The caller must already hold the lock.
func (s *Store) expired(key string) bool {
	t, ok := s.expires[key]
	return ok && time.Now().After(t)
}

// set stores a value. A ttl of 0 means no expiry.
func (s *Store) set(key, value string, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
	if ttl > 0 {
		s.expires[key] = time.Now().Add(ttl)
	} else {
		delete(s.expires, key) // a plain SET clears any old expiry
	}
}

func (s *Store) get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.expired(key) {
		return "", false
	}
	v, ok := s.data[key]
	return v, ok
}

func (s *Store) del(keys ...string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, k := range keys {
		found := false
		if _, ok := s.data[k]; ok && !s.expired(k) {
			found = true
		}
		if _, ok := s.hashes[k]; ok {
			found = true
		}
		if _, ok := s.lists[k]; ok {
			found = true
		}
		if found {
			count++
		}
		delete(s.data, k)
		delete(s.hashes, k)
		delete(s.lists, k)
		delete(s.expires, k)
	}
	return count
}

// expire sets a new expiry on an existing key
func (s *Store) expire(key string, seconds int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[key]; !ok || s.expired(key) {
		return false
	}
	s.expires[key] = time.Now().Add(time.Duration(seconds) * time.Second)
	return true
}

// ttl returns seconds left, -1 if no expiry, -2 if the key is missing
func (s *Store) ttl(key string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.data[key]; !ok || s.expired(key) {
		return -2
	}
	t, ok := s.expires[key]
	if !ok {
		return -1
	}
	return int(time.Until(t).Round(time.Second).Seconds())
}

// cleanup removes expired keys once per second
func (s *Store) cleanup() {
	ticker := time.NewTicker(time.Second)
	for range ticker.C {
		s.mu.Lock()
		for k := range s.expires {
			if s.expired(k) {
				delete(s.data, k)
				delete(s.expires, k)
			}
		}
		s.mu.Unlock()
	}
}
