package claude2x

// SetTestCache sets an explicit status override for testing purposes.
// If isNerfed is true and remainingSeconds > 0, returns an active nerfed status.
// Otherwise returns a zero Status (simulates normal/off hours).
func SetTestCache(isNerfed bool, remainingSeconds int) {
	mu.Lock()
	defer mu.Unlock()
	if isNerfed && remainingSeconds > 0 {
		testOverride = &Status{
			IsNerfed:                   true,
			TwoXWindowExpiresInSeconds: remainingSeconds,
			TwoXWindowExpiresIn:        formatRemaining(remainingSeconds),
		}
	} else {
		testOverride = &Status{}
	}
}

// ResetTestCache clears the test override so FetchStatus uses the real time-based logic.
func ResetTestCache() {
	mu.Lock()
	defer mu.Unlock()
	testOverride = nil
}
