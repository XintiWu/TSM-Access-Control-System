package export

import (
	"testing"
	"time"
)

func TestWritePDFProducesBytes(t *testing.T) {
	doc := Document{
		Title: "TSMC Access Control Report", Subtitle: "Test",
		GeneratedAt: time.Now().UTC(),
		Meta:        []MetaLine{{Label: "Period", Value: "2026-05-01 to 2026-05-19"}},
		Headers:     []string{"Date", "Entries"},
		Rows:        [][]string{{"2026-05-19", "10"}},
		FooterNote:  "Internal use only",
	}
	b, err := WritePDF(doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) < 100 {
		t.Fatalf("pdf too small: %d bytes", len(b))
	}
	if b[0] != '%' || b[1] != 'P' || b[2] != 'D' || b[3] != 'F' {
		t.Fatalf("not a PDF header: %q", b[:8])
	}
}
