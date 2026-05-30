package queue

import (
	"context"
	"testing"

	"github.com/tsmc/admin-api/internal/model"
)

func TestNoopPublisher(t *testing.T) {
	var pub PermissionPublisher = NoopPublisher{}
	err := pub.Publish(context.Background(), model.PermissionEvent{UserID: "user-1"})
	if err != nil {
		t.Fatalf("NoopPublisher.Publish failed: %v", err)
	}
	err = pub.Close()
	if err != nil {
		t.Fatalf("NoopPublisher.Close failed: %v", err)
	}
}
