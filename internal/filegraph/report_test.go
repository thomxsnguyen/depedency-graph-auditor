package filegraph

import (
	"bytes"
	"testing"
)

func TestReportIsDeterministic(t *testing.T) {
	first := NewStore()
	first.AddNode(Node{Path: "src/z.ts"})
	first.AddNode(Node{Path: "src/a.ts"})
	first.AddEdge(Edge{From: "src/z.ts", To: "src/a.ts"})
	first.AddEdge(Edge{From: "src/a.ts", To: "src/z.ts"})
	first.AddDiagnostic(Diagnostic{Path: "src/z.ts", Import: "./missing", Message: "unresolved local import"})

	second := NewStore()
	second.AddDiagnostic(Diagnostic{Path: "src/z.ts", Import: "./missing", Message: "unresolved local import"})
	second.AddEdge(Edge{From: "src/a.ts", To: "src/z.ts"})
	second.AddEdge(Edge{From: "src/z.ts", To: "src/a.ts"})
	second.AddNode(Node{Path: "src/a.ts"})
	second.AddNode(Node{Path: "src/z.ts"})

	firstJSON, err := MarshalReport(GenerateReport("project", first))
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := MarshalReport(GenerateReport("project", second))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstJSON, secondJSON) {
		t.Fatalf("reports differ:\n%s\n%s", firstJSON, secondJSON)
	}
	if len(firstJSON) == 0 || firstJSON[len(firstJSON)-1] != '\n' {
		t.Fatal("report must end in a newline")
	}
	if !bytes.Contains(firstJSON, []byte(`"schemaVersion": 1`)) {
		t.Fatalf("report omitted schema version:\n%s", firstJSON)
	}
}

func TestEmptyReportUsesJSONArrays(t *testing.T) {
	data, err := MarshalReport(GenerateReport("empty", NewStore()))
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range [][]byte{[]byte(`"nodes": []`), []byte(`"edges": []`), []byte(`"diagnostics": []`)} {
		if !bytes.Contains(data, field) {
			t.Fatalf("report omitted empty array %s:\n%s", field, data)
		}
	}
}
