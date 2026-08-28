package auditor

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// MarkdownReportInput is a snapshot of the completed audit data needed to
// render a Markdown report. The renderer does not read stores or perform I/O.
type MarkdownReportInput struct {
	Root     string
	Packages []Package
	Edges    []DependencyEdge
	Report   *Report
}

type markdownGraphNode struct {
	name    string
	version string
}

// GenerateMarkdownReport renders a deterministic Markdown report containing
// the audit summary, complete stored dependency graph, package inventory, and
// policy violations.
func GenerateMarkdownReport(input MarkdownReportInput) (string, error) {
	if input.Report == nil {
		return "", errors.New("markdown report: report is required")
	}

	packages := append([]Package(nil), input.Packages...)
	sort.Slice(packages, func(i, j int) bool {
		if packages[i].Name != packages[j].Name {
			return packages[i].Name < packages[j].Name
		}
		return packages[i].Version < packages[j].Version
	})

	edges := append([]DependencyEdge(nil), input.Edges...)
	sort.Slice(edges, func(i, j int) bool { return dependencyEdgeLess(edges[i], edges[j]) })
	edges = deduplicateDependencyEdges(edges)

	violations := append([]PackageViolation(nil), input.Report.PolicyViolations...)
	sort.Slice(violations, func(i, j int) bool {
		if violations[i].Package.Name != violations[j].Package.Name {
			return violations[i].Package.Name < violations[j].Package.Name
		}
		if violations[i].Package.Version != violations[j].Package.Version {
			return violations[i].Package.Version < violations[j].Package.Version
		}
		return strings.Join(violations[i].Path, "\x00") < strings.Join(violations[j].Path, "\x00")
	})

	nodes, nodeIDs := markdownGraphNodes(packages, edges)

	var out strings.Builder
	out.WriteString("# Dependency Audit Report\n\n")
	out.WriteString("## Summary\n\n")
	fmt.Fprintf(&out, "- Root: %s\n", markdownInlineCode(input.Root))
	fmt.Fprintf(&out, "- Packages scanned: %d\n", input.Report.TotalPackages)
	fmt.Fprintf(&out, "- Policy violations: %d\n\n", len(violations))

	out.WriteString("## Dependency Graph\n\n")
	out.WriteString("```mermaid\n")
	out.WriteString("graph TD\n")
	for _, node := range nodes {
		fmt.Fprintf(&out, "    %s[\"%s\"]\n",
			nodeIDs[markdownGraphNodeKey(node.name, node.version)],
			escapeMermaidLabel(packageCoordinate(node.name, node.version)),
		)
	}
	for _, edge := range edges {
		fmt.Fprintf(&out, "    %s --> %s\n",
			nodeIDs[markdownGraphNodeKey(edge.FromName, edge.FromVersion)],
			nodeIDs[markdownGraphNodeKey(edge.ToName, edge.ToVersion)],
		)
	}
	out.WriteString("```\n\n")

	out.WriteString("## Packages\n\n")
	out.WriteString("| Package | Version | License | Verdict |\n")
	out.WriteString("|---|---|---|---|\n")
	for _, pkg := range packages {
		license := "Not declared"
		if pkg.License != "" {
			license = markdownInlineCode(pkg.License)
		}
		fmt.Fprintf(&out, "| %s | %s | %s | %s |\n",
			markdownInlineCode(pkg.Name),
			markdownInlineCode(pkg.Version),
			license,
			markdownInlineCode(string(pkg.Verdict)),
		)
	}
	out.WriteString("\n")

	out.WriteString("## Policy Violations\n\n")
	out.WriteString("| Package | License | Dependency path |\n")
	out.WriteString("|---|---|---|\n")
	for _, violation := range violations {
		license := "Not declared"
		if violation.Package.License != "" {
			license = markdownInlineCode(violation.Package.License)
		}
		path := "Not available"
		if len(violation.Path) > 0 {
			path = markdownInlineCode(strings.Join(violation.Path, " → "))
		}
		fmt.Fprintf(&out, "| %s | %s | %s |\n",
			markdownInlineCode(packageCoordinate(violation.Package.Name, violation.Package.Version)),
			license,
			path,
		)
	}

	return out.String(), nil
}

func markdownGraphNodes(packages []Package, edges []DependencyEdge) ([]markdownGraphNode, map[string]string) {
	byKey := make(map[string]markdownGraphNode, len(packages)+(2*len(edges)))
	for _, pkg := range packages {
		byKey[markdownGraphNodeKey(pkg.Name, pkg.Version)] = markdownGraphNode{
			name:    pkg.Name,
			version: pkg.Version,
		}
	}
	for _, edge := range edges {
		byKey[markdownGraphNodeKey(edge.FromName, edge.FromVersion)] = markdownGraphNode{
			name:    edge.FromName,
			version: edge.FromVersion,
		}
		byKey[markdownGraphNodeKey(edge.ToName, edge.ToVersion)] = markdownGraphNode{
			name:    edge.ToName,
			version: edge.ToVersion,
		}
	}

	nodes := make([]markdownGraphNode, 0, len(byKey))
	for _, node := range byKey {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].name != nodes[j].name {
			return nodes[i].name < nodes[j].name
		}
		return nodes[i].version < nodes[j].version
	})

	ids := make(map[string]string, len(nodes))
	for i, node := range nodes {
		ids[markdownGraphNodeKey(node.name, node.version)] = fmt.Sprintf("n%d", i)
	}
	return nodes, ids
}

func deduplicateDependencyEdges(edges []DependencyEdge) []DependencyEdge {
	if len(edges) < 2 {
		return edges
	}
	deduplicated := edges[:0]
	for _, edge := range edges {
		if len(deduplicated) == 0 || deduplicated[len(deduplicated)-1] != edge {
			deduplicated = append(deduplicated, edge)
		}
	}
	return deduplicated
}

func dependencyEdgeLess(a, b DependencyEdge) bool {
	if a.FromName != b.FromName {
		return a.FromName < b.FromName
	}
	if a.FromVersion != b.FromVersion {
		return a.FromVersion < b.FromVersion
	}
	if a.ToName != b.ToName {
		return a.ToName < b.ToName
	}
	return a.ToVersion < b.ToVersion
}

func markdownGraphNodeKey(name, version string) string {
	return name + "\x00" + version
}

func packageCoordinate(name, version string) string {
	if version == "" {
		return name
	}
	return name + "@" + version
}

func escapeMermaidLabel(value string) string {
	return strings.NewReplacer(
		"\\", "\\\\",
		"\"", "\\\"",
		"\r\n", " ",
		"\r", " ",
		"\n", " ",
	).Replace(value)
}

func markdownInlineCode(value string) string {
	value = strings.NewReplacer(
		"\r\n", " ",
		"\r", " ",
		"\n", " ",
		"|", "\\|",
		"`", "&#96;",
	).Replace(value)
	return "`" + value + "`"
}
