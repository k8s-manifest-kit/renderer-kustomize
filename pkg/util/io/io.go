package io

import (
	"fmt"
	"io"
	"os"
)

// SuppressStderr temporarily redirects stderr to /dev/null,
// executes the provided function, then restores stderr.
// Returns the function's error.
func SuppressStderr(fn func() error) error {
	// Save original stderr
	oldStderr := os.Stderr

	// Create a pipe to capture (and discard) stderr output
	r, w, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("failed to create pipe: %w", err)
	}

	// Redirect stderr to the write end of the pipe
	os.Stderr = w

	// Ensure stderr is restored even if fn panics
	defer func() {
		os.Stderr = oldStderr
		_ = w.Close()
		_ = r.Close()
	}()

	// Discard any output written to the pipe
	go func() {
		_, _ = io.Copy(io.Discard, r)
	}()

	// Execute the function
	return fn()
}
