package auditor

import (
	"fmt"
	"strings"
)

// PackageViolation pairs a policy-violating package with its path from the root.
type PackageViolation struct {
	Package Package
	Path    []string // e.g. ["my-app", "express@4.18.2", "evil-lib@0.1.0"]
}

// Report is the output of GenerateReport. It is populated after the pool
// signals completion and the graph is fully traversed.
type Report struct {
	TotalPackages    int
	PolicyViolations []PackageViolation
	DependencyPaths  map[string][]string // "name@version" → path from root
	Summary          string
}

// GenerateReport builds a Report from the completed package and edge stores.
// root is the name of the root project (e.g. "my-app") — it is the starting
// node for all backwards-path walks.
func GenerateReport(packages *PackageStore, edges *EdgeStore, root string) *Report {
	allPkgs := packages.All()
	allEdges := edges.All()

	// Build a reverse adjacency map: child "name@version" → list of parent "name@version".
	// This lets us walk backwards from a violation to the root.
	//
	// Edges originating from the root project are stored with the raw key
	// key(fromName, fromVersion). When the root has no version (common in the
	// seed step) this produces "root@". We normalise both forms in findPath.
	parents := make(map[string][]string)
	for _, e := range allEdges {
		child := key(e.ToName, e.ToVersion)
		parent := key(e.FromName, e.FromVersion)
		parents[child] = append(parents[child], parent)
	}

	// Collect violations and compute their paths.
	var violations []PackageViolation
	depPaths := make(map[string][]string)

	for _, pkg := range allPkgs {
		if pkg.Verdict != VerdictPolicyViolation {
			continue
		}
		k := key(pkg.Name, pkg.Version)
		path := findPath(k, root, parents)
		violations = append(violations, PackageViolation{
			Package: pkg,
			Path:    path,
		})
		depPaths[k] = path
	}

	clean := len(allPkgs) - len(violations)

	r := &Report{
		TotalPackages:    len(allPkgs),
		PolicyViolations: violations,
		DependencyPaths:  depPaths,
		Summary:          buildSummary(root, allPkgs, violations, clean),
	}
	return r
}

// findPath walks the reverse-edge map backwards from node to root using BFS,
// returning the path as a slice ordered root → ... → node.
// If no path is found, returns a slice containing only the node.
//
// Edges seeded from the root project use key(root, "") = "root@", so we
// match both "root" and "root@" as the root sentinel.
func findPath(node, root string, parents map[string][]string) []string {
	rootKey := root + "@" // normalised form when version is empty

	type state struct {
		node string
		path []string // ordered: [node, intermediate..., closest-to-root]
	}

	visited := map[string]bool{node: true}
	queue := []state{{node: node, path: []string{node}}}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		for _, p := range parents[cur.node] {
			if p == root || p == rootKey {
				// Prepend root to the reversed path to get root → ... → node.
				full := make([]string, 0, len(cur.path)+1)
				full = append(full, root)
				for i := len(cur.path) - 1; i >= 0; i-- {
					full = append(full, cur.path[i])
				}
				return full
			}
			if visited[p] {
				continue
			}
			visited[p] = true
			newPath := make([]string, len(cur.path)+1)
			copy(newPath, cur.path)
			newPath[len(cur.path)] = p
			queue = append(queue, state{node: p, path: newPath})
		}
	}

	// Root not found — return just the node (shouldn't happen in a well-formed graph).
	return []string{node}
}

// reverse returns a new slice with elements in reverse order.
func reverse(s []string) []string {
	out := make([]string, len(s))
	for i, v := range s {
		out[len(s)-1-i] = v
	}
	return out
}

// buildSummary produces the human-readable report string matching the spec format.
func buildSummary(root string, allPkgs []Package, violations []PackageViolation, clean int) string {
	var b strings.Builder

	b.WriteString("=== Dependency Audit Report ===\n\n")
	fmt.Fprintf(&b, "Root: %s\n\n", root)
	fmt.Fprintf(&b, "Packages scanned: %d\n", len(allPkgs))
	fmt.Fprintf(&b, "Policy violations: %d\n", len(violations))

	for _, v := range violations {
		reason := licenseViolationReason(v.Package.License)
		fmt.Fprintf(&b, "  ✗ %s@%s — %s (%s)\n",
			v.Package.Name, v.Package.Version, v.Package.License, reason)
		fmt.Fprintf(&b, "    Path: %s\n", strings.Join(v.Path, " → "))
	}

	fmt.Fprintf(&b, "\nClean: %d packages passed all checks.\n", clean)
	return b.String()
}

// licenseViolationReason returns a short human-readable explanation for a
// license that failed the policy check.
func licenseViolationReason(license string) string {
	if license == "" {
		return "no license declared"
	}
	return "license not in allowlist"
}
