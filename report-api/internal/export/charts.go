package export

import (
	"bytes"
	"fmt"
	"time"

	"github.com/tsmc/report-api/internal/model"
	"github.com/wcharczuk/go-chart/v2"
	"github.com/wcharczuk/go-chart/v2/drawing"
)

const chartWidth = 680
const chartHeight = 280

var yAxisFromZero = chart.YAxis{
	Range: &chart.ContinuousRange{Min: 0},
}

func yAxisFromZeroTo(maxVal float64) chart.YAxis {
	max := maxVal * 1.25
	if max < 1 {
		max = 1
	}
	return chart.YAxis{Range: &chart.ContinuousRange{Min: 0, Max: max}}
}

func renderBarPNG(bar chart.BarChart) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	if err := bar.Render(chart.PNG, buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func renderChartPNG(c *chart.Chart) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	if err := c.Render(chart.PNG, buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func chartIndexed(n int) []float64 {
	v := make([]float64, n)
	for i := range v {
		v[i] = float64(i + 1)
	}
	return v
}

func emptyChartPNG(msg string) ([]byte, error) {
	graph := chart.BarChart{
		Title:  msg,
		Width:  chartWidth,
		Height: chartHeight,
		YAxis:  yAxisFromZero,
		Bars:   []chart.Value{{Label: "—", Value: 0}},
	}
	return renderBarPNG(graph)
}

// DoorHeatmapChartPNG — gate traffic ranking (horizontal bars via labels).
func DoorHeatmapChartPNG(resp *model.DoorHeatmapResponse) ([]byte, error) {
	if resp == nil || len(resp.Doors) == 0 {
		return emptyChartPNG("No door traffic in selected window")
	}
	bars := make([]chart.Value, len(resp.Doors))
	for i, d := range resp.Doors {
		label := d.DoorName
		if label == "" {
			label = d.DoorID
		}
		if len(label) > 22 {
			label = label[:19] + "..."
		}
		bars[i] = chart.Value{Label: label, Value: float64(d.SwipeCount)}
	}
	graph := chart.BarChart{
		Title:      fmt.Sprintf("Gate Traffic (last %d min)", resp.WindowMinutes),
		TitleStyle: chart.Style{FontSize: 13},
		Background: chart.Style{Padding: chart.Box{Top: 36, Left: 24, Right: 24, Bottom: 24}},
		Width:      chartWidth,
		Height:     chartHeight,
		YAxis:      yAxisFromZero,
		Bars:       bars,
	}
	return renderBarPNG(graph)
}

// SubUnitsComparisonChartPNG — child org units IN/OUT comparison.
func SubUnitsComparisonChartPNG(resp *model.DepartmentReportResponse) ([]byte, error) {
	if resp == nil || len(resp.SubUnits) == 0 {
		return emptyChartPNG("No sub-units to compare")
	}
	var bars []chart.Value
	for _, su := range resp.SubUnits {
		name := su.OrgUnitName
		if len(name) > 14 {
			name = name[:11] + "..."
		}
		bars = append(bars,
			chart.Value{Label: name + " IN", Value: float64(su.TotalEntries)},
			chart.Value{Label: name + " OUT", Value: float64(su.TotalExits)},
		)
	}
	graph := chart.BarChart{
		Title:      fmt.Sprintf("Sub-Unit Comparison — under %s", resp.OrgUnitName),
		TitleStyle: chart.Style{FontSize: 13},
		Background: chart.Style{Padding: chart.Box{Top: 36, Left: 24, Right: 24, Bottom: 24}},
		Width:      chartWidth,
		Height:     chartHeight,
		YAxis:      yAxisFromZero,
		Bars:       bars,
	}
	return renderBarPNG(graph)
}

// DepartmentEntriesChartPNG — period IN/OUT (Y axis from 0).
func DepartmentEntriesChartPNG(resp *model.DepartmentReportResponse) ([]byte, error) {
	if resp == nil {
		return emptyChartPNG("No department data")
	}
	var bars []chart.Value
	if len(resp.Periods) <= 1 {
		bars = []chart.Value{
			{Label: "Total IN", Value: float64(resp.Summary.TotalEntries)},
			{Label: "Total OUT", Value: float64(resp.Summary.TotalExits)},
		}
	} else {
		for _, p := range resp.Periods {
			lbl := p.PeriodStart
			if len(lbl) > 10 {
				lbl = lbl[5:]
			}
			bars = append(bars,
				chart.Value{Label: lbl + " IN", Value: float64(p.TotalEntries)},
				chart.Value{Label: lbl + " OUT", Value: float64(p.TotalExits)},
			)
		}
	}
	graph := chart.BarChart{
		Title:      fmt.Sprintf("Entries / Exits — %s", resp.OrgUnitName),
		TitleStyle: chart.Style{FontSize: 13},
		Background: chart.Style{Padding: chart.Box{Top: 36, Left: 24, Right: 24, Bottom: 24}},
		Width:      chartWidth,
		Height:     chartHeight,
		YAxis:      yAxisFromZero,
		Bars:       bars,
	}
	return renderBarPNG(graph)
}

func periodLabel(p model.PeriodReport) string {
	lbl := p.PeriodStart
	if len(lbl) > 10 {
		lbl = lbl[5:]
	}
	return lbl
}

// DepartmentHoursChartPNG — avg hours only (separate scale from late rate).
func DepartmentHoursChartPNG(resp *model.DepartmentReportResponse) ([]byte, error) {
	if resp == nil || len(resp.Periods) == 0 {
		return emptyChartPNG("No period data")
	}
	if len(resp.Periods) == 1 {
		p := resp.Periods[0]
		graph := chart.BarChart{
			Title:      fmt.Sprintf("Avg Hours — %s", resp.OrgUnitName),
			TitleStyle: chart.Style{FontSize: 13},
			Background: chart.Style{Padding: chart.Box{Top: 36, Left: 24, Right: 24, Bottom: 24}},
			Width:      chartWidth,
			Height:     chartHeight,
			YAxis:      yAxisFromZeroTo(p.AvgHours),
			Bars:       []chart.Value{{Label: periodLabel(p), Value: p.AvgHours}},
		}
		return renderBarPNG(graph)
	}
	n := len(resp.Periods)
	y := make([]float64, n)
	for i, p := range resp.Periods {
		y[i] = p.AvgHours
	}
	graph := chart.Chart{
		Title:      fmt.Sprintf("Avg Hours — %s", resp.OrgUnitName),
		TitleStyle: chart.Style{FontSize: 13},
		Background: chart.Style{Padding: chart.Box{Top: 36, Left: 20, Right: 20, Bottom: 20}},
		Width:      chartWidth,
		Height:     chartHeight,
		YAxis:      yAxisFromZero,
		Series: []chart.Series{
			chart.ContinuousSeries{
				Name:    "Hours",
				Style:   chart.Style{StrokeColor: drawing.Color{R: 59, G: 130, B: 246, A: 255}, StrokeWidth: 2.5},
				XValues: chartIndexed(n),
				YValues: y,
			},
		},
	}
	return renderChartPNG(&graph)
}

// DepartmentLateChartPNG — late rate % only.
func DepartmentLateChartPNG(resp *model.DepartmentReportResponse) ([]byte, error) {
	if resp == nil || len(resp.Periods) == 0 {
		return emptyChartPNG("No period data")
	}
	if len(resp.Periods) == 1 {
		p := resp.Periods[0]
		graph := chart.BarChart{
			Title:      fmt.Sprintf("Late Rate %% — %s", resp.OrgUnitName),
			TitleStyle: chart.Style{FontSize: 13},
			Background: chart.Style{Padding: chart.Box{Top: 36, Left: 24, Right: 24, Bottom: 24}},
			Width:      chartWidth,
			Height:     chartHeight,
			YAxis:      chart.YAxis{Range: &chart.ContinuousRange{Min: 0, Max: 100}},
			Bars:       []chart.Value{{Label: periodLabel(p), Value: p.LateRate * 100}},
		}
		return renderBarPNG(graph)
	}
	n := len(resp.Periods)
	y := make([]float64, n)
	for i, p := range resp.Periods {
		y[i] = p.LateRate * 100
	}
	graph := chart.Chart{
		Title:      fmt.Sprintf("Late Rate %% — %s", resp.OrgUnitName),
		TitleStyle: chart.Style{FontSize: 13},
		Background: chart.Style{Padding: chart.Box{Top: 36, Left: 20, Right: 20, Bottom: 20}},
		Width:      chartWidth,
		Height:     chartHeight,
		YAxis:      chart.YAxis{Range: &chart.ContinuousRange{Min: 0, Max: 100}},
		Series: []chart.Series{
			chart.ContinuousSeries{
				Name:    "Late %",
				Style:   chart.Style{StrokeColor: drawing.Color{R: 245, G: 158, B: 11, A: 255}, StrokeWidth: 2.5},
				XValues: chartIndexed(n),
				YValues: y,
			},
		},
	}
	return renderChartPNG(&graph)
}

// AttendanceHoursChartPNG — average hours trend.
func AttendanceHoursChartPNG(resp *model.AttendanceTrendsResponse) ([]byte, error) {
	if resp == nil || len(resp.Series) == 0 {
		return emptyChartPNG("No attendance data")
	}
	if len(resp.Series) == 1 {
		p := resp.Series[0]
		lbl := p.PeriodStart
		if len(lbl) > 10 {
			lbl = lbl[5:]
		}
		graph := chart.BarChart{
			Title:      fmt.Sprintf("Attendance Avg Hours — %s", resp.OrgUnitName),
			TitleStyle: chart.Style{FontSize: 13},
			Background: chart.Style{Padding: chart.Box{Top: 36, Left: 24, Right: 24, Bottom: 24}},
			Width:      chartWidth,
			Height:     chartHeight,
			YAxis:      yAxisFromZeroTo(p.AvgHours),
			Bars:       []chart.Value{{Label: lbl, Value: p.AvgHours}},
		}
		return renderBarPNG(graph)
	}
	n := len(resp.Series)
	y := make([]float64, n)
	for i, p := range resp.Series {
		y[i] = p.AvgHours
	}
	graph := chart.Chart{
		Title:      fmt.Sprintf("Attendance Avg Hours — %s", resp.OrgUnitName),
		TitleStyle: chart.Style{FontSize: 13},
		Background: chart.Style{Padding: chart.Box{Top: 36, Left: 20, Right: 20, Bottom: 20}},
		Width:      chartWidth,
		Height:     chartHeight,
		YAxis:      yAxisFromZero,
		Series: []chart.Series{
			chart.ContinuousSeries{
				Name:    "Avg Hours",
				Style:   chart.Style{StrokeColor: drawing.Color{R: 34, G: 197, B: 94, A: 255}, StrokeWidth: 2.5},
				XValues: chartIndexed(n),
				YValues: y,
			},
		},
	}
	return renderChartPNG(&graph)
}

// AttendanceLateChartPNG — late rate trend.
func AttendanceLateChartPNG(resp *model.AttendanceTrendsResponse) ([]byte, error) {
	if resp == nil || len(resp.Series) == 0 {
		return emptyChartPNG("No attendance data")
	}
	if len(resp.Series) == 1 {
		p := resp.Series[0]
		lbl := p.PeriodStart
		if len(lbl) > 10 {
			lbl = lbl[5:]
		}
		graph := chart.BarChart{
			Title:      "Attendance Late Rate % (09:00 UTC rule)",
			TitleStyle: chart.Style{FontSize: 13},
			Background: chart.Style{Padding: chart.Box{Top: 36, Left: 24, Right: 24, Bottom: 24}},
			Width:      chartWidth,
			Height:     chartHeight,
			YAxis:      chart.YAxis{Range: &chart.ContinuousRange{Min: 0, Max: 100}},
			Bars:       []chart.Value{{Label: lbl, Value: p.LateRate * 100}},
		}
		return renderBarPNG(graph)
	}
	n := len(resp.Series)
	y := make([]float64, n)
	for i, p := range resp.Series {
		y[i] = p.LateRate * 100
	}
	graph := chart.Chart{
		Title:      "Attendance Late Rate % (09:00 UTC rule)",
		TitleStyle: chart.Style{FontSize: 13},
		Background: chart.Style{Padding: chart.Box{Top: 36, Left: 20, Right: 20, Bottom: 20}},
		Width:      chartWidth,
		Height:     chartHeight,
		YAxis:      chart.YAxis{Range: &chart.ContinuousRange{Min: 0, Max: 100}},
		Series: []chart.Series{
			chart.ContinuousSeries{
				Name:    "Late %",
				Style:   chart.Style{StrokeColor: drawing.Color{R: 239, G: 68, B: 68, A: 255}, StrokeWidth: 2.5},
				XValues: chartIndexed(n),
				YValues: y,
			},
		},
	}
	return renderChartPNG(&graph)
}

// ReportPDFMeta is section 1 of the visual department PDF.
type ReportPDFMeta struct {
	ReportTitle      string
	StatWindowStart  string
	StatWindowEnd    string
	GeneratedAtUTC   time.Time
	RequesterLabel   string
	OrgScopePath     string
	OrgUnitName      string
}

// VisualReportBundle holds data for full PDF with charts.
type VisualReportBundle struct {
	Department       *model.DepartmentReportResponse
	Heatmap          *model.DoorHeatmapResponse
	Trends           *model.AttendanceTrendsResponse
	ShowCorpOverview bool
	Meta             ReportPDFMeta
	Security         model.SecurityDenySummary
	DetailRows       []model.ReportDetailRow
}
