package main

import (
	"sync"
	"time"
)

type Store struct {
	mu      sync.RWMutex
	data    map[string]string
	expires map[string]time.Time
}

func newStore() *Store {
	s := &Store{
		data:    make(map[string]string),
		expires: make(map[string]time.Time),
	}
	go s.cleanup() // runs forever in the background
	return s
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
		if _, ok := s.data[k]; ok && !s.expired(k) {
			count++
		}
		delete(s.data, k)
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
