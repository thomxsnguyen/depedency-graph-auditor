package pypi

import "testing"

func TestVersionOrdering(t *testing.T) {
	ordered := []string{
		"1.0.dev",
		"1.0a1.dev1",
		"1.0a1",
		"1.0b1",
		"1.0rc1",
		"1.0",
		"1.0.post1",
		"1!0.1",
	}
	for index := 1; index < len(ordered); index++ {
		left, err := ParseVersion(ordered[index-1])
		if err != nil {
			t.Fatal(err)
		}
		right, err := ParseVersion(ordered[index])
		if err != nil {
			t.Fatal(err)
		}
		if left.Compare(right) >= 0 {
			t.Fatalf("expected %s < %s", ordered[index-1], ordered[index])
		}
	}
}

func TestResolveVersionPEP440Constraints(t *testing.T) {
	available := []string{"1.4.0", "1.4.5", "1.5.0rc1", "1.5.0", "2.0.0", "2.0.0+local.1", "2.1.0.post1"}
	tests := []struct {
		constraint string
		want       string
	}{
		{constraint: ">=1.4,<2", want: "1.5.0"},
		{constraint: "~=1.4.0", want: "1.4.5"},
		{constraint: "==1.4.*", want: "1.4.5"},
		{constraint: ">=1.4,!=1.5.0,<2", want: "1.4.5"},
		{constraint: "===2.0.0", want: "2.0.0"},
		{constraint: "==2.0.0", want: "2.0.0+local.1"},
		{constraint: ">=1.5.0rc1,<1.5.0rc2", want: "1.5.0rc1"},
	}
	for _, test := range tests {
		t.Run(test.constraint, func(t *testing.T) {
			got, err := ResolveVersion(test.constraint, available)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("resolved: got %q, want %q", got, test.want)
			}
		})
	}
}

func TestResolveVersionRejectsNoMatch(t *testing.T) {
	if _, err := ResolveVersion(">=3", []string{"1.0", "2.0"}); err == nil {
		t.Fatal("expected no-match error")
	}
}

func TestResolveVersionRejectsInvalidCompatibleConstraint(t *testing.T) {
	if _, err := ResolveVersion("~=1", []string{"1.0"}); err == nil {
		t.Fatal("expected invalid compatible-constraint error")
	}
}

func TestResolveVersionExclusiveBoundaryRules(t *testing.T) {
	if _, err := ResolveVersion("<1.0", []string{"1.0rc1"}); err == nil {
		t.Fatal("<1.0 must not select a prerelease of 1.0")
	}
	if _, err := ResolveVersion(">1.0", []string{"1.0.post1"}); err == nil {
		t.Fatal(">1.0 must not select a post-release of 1.0")
	}
	if got, err := ResolveVersion(">1.0.post0", []string{"1.0.post1"}); err != nil || got != "1.0.post1" {
		t.Fatalf("post-release boundary: got %q, error %v", got, err)
	}
}
