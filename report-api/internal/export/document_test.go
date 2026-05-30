package export

import (
	"testing"
	"time"
)

func TestDocument_StructFields(t *testing.T) {
	now := time.Now().UTC()
	doc := Document{
		Title:       "Test Report",
		Subtitle:    "Unit Test",
		GeneratedAt: now,
		Meta: []MetaLine{
			{Label: "Key", Value: "Value"},
		},
		Headers:    []string{"Col1", "Col2"},
		Rows:       [][]string{{"a", "b"}, {"c", "d"}},
		FooterNote: "footer",
	}

	if doc.Title != "Test Report" {
		t.Errorf("Title = %q, want Test Report", doc.Title)
	}
	if doc.Subtitle != "Unit Test" {
		t.Errorf("Subtitle = %q, want Unit Test", doc.Subtitle)
	}
	if doc.GeneratedAt != now {
		t.Errorf("GeneratedAt mismatch")
	}
	if len(doc.Meta) != 1 {
		t.Errorf("Meta count = %d, want 1", len(doc.Meta))
	}
	if doc.Meta[0].Label != "Key" || doc.Meta[0].Value != "Value" {
		t.Errorf("Meta[0] = %v, want {Key, Value}", doc.Meta[0])
	}
	if len(doc.Headers) != 2 {
		t.Errorf("Headers count = %d, want 2", len(doc.Headers))
	}
	if len(doc.Rows) != 2 {
		t.Errorf("Rows count = %d, want 2", len(doc.Rows))
	}
	if doc.FooterNote != "footer" {
		t.Errorf("FooterNote = %q, want footer", doc.FooterNote)
	}
}

func TestMetaLine_StructFields(t *testing.T) {
	ml := MetaLine{Label: "Period", Value: "2026-01 to 2026-12"}
	if ml.Label != "Period" {
		t.Errorf("Label = %q, want Period", ml.Label)
	}
	if ml.Value != "2026-01 to 2026-12" {
		t.Errorf("Value = %q", ml.Value)
	}
}
