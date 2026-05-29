package queue

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/tsmc/access-api/internal/model"
)

// FileOutbox persists failed Kafka publishes as JSON lines for replay after restart.
type FileOutbox struct {
	path string
	mu   sync.Mutex
}

func NewFileOutbox(dir string) (*FileOutbox, error) {
	if dir == "" {
		dir = "/data/outbox"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &FileOutbox{path: filepath.Join(dir, "inout-events.jsonl")}, nil
}

func (o *FileOutbox) Append(event model.InOutEvent) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	f, err := os.OpenFile(o.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = f.Write(append(body, '\n'))
	return err
}

// Replay publishes pending events; rewrites file with only still-failing lines.
func (o *FileOutbox) Replay(ctx context.Context, publish func(context.Context, model.InOutEvent) error) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	data, err := os.ReadFile(o.path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	if len(data) == 0 {
		return 0, nil
	}

	var pending []model.InOutEvent
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev model.InOutEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			slog.Error("outbox skip bad line", "error", err)
			continue
		}
		pending = append(pending, ev)
	}
	if err := sc.Err(); err != nil {
		return 0, err
	}

	replayed := 0
	var stillPending []model.InOutEvent
	for _, ev := range pending {
		if err := publish(ctx, ev); err != nil {
			stillPending = append(stillPending, ev)
			continue
		}
		replayed++
	}
	return replayed, o.rewriteLocked(stillPending)
}

func (o *FileOutbox) rewriteLocked(events []model.InOutEvent) error {
	if len(events) == 0 {
		if err := os.Remove(o.path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	f, err := os.OpenFile(o.path, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, ev := range events {
		body, err := json.Marshal(ev)
		if err != nil {
			return err
		}
		if _, err := f.Write(append(body, '\n')); err != nil {
			return err
		}
	}
	return nil
}
