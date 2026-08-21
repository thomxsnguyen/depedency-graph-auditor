package auditor

// Verdict is the outcome of evaluating a package against the audit policy.
type Verdict string

const (
	// VerdictPass means the package cleared all policy checks.
	VerdictPass Verdict = "pass"
	// VerdictPolicyViolation means one or more policy rules were breached.
	VerdictPolicyViolation Verdict = "policy_violation"
)

// PolicyChecker evaluates a package's metadata against the audit policy.
// It is a pure function — no I/O, no state. Implementations must be safe for
// concurrent calls from multiple worker goroutines.
type PolicyChecker interface {
	Check(pkg PackageMetadata) Verdict
}

// allowedLicenses is the set of SPDX license identifiers considered compliant.
var allowedLicenses = map[string]bool{
	"MIT":          true,
	"Apache-2.0":   true,
	"ISC":          true,
	"BSD-2-Clause": true,
	"BSD-3-Clause": true,
}

// LicensePolicy enforces the license allowlist defined in the phase 1 spec.
// Any package whose declared license is absent from the list is a policy violation.
type LicensePolicy struct{}

// Check returns VerdictPolicyViolation if the package's license is not in the allowlist.
func (p LicensePolicy) Check(pkg PackageMetadata) Verdict {
	if allowedLicenses[pkg.License] {
		return VerdictPass
	}
	return VerdictPolicyViolation
}
