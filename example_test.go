package debounce_test

import (
	"fmt"
	"time"

	"github.com/zmwangx/debounce"
)

func ExampleDebounce() {
	start := time.Now()

	var count int
	sync, _ := debounce.Debounce(func() {
		fmt.Printf("syncing after %.1fs\n", time.Since(start).Seconds())
		count++ // This is thread-safe.
		// Sync data...
	}, 200*time.Millisecond, debounce.WithMaxWait(500*time.Millisecond))

	for time.Since(start) < 1200*time.Millisecond {
		// Do work that generates data here...
		sync()
	}

	time.Sleep(300 * time.Millisecond)
	fmt.Printf("synced %d times\n", count)
	// Output: syncing after 0.5s
	// syncing after 1.0s
	// syncing after 1.4s
	// synced 3 times
}

func ExampleThrottle() {
	start := time.Now()

	var count int
	sync, _ := debounce.Throttle(func() {
		fmt.Printf("syncing after %.1fs\n", time.Since(start).Seconds())
		count++ // This is thread-safe.
		// Sync data...
	}, 500*time.Millisecond)

	for time.Since(start) < 1200*time.Millisecond {
		// Do work that generates data here...
		sync()
	}

	time.Sleep(500 * time.Millisecond)
	fmt.Printf("synced %d times\n", count)
	// Output: syncing after 0.0s
	// syncing after 0.5s
	// syncing after 1.0s
	// syncing after 1.5s
	// synced 4 times
}
