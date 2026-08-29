package pypi

import "testing"

func testTarget(t *testing.T, version, platform string) Target {
	t.Helper()
	target, err := NewTarget(version, platform)
	if err != nil {
		t.Fatal(err)
	}
	return target
}

func TestParseRequirementNormalizesAndEvaluatesMarkers(t *testing.T) {
	target := testTarget(t, "3.12", "linux")
	requirement, err := ParseRequirement(`Requests_Lib (>=2.31,<3) ; python_version >= "3.10" and sys_platform == "linux"`, target)
	if err != nil {
		t.Fatal(err)
	}
	if requirement.Name != "requests-lib" || requirement.Constraint != ">=2.31,<3" || !requirement.Active {
		t.Fatalf("requirement: %+v", requirement)
	}

	inactive, err := ParseRequirement(`colorama==0.4.6 ; sys_platform == "win32"`, target)
	if err != nil {
		t.Fatal(err)
	}
	if inactive.Active {
		t.Fatal("Windows-only requirement active for Linux target")
	}
}

func TestEvaluateMarkerPrecedenceAndParentheses(t *testing.T) {
	target := testTarget(t, "3.12", "darwin")
	active, err := EvaluateMarker(`python_version >= "3.11" and (sys_platform == "darwin" or sys_platform == "linux")`, target)
	if err != nil {
		t.Fatal(err)
	}
	if !active {
		t.Fatal("expected marker to be active")
	}
}

func TestParseRequirementRejectsOutOfScopeSources(t *testing.T) {
	target := testTarget(t, "3.12", "linux")
	for _, input := range []string{
		`requests[socks]>=2`,
		`requests @ https://example.com/requests.whl`,
		`git+https://github.com/example/project.git`,
		`../local-package`,
	} {
		if _, err := ParseRequirement(input, target); err == nil {
			t.Errorf("out-of-scope requirement %q was accepted", input)
		}
	}
}

func TestNewTargetRejectsUnsupportedValues(t *testing.T) {
	if _, err := NewTarget("3", "linux"); err == nil {
		t.Fatal("expected short version error")
	}
	if _, err := NewTarget("3.12", "solaris"); err == nil {
		t.Fatal("expected platform error")
	}
}
