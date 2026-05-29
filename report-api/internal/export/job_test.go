package export

import (
	"os"
	"testing"
)

func TestNewJobStore(t *testing.T) {
	dir := t.TempDir()
	store, err := NewJobStore(dir)
	if err != nil {
		t.Fatalf("NewJobStore() error = %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestNewJobStore_DefaultDir(t *testing.T) {
	store, err := NewJobStore("")
	if err != nil {
		t.Fatalf("NewJobStore(\"\") error = %v", err)
	}
	if store.dir != "/tmp/report-exports" {
		t.Errorf("dir = %q, want /tmp/report-exports", store.dir)
	}
}

func TestJobStore_CreateGet(t *testing.T) {
	store, err := NewJobStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	id := store.Create("csv", "personal")
	job, ok := store.Get(id)
	if !ok {
		t.Fatal("job not found")
	}
	if job.Status != JobPending {
		t.Errorf("status = %q, want pending", job.Status)
	}
	if job.Format != "csv" || job.Type != "personal" {
		t.Errorf("unexpected job: %+v", job)
	}
}

func TestJobStore_MarkDoneFailed(t *testing.T) {
	store, err := NewJobStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	id := store.Create("pdf", "department")
	store.MarkDone(id)
	job, _ := store.Get(id)
	if job.Status != JobDone {
		t.Errorf("status = %q, want done", job.Status)
	}

	id2 := store.Create("csv", "events")
	store.MarkFailed(id2, "internal detail")
	job2, _ := store.Get(id2)
	if job2.Status != JobFailed {
		t.Errorf("status = %q, want failed", job2.Status)
	}
	if job2.Error != "internal detail" {
		t.Errorf("error = %q", job2.Error)
	}
}

func TestJobStore_OpenResult(t *testing.T) {
	dir := t.TempDir()
	store, err := NewJobStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	id := store.Create("csv", "personal")
	store.MarkDone(id)

	path := store.FilePath(id, ".csv")
	if err := os.WriteFile(path, []byte("col1,col2\na,b"), 0o644); err != nil {
		t.Fatal(err)
	}

	f, name, err := store.OpenResult(id)
	if err != nil {
		t.Fatalf("OpenResult() error = %v", err)
	}
	defer f.Close()
	if name == "" {
		t.Error("expected non-empty filename")
	}
}

func TestJobStore_OpenResult_NotReady(t *testing.T) {
	store, err := NewJobStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	id := store.Create("csv", "personal")
	_, _, err = store.OpenResult(id)
	if err == nil {
		t.Error("expected error for pending job")
	}
}

func TestJobStore_Get_NotFound(t *testing.T) {
	store, err := NewJobStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, ok := store.Get("nonexistent")
	if ok {
		t.Error("expected not found")
	}
}
