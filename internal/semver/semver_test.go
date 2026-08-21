package semver_test

import (
	"testing"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/semver"
)

var available = []string{
	"1.0.0", "1.2.0", "1.2.3", "1.3.0",
	"2.0.0", "2.1.0",
	"4.17.0", "4.17.21", "4.18.0", "4.18.2",
	"5.0.0",
}

// TestParseRangeValid verifies that well-formed range strings parse without error.
func TestParseRangeValid(t *testing.T) {
	cases := []string{
		"^4.18.0",
		"~1.2.3",
		"4.18.2",
		">=1.0.0 <2.0.0",
		">=2.0.0",
		"<5.0.0",
	}
	for _, r := range cases {
		_, err := semver.ParseRange(r)
		if err != nil {
			t.Errorf("ParseRange(%q) unexpected error: %v", r, err)
		}
	}
}

// TestParseRangeInvalid verifies that garbage input returns an error.
func TestParseRangeInvalid(t *testing.T) {
	_, err := semver.ParseRange("not-a-semver-range!!!")
	if err == nil {
		t.Error("ParseRange(invalid): expected error, got nil")
	}
}

// TestResolveCaretPicksHighestCompatibleMinor verifies caret (^) behaviour:
// ^4.18.0 must match >=4.18.0 and <5.0.0, returning the highest available.
func TestResolveCaretPicksHighest(t *testing.T) {
	c, _ := semver.ParseRange("^4.18.0")
	got, err := semver.Resolve(c, available)
	if err != nil {
		t.Fatalf("Resolve(^4.18.0) error: %v", err)
	}
	if got != "4.18.2" {
		t.Errorf("Resolve(^4.18.0): got %q, want %q", got, "4.18.2")
	}
}

// TestResolveTildePicksHighestCompatiblePatch verifies tilde (~) behaviour:
// ~1.2.3 must match >=1.2.3 and <1.3.0.
func TestResolveTildePicksHighest(t *testing.T) {
	c, _ := semver.ParseRange("~1.2.3")
	got, err := semver.Resolve(c, available)
	if err != nil {
		t.Fatalf("Resolve(~1.2.3) error: %v", err)
	}
	if got != "1.2.3" {
		t.Errorf("Resolve(~1.2.3): got %q, want %q", got, "1.2.3")
	}
}

// TestResolveExactVersion verifies that an exact version string resolves to itself.
func TestResolveExactVersion(t *testing.T) {
	c, _ := semver.ParseRange("4.17.21")
	got, err := semver.Resolve(c, available)
	if err != nil {
		t.Fatalf("Resolve(4.17.21) error: %v", err)
	}
	if got != "4.17.21" {
		t.Errorf("Resolve(4.17.21): got %q, want %q", got, "4.17.21")
	}
}

// TestResolveExplicitRange verifies >=1.0.0 <2.0.0 picks the highest within the band.
func TestResolveExplicitRange(t *testing.T) {
	c, _ := semver.ParseRange(">=1.0.0 <2.0.0")
	got, err := semver.Resolve(c, available)
	if err != nil {
		t.Fatalf("Resolve(>=1.0.0 <2.0.0) error: %v", err)
	}
	if got != "1.3.0" {
		t.Errorf("Resolve(>=1.0.0 <2.0.0): got %q, want %q", got, "1.3.0")
	}
}

// TestResolveNoMatchErrors verifies that an error is returned when no version satisfies.
func TestResolveNoMatchErrors(t *testing.T) {
	c, _ := semver.ParseRange("^9.0.0") // nothing in available is >=9.0.0 <10.0.0
	_, err := semver.Resolve(c, available)
	if err == nil {
		t.Error("Resolve(^9.0.0): expected error for no match, got nil")
	}
}

// TestResolveMalformedVersionsSkipped verifies that unparseable entries in the
// available list are silently ignored and do not cause an error.
func TestResolveMalformedVersionsSkipped(t *testing.T) {
	mixed := []string{"bad-version", "also-bad", "2.1.0", "not-semver"}
	c, _ := semver.ParseRange("^2.0.0")
	got, err := semver.Resolve(c, mixed)
	if err != nil {
		t.Fatalf("Resolve with malformed entries: unexpected error: %v", err)
	}
	if got != "2.1.0" {
		t.Errorf("Resolve with malformed entries: got %q, want %q", got, "2.1.0")
	}
}
