package auditor_test

import (
	"testing"

	"github.com/thomxsnguyen/mini-distributed-job-api/internal/auditor"
)

func TestLicensePolicyAllowedLicenses(t *testing.T) {
	p := auditor.LicensePolicy{}
	allowed := []string{"MIT", "Apache-2.0", "ISC", "BSD-2-Clause", "BSD-3-Clause"}

	for _, lic := range allowed {
		pkg := auditor.PackageMetadata{Name: "pkg", Version: "1.0.0", License: lic}
		got := p.Check(pkg)
		if got != auditor.VerdictPass {
			t.Errorf("LicensePolicy.Check(%q): got %q, want %q", lic, got, auditor.VerdictPass)
		}
	}
}

func TestLicensePolicyDisallowedLicenses(t *testing.T) {
	p := auditor.LicensePolicy{}
	disallowed := []string{"GPL-2.0", "GPL-3.0", "AGPL-3.0", "WTFPL", "LGPL-2.1", "CC-BY-SA-4.0"}

	for _, lic := range disallowed {
		pkg := auditor.PackageMetadata{Name: "pkg", Version: "1.0.0", License: lic}
		got := p.Check(pkg)
		if got != auditor.VerdictPolicyViolation {
			t.Errorf("LicensePolicy.Check(%q): got %q, want %q", lic, got, auditor.VerdictPolicyViolation)
		}
	}
}

func TestLicensePolicyEmptyLicenseIsViolation(t *testing.T) {
	p := auditor.LicensePolicy{}
	pkg := auditor.PackageMetadata{Name: "no-license-pkg", Version: "1.0.0", License: ""}
	got := p.Check(pkg)
	if got != auditor.VerdictPolicyViolation {
		t.Errorf("LicensePolicy.Check(empty license): got %q, want %q", got, auditor.VerdictPolicyViolation)
	}
}
