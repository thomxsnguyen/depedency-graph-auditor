package filegraph

import (
	"reflect"
	"testing"
)

func TestExtractPythonImports(t *testing.T) {
	source := []byte(`
"""Module docs mentioning import ignored.fake."""
import pc_diagnostic.cache
import pc_diagnostic.models as models
import first, second as second_alias
from pc_diagnostic.models import Snapshot
from .bridge import TelemetryBridge
from ..alerts.models import Incident
from . import helpers, constants
from pc_diagnostic.gui.components import (
    ProcessTableWidget,
    MetricCard as Card,
)

if TYPE_CHECKING:
    from pc_diagnostic.providers.base import Provider  # local type import

text = "import ignored.string"
# from ignored.comment import Value
`)

	got, err := ExtractPythonImports(source)
	if err != nil {
		t.Fatal(err)
	}
	want := []PythonImport{
		{Module: "pc_diagnostic.cache"},
		{Module: "pc_diagnostic.models"},
		{Module: "first"},
		{Module: "second"},
		{Module: "pc_diagnostic.models", Names: []string{"Snapshot"}},
		{Module: "bridge", Level: 1, Names: []string{"TelemetryBridge"}},
		{Module: "alerts.models", Level: 2, Names: []string{"Incident"}},
		{Level: 1, Names: []string{"helpers", "constants"}},
		{Module: "pc_diagnostic.gui.components", Names: []string{"ProcessTableWidget", "MetricCard"}},
		{Module: "pc_diagnostic.providers.base", Names: []string{"Provider"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("imports:\n got: %+v\nwant: %+v", got, want)
	}
}

func TestExtractPythonImportsHandlesBackslashContinuation(t *testing.T) {
	got, err := ExtractPythonImports([]byte("import pc_diagnostic.cache, \\\npc_diagnostic.models\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []PythonImport{{Module: "pc_diagnostic.cache"}, {Module: "pc_diagnostic.models"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("imports: got %+v, want %+v", got, want)
	}
}

func TestExtractPythonImportsRejectsMalformedSupportedSyntax(t *testing.T) {
	for _, source := range []string{
		"import\n",
		"from pc_diagnostic.models Snapshot\n",
		"from pc_diagnostic.models import\n",
		"import module as\n",
		`"""unterminated`,
	} {
		t.Run(source, func(t *testing.T) {
			if _, err := ExtractPythonImports([]byte(source)); err == nil {
				t.Fatalf("expected error for %q", source)
			}
		})
	}
}
