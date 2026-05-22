package export

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

type JobStatus string

const (
	JobPending JobStatus = "pending"
	JobDone    JobStatus = "done"
	JobFailed  JobStatus = "failed"
)

type Job struct {
	ID        string    `json:"jobId"`
	Status    JobStatus `json:"status"`
	Format    string    `json:"format"`
	Type      string    `json:"type,omitempty"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type JobStore struct {
	dir  string
	mu   sync.RWMutex
	jobs map[string]*Job
}

func NewJobStore(dir string) (*JobStore, error) {
	if dir == "" {
		dir = "/tmp/report-exports"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir export dir: %w", err)
	}
	return &JobStore{dir: dir, jobs: make(map[string]*Job)}, nil
}

func (s *JobStore) Create(format, reportType string) string {
	id := uuid.New().String()
	j := &Job{ID: id, Status: JobPending, Format: format, Type: reportType, CreatedAt: time.Now().UTC()}
	s.mu.Lock()
	s.jobs[id] = j
	s.mu.Unlock()
	return id
}

func (s *JobStore) Get(id string) (*Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j, ok := s.jobs[id]
	if !ok {
		return nil, false
	}
	cp := *j
	return &cp, true
}

func (s *JobStore) MarkDone(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.Status = JobDone
	}
}

func (s *JobStore) MarkFailed(id, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.Status = JobFailed
		j.Error = errMsg
	}
}

func (s *JobStore) FilePath(id, ext string) string {
	return filepath.Join(s.dir, id+ext)
}

func (s *JobStore) OpenResult(id string) (*os.File, string, error) {
	j, ok := s.Get(id)
	if !ok {
		return nil, "", fmt.Errorf("job not found")
	}
	if j.Status != JobDone {
		return nil, "", fmt.Errorf("job not ready: %s", j.Status)
	}
	ext := ".csv"
	if j.Format == "pdf" {
		ext = ".pdf"
	}
	path := s.FilePath(id, ext)
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	return f, "access-report-" + id[:8] + ext, nil
}
