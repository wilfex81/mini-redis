package main

import (
	"strconv"
	"sync"
	"testing"
)

func TestConcurrentSet(t *testing.T)  {
	db := newStore()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				db.set("key"+strconv.Itoa(n), "value")
				db.get("key" + strconv.Itoa(n))
			}
		}(i)
	}
	wg.Wait()

}