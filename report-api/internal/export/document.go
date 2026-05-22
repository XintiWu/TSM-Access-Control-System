package export

import "time"

// Document is the shared layout model for CSV and PDF exporters.
type Document struct {
	Title       string
	Subtitle    string
	GeneratedAt time.Time
	Meta        []MetaLine
	Headers     []string
	Rows        [][]string
	FooterNote  string
}

// MetaLine is a key-value line shown under the title block.
type MetaLine struct {
	Label string
	Value string
}
