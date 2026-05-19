package service

import (
	"context"
	"errors"
	"testing"

	"github.com/tsmc/access-api/internal/model"
)

type mockCache struct {
	denied   map[string]bool
	passback map[string]model.PassbackState
	setErr   error
}

func (m *mockCache) IsDenied(_ context.Context, userID string) (bool, error) {
	return m.denied[userID], nil
}

func (m *mockCache) GetPassback(_ context.Context, userID string) (model.PassbackState, error) {
	if s, ok := m.passback[userID]; ok {
		return s, nil
	}
	return model.PassbackNone, nil
}

func (m *mockCache) SetPassback(_ context.Context, userID string, state model.PassbackState) error {
	if m.setErr != nil {
		return m.setErr
	}
	if m.passback == nil {
		m.passback = make(map[string]model.PassbackState)
	}
	m.passback[userID] = state
	return nil
}

func TestEvaluate_PermissionDenied(t *testing.T) {
	svc := NewAccessDecisionService(&mockCache{
		denied: map[string]bool{"u1": true},
	})
	res, err := svc.Evaluate(context.Background(), "u1", model.DirectionIN)
	if err != nil {
		t.Fatal(err)
	}
	if res.Decision != model.DecisionDeny || res.Reason == nil || *res.Reason != model.ReasonPermissionDenied {
		t.Fatalf("got %+v", res)
	}
}

func TestEvaluate_AntiPassbackIN(t *testing.T) {
	svc := NewAccessDecisionService(&mockCache{
		passback: map[string]model.PassbackState{"u1": model.PassbackIN},
	})
	res, err := svc.Evaluate(context.Background(), "u1", model.DirectionIN)
	if err != nil {
		t.Fatal(err)
	}
	if res.Decision != model.DecisionDeny || *res.Reason != model.ReasonAntiPassback {
		t.Fatalf("got %+v", res)
	}
}

func TestEvaluate_AntiPassbackOUT(t *testing.T) {
	svc := NewAccessDecisionService(&mockCache{
		passback: map[string]model.PassbackState{"u1": model.PassbackOUT},
	})
	res, err := svc.Evaluate(context.Background(), "u1", model.DirectionOUT)
	if err != nil {
		t.Fatal(err)
	}
	if res.Decision != model.DecisionDeny {
		t.Fatalf("got %+v", res)
	}
}

func TestEvaluate_AllowINFromOUT(t *testing.T) {
	svc := NewAccessDecisionService(&mockCache{
		passback: map[string]model.PassbackState{"u1": model.PassbackOUT},
	})
	res, err := svc.Evaluate(context.Background(), "u1", model.DirectionIN)
	if err != nil {
		t.Fatal(err)
	}
	if res.Decision != model.DecisionAllow {
		t.Fatalf("got %+v", res)
	}
}

func TestEvaluate_AllowOUTFromIN(t *testing.T) {
	svc := NewAccessDecisionService(&mockCache{
		passback: map[string]model.PassbackState{"u1": model.PassbackIN},
	})
	res, err := svc.Evaluate(context.Background(), "u1", model.DirectionOUT)
	if err != nil {
		t.Fatal(err)
	}
	if res.Decision != model.DecisionAllow {
		t.Fatalf("got %+v", res)
	}
}

func TestEvaluate_AllowFirstIN(t *testing.T) {
	svc := NewAccessDecisionService(&mockCache{})
	res, err := svc.Evaluate(context.Background(), "u1", model.DirectionIN)
	if err != nil {
		t.Fatal(err)
	}
	if res.Decision != model.DecisionAllow {
		t.Fatalf("got %+v", res)
	}
}

func TestEvaluate_CacheError(t *testing.T) {
	svc := NewAccessDecisionService(&mockCache{setErr: errors.New("redis down")})
	_, err := svc.Evaluate(context.Background(), "u1", model.DirectionIN)
	if err == nil {
		t.Fatal("expected error")
	}
}
