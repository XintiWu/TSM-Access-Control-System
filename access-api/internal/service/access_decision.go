package service

import (
	"context"

	"github.com/tsmc/access-api/internal/model"
)

type CacheStore interface {
	IsDenied(ctx context.Context, userID string) (bool, error)
	GetPassback(ctx context.Context, userID string) (model.PassbackState, error)
	SetPassback(ctx context.Context, userID string, state model.PassbackState) error
}

type DecisionResult struct {
	Decision model.Decision
	Reason   *model.DenyReason
}

type AccessDecisionService struct {
	cache CacheStore
}

func NewAccessDecisionService(cache CacheStore) *AccessDecisionService {
	return &AccessDecisionService{cache: cache}
}

func (s *AccessDecisionService) Evaluate(ctx context.Context, userID string, direction model.Direction) (DecisionResult, error) {
	denied, err := s.cache.IsDenied(ctx, userID)
	if err != nil {
		return DecisionResult{}, err
	}
	if denied {
		r := model.ReasonPermissionDenied
		return DecisionResult{Decision: model.DecisionDeny, Reason: &r}, nil
	}

	state, err := s.cache.GetPassback(ctx, userID)
	if err != nil {
		return DecisionResult{}, err
	}

	if violation := checkAntiPassback(state, direction); violation {
		r := model.ReasonAntiPassback
		return DecisionResult{Decision: model.DecisionDeny, Reason: &r}, nil
	}

	if err := s.cache.SetPassback(ctx, userID, model.PassbackState(direction)); err != nil {
		return DecisionResult{}, err
	}

	return DecisionResult{Decision: model.DecisionAllow, Reason: nil}, nil
}

func checkAntiPassback(state model.PassbackState, direction model.Direction) bool {
	switch {
	case direction == model.DirectionIN && state == model.PassbackIN:
		return true
	case direction == model.DirectionOUT && state == model.PassbackOUT:
		return true
	default:
		return false
	}
}
