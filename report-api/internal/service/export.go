package service

import (
	"context"
	"fmt"
	"os"

	"github.com/tsmc/report-api/internal/export"
	"github.com/tsmc/report-api/internal/model"
)

// BuildExportDocument loads data and builds a layout document for the given export type.
func (s *ReportService) BuildExportDocument(ctx context.Context, req model.ExportRequest, userID, requesterOrgUnitID string) (export.Document, error) {
	reportType := req.Type
	if reportType == "" {
		reportType = "events"
	}

	switch reportType {
	case "personal":
		resp, err := s.GetPersonalReport(ctx, userID, req.StartDate, req.EndDate)
		if err != nil {
			return export.Document{}, err
		}
		return export.PersonalDocument(resp), nil

	case "department":
		if req.OrgUnitID == "" {
			return export.Document{}, fmt.Errorf("orgUnitId is required for department export")
		}
		deptReq := model.DepartmentReportRequest{
			OrgUnitID: req.OrgUnitID, StartDate: req.StartDate, EndDate: req.EndDate,
			Granularity: req.Granularity,
		}
		resp, err := s.GetDepartmentReport(ctx, deptReq, requesterOrgUnitID)
		if err != nil {
			return export.Document{}, err
		}
		return export.DepartmentDocument(resp), nil

	default: // events
		if req.OrgUnitID == "" {
			return export.Document{}, fmt.Errorf("orgUnitId is required for events export")
		}
		inSubtree, err := s.orgRepo.IsInSubtree(ctx, requesterOrgUnitID, req.OrgUnitID)
		if err != nil {
			return export.Document{}, fmt.Errorf("check subtree: %w", err)
		}
		if !inSubtree {
			return export.Document{}, fmt.Errorf("access denied: orgUnitId %s is not in your subtree", req.OrgUnitID)
		}
		orgUnit, err := s.orgRepo.GetOrgUnit(ctx, req.OrgUnitID)
		if err != nil || orgUnit == nil {
			return export.Document{}, fmt.Errorf("org unit not found: %s", req.OrgUnitID)
		}
		subtreeIDs, err := s.orgRepo.GetSubtreeIDs(ctx, req.OrgUnitID)
		if err != nil {
			return export.Document{}, err
		}
		events, err := s.inoutRepo.GetEventsForExport(ctx, subtreeIDs, req.StartDate, req.EndDate)
		if err != nil {
			return export.Document{}, err
		}
		name := orgUnit.Name
		return export.EventsDocument(name, req.OrgUnitID, req.StartDate, req.EndDate, events), nil
	}
}

// RenderExport produces file bytes for csv or pdf format.
func RenderExport(doc export.Document, format string) ([]byte, string, error) {
	switch format {
	case "pdf":
		b, err := export.WritePDF(doc)
		return b, ".pdf", err
	default:
		b, err := export.WriteCSV(doc)
		return b, ".csv", err
	}
}

// ExportSync builds and renders an export (used by GET /reports/export).
func (s *ReportService) ExportSync(ctx context.Context, req model.ExportRequest, userID, requesterOrgUnitID string) ([]byte, string, error) {
	doc, err := s.BuildExportDocument(ctx, req, userID, requesterOrgUnitID)
	if err != nil {
		return nil, "", err
	}
	return RenderExport(doc, req.Format)
}

// RunExportJob generates a file asynchronously and updates the job store.
func (s *ReportService) RunExportJob(jobID string, req model.ExportRequest, userID, requesterOrgUnitID string) {
	if s.jobs == nil {
		return
	}
	go func() {
		ctx := context.Background()
		doc, err := s.BuildExportDocument(ctx, req, userID, requesterOrgUnitID)
		if err != nil {
			s.jobs.MarkFailed(jobID, err.Error())
			return
		}
		data, ext, err := RenderExport(doc, req.Format)
		if err != nil {
			s.jobs.MarkFailed(jobID, err.Error())
			return
		}
		path := s.jobs.FilePath(jobID, ext)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			s.jobs.MarkFailed(jobID, err.Error())
			return
		}
		s.jobs.MarkDone(jobID)
	}()
}
