package main

type Store struct {
	data map[string]string
}

func newStore() *Store {
	return &Store{data: make(map[string]string)}
}

func (s *Store) set(key, value string) {
	s.data[key] = value
}

func (s *Store) get(key string) (string, bool) {
	v, ok := s.data[key]
	return v, ok
}

func (s *Store) del(keys ...string) int {
	count := 0
	for _, k := range keys {
		if _, ok := s.data[k]; ok {
			delete(s.data, k)
			count++
		}
	}
	return count
}
