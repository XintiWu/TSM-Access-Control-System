package queue

import (
	"context"
	"testing"
	"time"

	"github.com/tsmc/access-api/internal/model"
)

func TestFileOutboxAppendAndReplay(t *testing.T) {
	ob, err := NewFileOutbox(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ev := model.InOutEvent{
		EventID:    "e1",
		EmployeeID: "22222222-2222-2222-2222-222222222222",
		DoorID:     "11111111-1111-1111-1111-111111111111",
		Direction:  model.DirectionIN,
		EventTime:  time.Now().UTC(),
		Status:     model.DecisionAllow,
	}
	if err := ob.Append(ev); err != nil {
		t.Fatal(err)
	}
	var published int
	n, err := ob.Replay(context.Background(), func(_ context.Context, e model.InOutEvent) error {
		if e.EventID != "e1" {
			t.Fatalf("unexpected event %s", e.EventID)
		}
		published++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || published != 1 {
		t.Fatalf("expected 1 replayed, got n=%d published=%d", n, published)
	}
}
