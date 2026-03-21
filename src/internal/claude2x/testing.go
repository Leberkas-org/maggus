package claude2x

import (
	"sync"
	"time"
)

// SetTestCache sets the cache directly for testing purposes.
// The once is marked as done so FetchStatus() will not make an HTTP call.
// remainingSeconds controls how much time appears to be left.
func SetTestCache(is2x bool, remainingSeconds int) {
	once = sync.Once{}
	cached = Status{
		Is2x:                       is2x,
		TwoXWindowExpiresInSeconds: remainingSeconds,
	}
	fetchedAt = time.Now()
	// Mark once as done so FetchStatus() uses the cache.
	once.Do(func() {})
}

// ResetTestCache resets the cache to its zero state for test cleanup.
func ResetTestCache() {
	resetCache()
}
