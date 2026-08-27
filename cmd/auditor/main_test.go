package main

import (
	"strings"
	"testing"
	"time"
)

func TestShutdownTimeoutFromEnvUsesDefault(t *testing.T) {
	t.Setenv("SHUTDOWN_TIMEOUT", "")

	got, err := shutdownTimeoutFromEnv()
	if err != nil {
		t.Fatalf("shutdownTimeoutFromEnv: %v", err)
	}
	if got != 30*time.Second {
		t.Fatalf("timeout: got %v, want 30s", got)
	}
}

func TestShutdownTimeoutFromEnvParsesDuration(t *testing.T) {
	t.Setenv("SHUTDOWN_TIMEOUT", "45s")

	got, err := shutdownTimeoutFromEnv()
	if err != nil {
		t.Fatalf("shutdownTimeoutFromEnv: %v", err)
	}
	if got != 45*time.Second {
		t.Fatalf("timeout: got %v, want 45s", got)
	}
}

func TestShutdownTimeoutFromEnvRejectsInvalidDuration(t *testing.T) {
	t.Setenv("SHUTDOWN_TIMEOUT", "later")

	_, err := shutdownTimeoutFromEnv()
	if err == nil || !strings.Contains(err.Error(), "SHUTDOWN_TIMEOUT") {
		t.Fatalf("error: got %v, want clear SHUTDOWN_TIMEOUT error", err)
	}
}

func TestShutdownTimeoutFromEnvRejectsNonPositiveDuration(t *testing.T) {
	for _, value := range []string{"0s", "-1s"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("SHUTDOWN_TIMEOUT", value)

			_, err := shutdownTimeoutFromEnv()
			if err == nil || !strings.Contains(err.Error(), "must be positive") {
				t.Fatalf("error: got %v, want positive-duration error", err)
			}
		})
	}
}
