package export

import (
	"bytes"
	"encoding/csv"
	"fmt"
)

func WriteCSV(doc Document) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	for _, m := range doc.Meta {
		if err := w.Write([]string{m.Label, m.Value}); err != nil {
			return nil, err
		}
	}
	if len(doc.Meta) > 0 {
		if err := w.Write([]string{}); err != nil {
			return nil, err
		}
	}
	if err := w.Write(doc.Headers); err != nil {
		return nil, err
	}
	for _, row := range doc.Rows {
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("csv write: %w", err)
	}
	return buf.Bytes(), nil
}
