package findings

import (
	"testing"
)

func TestFinding_ZeroValueIsUsable(t *testing.T) {
	var f Finding
	if f.Severity != SeverityLow {
		t.Errorf("zero Finding.Severity = %v, want SeverityLow", f.Severity)
	}
	if f.Category != "" {
		t.Errorf("zero Finding.Category = %q, want empty", f.Category)
	}
}

func TestCategory_String(t *testing.T) {
	if string(CategoryCardinality) != "cardinality" {
		t.Errorf("CategoryCardinality = %q, want %q", CategoryCardinality, "cardinality")
	}
}
