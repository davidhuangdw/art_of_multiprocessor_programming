package main

// Command mutex runs the Chapter 2 bakery lock with several goroutines and
// verifies that the protected counter ends up with the expected value.
//
//	go run ./examples/mutex

import (
	"fmt"
	"os"
	"sync"

	mutex "github.com/davidhuangdw/art_of_multiprocessor_programming/ch02_mutex"
)

func main() {
	const (
		n   = 4     // threads, each with a fixed index in [0, n)
		per = 10000 // increments per goroutine
	)

	lock := mutex.NewBakery(n)

	var (
		counter int // guarded only by the bakery lock
		wg      sync.WaitGroup
	)
	wg.Add(n)
	for g := 0; g < n; g++ {
		go func(g int) {
			defer wg.Done()
			for i := 0; i < per; i++ {
				lock.Lock(g)
				counter++
				lock.Unlock(g)
			}
		}(g)
	}
	wg.Wait()

	fmt.Printf("threads=%d increments-each=%d counter=%d expected=%d\n",
		n, per, counter, n*per)
	if counter != n*per {
		fmt.Println("FAIL: lost updates")
		os.Exit(1)
	}
	fmt.Println("OK: no lost updates")
}
