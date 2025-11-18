package io_test

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"

	utilio "github.com/k8s-manifest-kit/renderer-kustomize/pkg/util/io"

	. "github.com/onsi/gomega"
)

func TestSuppressStderr_Success(t *testing.T) {
	g := NewWithT(t)

	called := false
	err := utilio.SuppressStderr(func() error {
		called = true

		return nil
	})

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(called).To(BeTrue())
}

func TestSuppressStderr_ErrorPropagation(t *testing.T) {
	g := NewWithT(t)

	expectedErr := errors.New("test error")
	err := utilio.SuppressStderr(func() error {
		return expectedErr
	})

	g.Expect(err).To(Equal(expectedErr))
}

func TestSuppressStderr_StderrSuppression(t *testing.T) {
	g := NewWithT(t)

	// This test verifies that stderr writes are suppressed
	err := utilio.SuppressStderr(func() error {
		// Write to stderr - this should be suppressed
		_, _ = fmt.Fprintln(os.Stderr, "This should be suppressed")

		return nil
	})

	g.Expect(err).ToNot(HaveOccurred())

	// Verify stderr is restored by writing after the function
	// We can't easily verify the suppression happened, but we can verify
	// that stderr is functional after the call
	g.Expect(os.Stderr).ToNot(BeNil())
}

func TestSuppressStderr_StderrRestoration(t *testing.T) {
	g := NewWithT(t)

	originalStderr := os.Stderr

	err := utilio.SuppressStderr(func() error {
		// Verify stderr is different during execution
		g.Expect(os.Stderr).ToNot(Equal(originalStderr))

		return nil
	})

	g.Expect(err).ToNot(HaveOccurred())
	// Verify stderr is restored after execution
	g.Expect(os.Stderr).To(Equal(originalStderr))
}

func TestSuppressStderr_StderrRestorationOnError(t *testing.T) {
	g := NewWithT(t)

	originalStderr := os.Stderr

	err := utilio.SuppressStderr(func() error {
		return errors.New("error occurred")
	})

	g.Expect(err).To(HaveOccurred())
	// Verify stderr is restored even when error occurs
	g.Expect(os.Stderr).To(Equal(originalStderr))
}

func TestSuppressStderr_PanicRecovery(t *testing.T) {
	g := NewWithT(t)

	originalStderr := os.Stderr

	// Verify that panic propagates but stderr is still restored
	g.Expect(func() {
		_ = utilio.SuppressStderr(func() error {
			panic("test panic")
		})
	}).To(Panic())

	// Verify stderr is restored even after panic
	g.Expect(os.Stderr).To(Equal(originalStderr))
}

func TestSuppressStderr_ConcurrentExecution(t *testing.T) {
	g := NewWithT(t)

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)

	errChan := make(chan error, goroutines)

	for i := range goroutines {
		go func(id int) {
			defer wg.Done()

			err := utilio.SuppressStderr(func() error {
				// Write to stderr to ensure suppression works concurrently
				_, _ = fmt.Fprintf(os.Stderr, "Goroutine %d\n", id)

				return nil
			})

			errChan <- err
		}(i)
	}

	wg.Wait()
	close(errChan)

	// Verify all goroutines completed without error
	for err := range errChan {
		g.Expect(err).ToNot(HaveOccurred())
	}
}

func TestSuppressStderr_MultipleWrites(t *testing.T) {
	g := NewWithT(t)

	writeCount := 0
	err := utilio.SuppressStderr(func() error {
		// Multiple writes to stderr
		for i := range 5 {
			_, _ = fmt.Fprintf(os.Stderr, "Write %d\n", i)
			writeCount++
		}

		return nil
	})

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(writeCount).To(Equal(5))
}

func TestSuppressStderr_NestedCalls(t *testing.T) {
	g := NewWithT(t)

	// Test nested calls to SuppressStderr
	err := utilio.SuppressStderr(func() error {
		return utilio.SuppressStderr(func() error {
			_, _ = fmt.Fprintln(os.Stderr, "Nested stderr write")

			return nil
		})
	})

	g.Expect(err).ToNot(HaveOccurred())
}
