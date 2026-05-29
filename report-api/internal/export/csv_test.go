package export

import (
	"strings"
	"testing"
)

func TestWriteCSV(t *testing.T) {
	doc := Document{
		Title:   "Test Report",
		Meta:    []MetaLine{{Label: "Period", Value: "2026-05"}},
		Headers: []string{"Col1", "Col2"},
		Rows:    [][]string{{"a", "b"}, {"c", "d"}},
	}
	data, err := WriteCSV(doc)
	if err != nil {
		t.Fatalf("WriteCSV() error = %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty CSV output")
	}
	s := string(data)
	if !containsAll(s, "Period", "2026-05", "Col1", "Col2", "a", "b") {
		t.Errorf("unexpected CSV content: %s", s)
	}
}

func TestWriteCSV_EmptyDoc(t *testing.T) {
	data, err := WriteCSV(Document{Headers: []string{"H1"}, Rows: [][]string{}})
	if err != nil {
		t.Fatalf("WriteCSV() error = %v", err)
	}
	if len(data) == 0 {
		t.Error("expected CSV bytes")
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !strings.Contains(s, p) {
			return false
		}
	}
	return true
}
