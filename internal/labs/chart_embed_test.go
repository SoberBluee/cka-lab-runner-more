package labs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMaterializeCKAWebappChart(t *testing.T) {
	dir := t.TempDir()
	chartDir, err := materializeCKAWebappChart(dir)
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	for _, rel := range []string{
		"Chart.yaml",
		"values.yaml",
		"templates/deployment.yaml",
		"templates/service.yaml",
		"templates/_helpers.tpl",
	} {
		if _, err := os.Stat(filepath.Join(chartDir, rel)); err != nil {
			t.Errorf("missing %s: %v", rel, err)
		}
	}
}

func TestExtractJSONInt(t *testing.T) {
	blob := `{"name":"webshop","version":2,"info":{"status":"deployed","revision":3}}`
	if got := extractJSONInt(blob, "revision"); got != 3 {
		t.Fatalf("revision: got %d want 3", got)
	}
	if got := extractJSONInt(blob, "version"); got != 2 {
		t.Fatalf("version: got %d want 2", got)
	}
}
