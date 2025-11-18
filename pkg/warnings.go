package kustomize

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

// WarningHandler is called when kustomize emits deprecation warnings.
// The handler receives a list of warning messages and should return an error
// to fail the render, or nil to continue.
type WarningHandler func(warnings []string) error

var (
	// ErrKustomizeWarnings is returned when kustomize warnings are detected and the handler fails.
	ErrKustomizeWarnings = errors.New("kustomize warnings detected")
)

// WarningIgnore returns a handler that suppresses all warnings.
// Use this when you want to silence kustomize deprecation warnings entirely.
//
// Example:
//
//	renderer := kustomize.New(
//	    []kustomize.Source{{Path: "/path/to/kustomization"}},
//	    kustomize.WithWarningHandler(kustomize.WarningIgnore()),
//	)
func WarningIgnore() WarningHandler {
	return func(_ []string) error {
		return nil
	}
}

// WarningLog returns a handler that writes warnings to the provided writer.
// Each warning message is written on a separate line.
//
// Example:
//
//	renderer := kustomize.New(
//	    []kustomize.Source{{Path: "/path/to/kustomization"}},
//	    kustomize.WithWarningHandler(kustomize.WarningLog(os.Stderr)),
//	)
func WarningLog(w io.Writer) WarningHandler {
	return func(warnings []string) error {
		for _, msg := range warnings {
			if _, err := fmt.Fprintf(w, "%s\n", msg); err != nil {
				return fmt.Errorf("failed to write warning: %w", err)
			}
		}

		return nil
	}
}

// WarningFail returns a handler that fails the render when warnings are present.
// All warnings are combined into a single error message.
//
// Example:
//
//	renderer := kustomize.New(
//	    []kustomize.Source{{Path: "/path/to/kustomization"}},
//	    kustomize.WithWarningHandler(kustomize.WarningFail()),
//	)
func WarningFail() WarningHandler {
	return func(warnings []string) error {
		if len(warnings) > 0 {
			return fmt.Errorf("%w:\n%s", ErrKustomizeWarnings, strings.Join(warnings, "\n"))
		}

		return nil
	}
}
