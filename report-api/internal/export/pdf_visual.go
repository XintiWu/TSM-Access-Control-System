package export

import (
	"bytes"
	"fmt"
	"image"
	_ "image/png"

	"github.com/jung-kurt/gofpdf"
	"github.com/tsmc/report-api/internal/model"
)

const (
	chartMaxHeightMM = 58.0
	chartGapMM       = 6.0
)

// WriteVisualReportPDF builds a multi-section PDF with embedded chart images.
func WriteVisualReportPDF(bundle VisualReportBundle) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 18)
	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont("Helvetica", "I", 8)
		pdf.SetTextColor(120, 120, 120)
		pdf.CellFormat(0, 8, "Internal use only - TSMC Access Control", "", 0, "L", false, 0, "")
		pdf.CellFormat(0, 8, fmt.Sprintf("Page %d", pdf.PageNo()), "", 0, "R", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	})

	dept := bundle.Department
	if dept == nil {
		return nil, fmt.Errorf("department report required")
	}

	compact := isCompactReport(bundle)

	// Page 1: Metadata + Summary + Detail table (+ optional period breakdown)
	pdf.AddPage()
	writeReportPageOne(pdf, bundle)

	// CEO / sub-units
	if bundle.ShowCorpOverview {
		pdf.AddPage()
		writeSectionTitle(pdf, "CEO - Full Site Overview", "Company-wide KPI, sub-units, top gates")
		writeMetaBlock(pdf, deptSummaryMeta(dept))
		pdf.Ln(2)
		if len(dept.SubUnits) > 0 {
			if png, err := SubUnitsComparisonChartPNG(dept); err == nil {
				embedPNG(pdf, png, "exec_subunits", chartMaxHeightMM)
			}
			writeSubUnitsTable(pdf, dept.SubUnits)
		}
		writeTopGatesList(pdf, bundle.Heatmap)
	} else if len(dept.SubUnits) > 0 {
		pdf.AddPage()
		writeSectionTitle(pdf, "Sub-Unit Comparison", "Direct child departments")
		if png, err := SubUnitsComparisonChartPNG(dept); err == nil {
			embedPNG(pdf, png, "subunit_cmp", chartMaxHeightMM)
		}
		writeSubUnitsTable(pdf, dept.SubUnits)
	}

	// Charts
	if compact {
		writeCompactDepartmentCharts(pdf, dept)
		writeCompactAnalytics(pdf, bundle)
	} else {
		writeFullDepartmentCharts(pdf, dept)
		writeFullAnalytics(pdf, bundle)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeReportPageOne(pdf *gofpdf.Fpdf, bundle VisualReportBundle) {
	dept := bundle.Department
	meta := bundle.Meta

	writeHeader(pdf, meta.ReportTitle, fmt.Sprintf("Org scope: %s", meta.OrgUnitName))

	// 1. Metadata Header
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(0, 7, "1. Report Metadata", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	writeKV(pdf, "Report name", meta.ReportTitle)
	writeKV(pdf, "Statistical window", meta.StatWindowStart+"  to  "+meta.StatWindowEnd)
	writeKV(pdf, "Generated at (UTC)", meta.GeneratedAtUTC.Format("2006-01-02 15:04:05"))
	writeKV(pdf, "Produced by", meta.RequesterLabel)
	writeKV(pdf, "Organization scope", meta.OrgScopePath)
	pdf.Ln(3)

	// 2. Pre-aggregated Summary
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(0, 7, "2. Executive Summary (Pre-aggregated)", "", 1, "L", false, 0, "")
	sm := dept.Summary
	sec := bundle.Security
	writeSummaryGrid(pdf, []summaryItem{
		{"Total Entries (IN)", fmt.Sprintf("%d", sm.TotalEntries)},
		{"Total Exits (OUT)", fmt.Sprintf("%d", sm.TotalExits)},
		{"Avg On-site Hours / day", fmt.Sprintf("%.2f h", sm.AvgHoursPerDay)},
		{"Unique Employees", fmt.Sprintf("%d", sm.UniqueEmployees)},
		{"Late Rate", fmt.Sprintf("%.1f%%", sm.LateRate*100)},
		{"Anti-passback Denies", fmt.Sprintf("%d", sec.AntiPassbackDenies)},
		{"Blacklist / Banned Swipes", fmt.Sprintf("%d", sec.PermissionDenied)},
	})
	pdf.Ln(3)

	// 3. Detailed Data Table
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(0, 7, "3. Detailed Breakdown (by permission scope)", "", 1, "L", false, 0, "")
	if len(bundle.DetailRows) == 0 {
		pdf.SetFont("Helvetica", "", 9)
		pdf.CellFormat(0, 5, "No sub-units or employees with activity in this window.", "", 1, "L", false, 0, "")
	} else {
		writeDetailTable(pdf, bundle.DetailRows)
	}

	// Period breakdown — start on page 2 if page 1 is already full
	if len(dept.Periods) > 0 {
		if pdf.GetY() > 200 {
			pdf.AddPage()
		}
		pdf.Ln(2)
		pdf.SetFont("Helvetica", "B", 10)
		pdf.CellFormat(0, 6, "Period breakdown (granularity: "+dept.Granularity+")", "", 1, "L", false, 0, "")
		doc := DepartmentDocument(dept)
		writeTable(pdf, doc.Headers, doc.Rows)
	}
}

type summaryItem struct {
	label, value string
}

func pdfContentWidth(pdf *gofpdf.Fpdf) float64 {
	pageW, _ := pdf.GetPageSize()
	left, _, right, _ := pdf.GetMargins()
	return pageW - left - right
}

// writeKV renders label + value; value wraps inside the content area (no horizontal overflow).
func writeKV(pdf *gofpdf.Fpdf, label, value string) {
	const labelW = 42.0
	left, _, _, _ := pdf.GetMargins()
	contentW := pdfContentWidth(pdf)
	valueW := contentW - labelW
	y := pdf.GetY()
	pdf.SetFont("Helvetica", "B", 8)
	pdf.SetXY(left, y)
	pdf.CellFormat(labelW, 5, label+":", "", 0, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetXY(left+labelW, y)
	pdf.MultiCell(valueW, 5, value, "", "L", false)
}

func writeSummaryGrid(pdf *gofpdf.Fpdf, items []summaryItem) {
	pageW, _ := pdf.GetPageSize()
	left, _, right, _ := pdf.GetMargins()
	colW := (pageW - left - right) / 2
	y0 := pdf.GetY()
	for i, it := range items {
		col := i % 2
		row := i / 2
		x := left + float64(col)*colW
		y := y0 + float64(row)*12
		pdf.SetXY(x, y)
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(100, 100, 100)
		pdf.CellFormat(colW, 4, it.label, "", 0, "L", false, 0, "")
		pdf.SetXY(x, y+4)
		pdf.SetFont("Helvetica", "B", 11)
		pdf.SetTextColor(0, 0, 0)
		pdf.CellFormat(colW, 7, it.value, "", 0, "L", false, 0, "")
	}
	rows := (len(items) + 1) / 2
	pdf.SetY(y0 + float64(rows)*12 + 2)
}

func detailTableColWidths(pdf *gofpdf.Fpdf) []float64 {
	total := pdfContentWidth(pdf)
	// Must sum to 1.0 — Notes column gets the most width for wrapped text.
	ratios := []float64{0.09, 0.17, 0.14, 0.11, 0.11, 0.38}
	out := make([]float64, len(ratios))
	for i, r := range ratios {
		out[i] = total * r
	}
	return out
}

func writeDetailTable(pdf *gofpdf.Fpdf, rows []model.ReportDetailRow) {
	headers := []string{"Type", "ID", "Name", "Swipes", "Hours", "Anomaly Notes"}
	colWidths := detailTableColWidths(pdf)
	left, _, _, _ := pdf.GetMargins()
	const lineH = 4.0

	drawHeader := func() {
		pdf.SetFont("Helvetica", "B", 7)
		pdf.SetFillColor(230, 230, 230)
		x := left
		y := pdf.GetY()
		for i, h := range headers {
			pdf.SetXY(x, y)
			pdf.CellFormat(colWidths[i], 6, h, "1", 0, "C", true, 0, "")
			x += colWidths[i]
		}
		pdf.SetY(y + 6)
	}

	drawHeader()
	for i, r := range rows {
		if pdf.GetY() > 250 {
			pdf.AddPage()
			drawHeader()
		}
		fill := i%2 == 0
		if fill {
			pdf.SetFillColor(248, 248, 248)
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		cells := []string{
			r.EntityType,
			shortEntityID(r.EntityID),
			r.EntityName,
			fmt.Sprintf("%d", r.TotalSwipes),
			fmt.Sprintf("%.2f", r.TotalHours),
			r.AnomalyNotes,
		}
		writeTableRowCells(pdf, left, colWidths, cells, lineH, fill)
	}
	pdf.Ln(3)
}

func shortEntityID(id string) string {
	if len(id) <= 13 {
		return id
	}
	return id[:8] + "..." + id[len(id)-4:]
}

// writeTableRowCells draws one row with MultiCell wrapping per column (shared row height).
func writeTableRowCells(pdf *gofpdf.Fpdf, left float64, colWidths []float64, cells []string, lineH float64, fill bool) {
	pdf.SetFont("Helvetica", "", 6.5)
	y0 := pdf.GetY()
	maxH := lineH
	for i, cell := range cells {
		lines := pdf.SplitText(cell, colWidths[i])
		h := float64(len(lines)) * lineH
		if h < lineH {
			h = lineH
		}
		if h > maxH {
			maxH = h
		}
	}
	x := left
	for i, cell := range cells {
		pdf.SetXY(x, y0)
		pdf.MultiCell(colWidths[i], lineH, cell, "1", "L", fill)
		x += colWidths[i]
	}
	pdf.SetY(y0 + maxH)
}

func isCompactReport(bundle VisualReportBundle) bool {
	dept := bundle.Department
	if dept == nil {
		return true
	}
	return !bundle.ShowCorpOverview && len(dept.SubUnits) == 0 && len(dept.Periods) <= 1
}

func writeCompactDepartmentCharts(pdf *gofpdf.Fpdf, dept *model.DepartmentReportResponse) {
	pdf.AddPage()
	writeSectionTitle(pdf, "Charts - Department", "Entries, avg hours, late rate")
	charts := []struct {
		name string
		gen  func(*model.DepartmentReportResponse) ([]byte, error)
	}{
		{"dept_inout", DepartmentEntriesChartPNG},
		{"dept_hours", DepartmentHoursChartPNG},
		{"dept_late", DepartmentLateChartPNG},
	}
	for _, c := range charts {
		png, err := c.gen(dept)
		if err != nil {
			continue
		}
		if pdf.GetY()+chartMaxHeightMM > 265 {
			pdf.AddPage()
		}
		embedPNG(pdf, png, c.name, chartMaxHeightMM)
	}
}

func writeFullDepartmentCharts(pdf *gofpdf.Fpdf, dept *model.DepartmentReportResponse) {
	sections := []struct {
		title, desc, name string
		gen               func(*model.DepartmentReportResponse) ([]byte, error)
	}{
		{"Charts - Traffic", "Entries vs exits", "dept_inout", DepartmentEntriesChartPNG},
		{"Charts - Avg Hours", "Hours per period", "dept_hours", DepartmentHoursChartPNG},
		{"Charts - Late Rate", "Late %", "dept_late", DepartmentLateChartPNG},
	}
	for _, s := range sections {
		pdf.AddPage()
		writeSectionTitle(pdf, s.title, s.desc)
		if png, err := s.gen(dept); err == nil {
			embedPNG(pdf, png, s.name, chartMaxHeightMM+14)
		}
	}
}

func trendsRedundantWithDept(dept *model.DepartmentReportResponse, trends *model.AttendanceTrendsResponse) bool {
	if dept == nil || trends == nil || len(dept.Periods) != 1 || len(trends.Series) != 1 {
		return false
	}
	return dept.Periods[0].PeriodStart == trends.Series[0].PeriodStart
}

func writeCompactAnalytics(pdf *gofpdf.Fpdf, bundle VisualReportBundle) {
	hasHeat := bundle.Heatmap != nil && len(bundle.Heatmap.Doors) > 0
	hasTrends := bundle.Trends != nil && len(bundle.Trends.Series) > 0 && !trendsRedundantWithDept(bundle.Department, bundle.Trends)
	if !hasHeat && !hasTrends {
		return
	}
	pdf.AddPage()
	writeSectionTitle(pdf, "Charts - Gate & Attendance", "")
	if hasHeat {
		if png, err := DoorHeatmapChartPNG(bundle.Heatmap); err == nil {
			embedPNG(pdf, png, "door_hm", chartMaxHeightMM)
		}
	}
	if hasTrends {
		if png, err := AttendanceHoursChartPNG(bundle.Trends); err == nil {
			embedPNG(pdf, png, "att_h", chartMaxHeightMM)
		}
		if png, err := AttendanceLateChartPNG(bundle.Trends); err == nil {
			embedPNG(pdf, png, "att_l", chartMaxHeightMM)
		}
	}
}

func writeFullAnalytics(pdf *gofpdf.Fpdf, bundle VisualReportBundle) {
	pdf.AddPage()
	writeSectionTitle(pdf, "Charts - Gate Heatmap", "")
	if bundle.Heatmap != nil {
		if png, err := DoorHeatmapChartPNG(bundle.Heatmap); err == nil {
			embedPNG(pdf, png, "door_hm", chartMaxHeightMM+14)
		}
	}
	pdf.AddPage()
	writeSectionTitle(pdf, "Charts - Attendance", "")
	if bundle.Trends != nil {
		if png, err := AttendanceHoursChartPNG(bundle.Trends); err == nil {
			embedPNG(pdf, png, "att_h", chartMaxHeightMM+14)
		}
		if png, err := AttendanceLateChartPNG(bundle.Trends); err == nil {
			embedPNG(pdf, png, "att_l", chartMaxHeightMM+14)
		}
	}
}

func deptSummaryMeta(dept *model.DepartmentReportResponse) []MetaLine {
	return []MetaLine{
		{Label: "Org Unit", Value: dept.OrgUnitName},
		{Label: "Period", Value: fmt.Sprintf("%s to %s", dept.StartDate, dept.EndDate)},
		{Label: "Total IN", Value: fmt.Sprintf("%d", dept.Summary.TotalEntries)},
		{Label: "Total OUT", Value: fmt.Sprintf("%d", dept.Summary.TotalExits)},
	}
}

func writeMetaBlock(pdf *gofpdf.Fpdf, lines []MetaLine) {
	for _, m := range lines {
		pdf.SetFont("Helvetica", "B", 9)
		pdf.CellFormat(34, 5, m.Label+":", "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 9)
		pdf.CellFormat(0, 5, m.Value, "", 1, "L", false, 0, "")
	}
}

func writeHeader(pdf *gofpdf.Fpdf, title, subtitle string) {
	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(0, 9, title, "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 11)
	pdf.CellFormat(0, 6, subtitle, "", 1, "L", false, 0, "")
	pdf.Ln(2)
}

func writeSectionTitle(pdf *gofpdf.Fpdf, title, desc string) {
	pdf.SetFont("Helvetica", "B", 14)
	pdf.CellFormat(0, 8, title, "", 1, "L", false, 0, "")
	if desc != "" {
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(100, 100, 100)
		pdf.CellFormat(0, 5, desc, "", 1, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	}
	pdf.Ln(3)
}

func embedPNG(pdf *gofpdf.Fpdf, png []byte, name string, maxHeightMM float64) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(png))
	if err != nil || cfg.Width == 0 {
		return
	}
	aspect := float64(cfg.Height) / float64(cfg.Width)
	pageW, _ := pdf.GetPageSize()
	left, _, right, _ := pdf.GetMargins()
	maxW := pageW - left - right
	w := maxW
	h := w * aspect
	if h > maxHeightMM {
		h = maxHeightMM
		w = h / aspect
	}
	y := pdf.GetY()
	if y+h > 275 {
		pdf.AddPage()
		y = pdf.GetY()
	}
	opt := gofpdf.ImageOptions{ImageType: "PNG", ReadDpi: false}
	imgName := fmt.Sprintf("%s_p%d_%d", name, pdf.PageNo(), int(y*10))
	_ = pdf.RegisterImageOptionsReader(imgName, opt, bytes.NewReader(png))
	x := left + (maxW-w)/2
	pdf.ImageOptions(imgName, x, y, w, h, false, opt, 0, "")
	pdf.SetY(y + h + chartGapMM)
}

func writeTopGatesList(pdf *gofpdf.Fpdf, heatmap *model.DoorHeatmapResponse) {
	if heatmap == nil || len(heatmap.Doors) == 0 {
		return
	}
	pdf.Ln(2)
	pdf.SetFont("Helvetica", "B", 9)
	pdf.CellFormat(0, 5, "Top gates (last hour):", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 8)
	for i, d := range heatmap.Doors {
		if i >= 5 {
			break
		}
		name := d.DoorName
		if name == "" {
			name = d.DoorID
		}
		pdf.CellFormat(0, 4, fmt.Sprintf("  %s - %d swipes", name, d.SwipeCount), "", 1, "L", false, 0, "")
	}
}

func writeSubUnitsTable(pdf *gofpdf.Fpdf, units []model.SubUnitSummary) {
	if len(units) == 0 {
		return
	}
	pdf.Ln(2)
	headers := []string{"Sub-Unit", "Entries", "Exits"}
	rows := make([][]string, len(units))
	for i, u := range units {
		rows[i] = []string{u.OrgUnitName, fmt.Sprintf("%d", u.TotalEntries), fmt.Sprintf("%d", u.TotalExits)}
	}
	writeTableWithTitle(pdf, "Sub-unit breakdown", headers, rows)
}

func writeTable(pdf *gofpdf.Fpdf, headers []string, rows [][]string) {
	writeTableWithTitle(pdf, "", headers, rows)
}

func writeTableWithTitle(pdf *gofpdf.Fpdf, title string, headers []string, rows [][]string) {
	if len(headers) == 0 {
		return
	}
	if title != "" {
		pdf.Ln(1)
		pdf.SetFont("Helvetica", "B", 10)
		pdf.CellFormat(0, 6, title, "", 1, "L", false, 0, "")
	}
	colW := pdfContentWidth(pdf) / float64(len(headers))
	drawHeader := func() {
		pdf.SetFont("Helvetica", "B", 7)
		pdf.SetFillColor(230, 230, 230)
		for _, h := range headers {
			pdf.CellFormat(colW, 6, h, "1", 0, "C", true, 0, "")
		}
		pdf.Ln(-1)
	}
	drawHeader()
	pdf.SetFont("Helvetica", "", 7)
	for i, row := range rows {
		if pdf.GetY() > 255 {
			pdf.AddPage()
			drawHeader()
			pdf.SetFont("Helvetica", "", 7)
		}
		fill := i%2 == 0
		if fill {
			pdf.SetFillColor(248, 248, 248)
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		for _, cell := range row {
			pdf.CellFormat(colW, 5, truncate(cell, 32), "1", 0, "L", fill, 0, "")
		}
		pdf.Ln(-1)
	}
	pdf.Ln(4)
}
