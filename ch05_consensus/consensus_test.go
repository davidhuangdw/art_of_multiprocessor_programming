package consensus

import (
	"sync"
	"testing"
	"time"

	"github.com/davidhuangdw/art_of_multiprocessor_programming/prim"
)

type consensusFactory struct {
	name string
	new  func() prim.Consensus[int]
}

func consensusFactories() []consensusFactory {
	return []consensusFactory{
		{name: "CASConsensus", new: func() prim.Consensus[int] { return NewCASConsensus[int]() }},
	}
}

const (
	consensusThreads = 8
	consensusRounds  = 100
	consensusTimeout = 5 * time.Second
)

// TestConsensusSingleCaller checks that a lone caller decides its own value.
func TestConsensusSingleCaller(t *testing.T) {
	for _, tc := range consensusFactories() {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.new()
			if got, want := c.Decide(42), 42; got != want {
				t.Fatalf("Decide(42) = %d, want %d", got, want)
			}
		})
	}
}

// TestConsensusAgreement checks agreement, validity, and that every Decide
// call completes (the protocol is wait-free).
func TestConsensusAgreement(t *testing.T) {
	for _, tc := range consensusFactories() {
		t.Run(tc.name, func(t *testing.T) {
			for round := 0; round < consensusRounds; round++ {
				c := tc.new()
				results := make([]int, consensusThreads)
				start := make(chan struct{})

				var wg sync.WaitGroup
				wg.Add(consensusThreads)
				for i := 0; i < consensusThreads; i++ {
					go func(i int) {
						defer wg.Done()
						<-start
						results[i] = c.Decide(i)
					}(i)
				}
				close(start)
				waitForDecide(t, &wg, round)

				want := results[0]
				for i, got := range results {
					if got != want {
						t.Fatalf("round %d: goroutine %d decided %d, want %d", round, i, got, want)
					}
					if got < 0 || got >= consensusThreads {
						t.Fatalf("round %d: decided value %d was not proposed", round, got)
					}
				}
			}
		})
	}
}

func waitForDecide(t *testing.T, wg *sync.WaitGroup, round int) {
	t.Helper()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(consensusTimeout):
		t.Fatalf("round %d: Decide did not complete within %s", round, consensusTimeout)
	}
}
