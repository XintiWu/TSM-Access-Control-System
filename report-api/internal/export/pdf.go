package export

import (
	"bytes"
	"fmt"

	"github.com/jung-kurt/gofpdf"
)

func WritePDF(doc Document) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 18)
	pdf.AddPage()

	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(0, 10, doc.Title, "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 12)
	pdf.CellFormat(0, 7, doc.Subtitle, "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.CellFormat(0, 5, fmt.Sprintf("Generated: %s UTC", doc.GeneratedAt.Format("2006-01-02 15:04:05")), "", 1, "L", false, 0, "")
	pdf.Ln(2)

	for _, m := range doc.Meta {
		pdf.SetFont("Helvetica", "B", 9)
		pdf.CellFormat(28, 5, m.Label+":", "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 9)
		pdf.CellFormat(0, 5, m.Value, "", 1, "L", false, 0, "")
	}
	pdf.Ln(4)

	colCount := len(doc.Headers)
	if colCount == 0 {
		var buf bytes.Buffer
		if err := pdf.Output(&buf); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}

	pageWidth, _ := pdf.GetPageSize()
	left, _, right, _ := pdf.GetMargins()
	colW := (pageWidth - left - right) / float64(colCount)

	drawHeader := func() {
		pdf.SetFont("Helvetica", "B", 8)
		pdf.SetFillColor(220, 220, 220)
		for _, h := range doc.Headers {
			pdf.CellFormat(colW, 7, h, "1", 0, "C", true, 0, "")
		}
		pdf.Ln(-1)
	}

	drawHeader()
	pdf.SetFont("Helvetica", "", 7)

	for i, row := range doc.Rows {
		fill := i%2 == 0
		if fill {
			pdf.SetFillColor(245, 245, 245)
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		if pdf.GetY() > 260 {
			pdf.AddPage()
			drawHeader()
			pdf.SetFont("Helvetica", "", 7)
		}
		for _, cell := range row {
			pdf.CellFormat(colW, 6, truncate(cell, 42), "1", 0, "L", fill, 0, "")
		}
		pdf.Ln(-1)
	}

	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont("Helvetica", "I", 8)
		pdf.SetTextColor(120, 120, 120)
		pdf.CellFormat(0, 8, doc.FooterNote, "", 0, "L", false, 0, "")
		pdf.CellFormat(0, 8, fmt.Sprintf("Page %d", pdf.PageNo()), "", 0, "R", false, 0, "")
	})

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("pdf output: %w", err)
	}
	return buf.Bytes(), nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}
