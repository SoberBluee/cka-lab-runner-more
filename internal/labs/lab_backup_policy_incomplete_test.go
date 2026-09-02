package labs

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestBackupPolicyCRDVariantsAreValidYAML(t *testing.T) {
	for _, strict := range []bool{false, true} {
		var parsed map[string]any
		if err := yaml.Unmarshal([]byte(backupPolicyCRD(strict)), &parsed); err != nil {
			t.Fatalf("strict=%v: CRD is not valid YAML: %v", strict, err)
		}
		if parsed["kind"] != "CustomResourceDefinition" {
			t.Fatalf("strict=%v: unexpected kind %v", strict, parsed["kind"])
		}
	}
}

func TestBackupPolicyCRDStrictVariantTightensSchema(t *testing.T) {
	loose := backupPolicyCRD(false)
	strict := backupPolicyCRD(true)

	if strings.Contains(loose, "- keepLast") {
		t.Error("loose variant should not require keepLast")
	}
	if strings.Contains(loose, "maximum: 30") {
		t.Error("loose variant should not bound keepLast")
	}
	if !strings.Contains(strict, "- keepLast") {
		t.Error("strict variant should require keepLast")
	}
	if !strings.Contains(strict, "minimum: 1") || !strings.Contains(strict, "maximum: 30") {
		t.Error("strict variant should bound keepLast between 1 and 30")
	}
}
