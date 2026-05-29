package service

import (
	"context"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/tsmc/access-api/internal/model"
)

var (
	fallbackTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "access_api_fallback_total",
		Help: "Number of swipe decisions that fell back to DB because Redis was unavailable",
	})
)

// CacheStore abstracts Redis operations for the decision service.
type CacheStore interface {
	IsDenied(ctx context.Context, userID string) (bool, error)
	GetPassback(ctx context.Context, userID string) (model.PassbackState, error)
	SetPassback(ctx context.Context, userID string, state model.PassbackState) error
	LookupCard(ctx context.Context, cardUID string) (string, error)
	SetCardMapping(ctx context.Context, cardUID, userID string) error
}

// DBStore abstracts ClickHouse employee queries used only when Redis is down (fallback path).
type DBStore interface {
	IsActive(ctx context.Context, userID string) (bool, error)
	LookupCardUID(ctx context.Context, cardUID string) (string, error)
	GetLastPassbackState(ctx context.Context, userID string) (string, error)
}

type DecisionResult struct {
	Decision model.Decision
	Reason   *model.DenyReason
	Degraded bool // true when decision was made via DB fallback
}

type AccessDecisionService struct {
	cache CacheStore
	db    DBStore // nil if DB fallback is not configured
}

func NewAccessDecisionService(cache CacheStore) *AccessDecisionService {
	return &AccessDecisionService{cache: cache}
}

// SetDBFallback enables the DB fallback path for when Redis is unavailable.
func (s *AccessDecisionService) SetDBFallback(db DBStore) {
	s.db = db
}

// Evaluate performs the access decision.
// Order: card validation → permission check → anti-passback → ALLOW.
func (s *AccessDecisionService) Evaluate(ctx context.Context, userID, cardUID string, direction model.Direction) (DecisionResult, error) {
	// Step 1: Card validation (via Redis cache with DB fallback and read-through caching)
	if cardUID != "" {
		mappedUser, err := s.cache.LookupCard(ctx, cardUID)
		if err != nil || mappedUser == "" {
			// Redis error or cache miss — try DB
			if s.db != nil {
				dbUser, dbErr := s.db.LookupCardUID(ctx, cardUID)
				if dbErr != nil {
					slog.Warn("card lookup DB fallback failed", "error", dbErr)
					// If Redis returned error (not just miss), we can fail-open or fail-safe.
					// But if it's a cache miss, DB is the source of truth.
				} else if dbUser != "" {
					mappedUser = dbUser
					// Populate Redis cache so subsequent hits are fast
					_ = s.cache.SetCardMapping(ctx, cardUID, dbUser)
				}
			}
		}

		if mappedUser == "" || mappedUser != userID {
			r := model.ReasonCardNotFound
			return DecisionResult{Decision: model.DecisionDeny, Reason: &r}, nil
		}
	}

	// Step 2: Permission denied check
	denied, err := s.cache.IsDenied(ctx, userID)
	if err != nil {
		return s.evaluateFallback(ctx, userID, direction)
	}
	if denied {
		r := model.ReasonPermissionDenied
		return DecisionResult{Decision: model.DecisionDeny, Reason: &r}, nil
	}

	// Step 3: Anti-passback
	state, err := s.cache.GetPassback(ctx, userID)
	if err != nil {
		return s.evaluateFallback(ctx, userID, direction)
	}

	if violation := checkAntiPassback(state, direction); violation {
		r := model.ReasonAntiPassback
		return DecisionResult{Decision: model.DecisionDeny, Reason: &r}, nil
	}

	// Step 4: Update passback state
	if err := s.cache.SetPassback(ctx, userID, model.PassbackState(direction)); err != nil {
		return s.evaluateFallback(ctx, userID, direction)
	}

	return DecisionResult{Decision: model.DecisionAllow, Reason: nil}, nil
}

// evaluateFallback makes a degraded decision via DB when Redis is down.
// Anti-passback uses the last ALLOW event from ClickHouse when available.
func (s *AccessDecisionService) evaluateFallback(ctx context.Context, userID string, direction model.Direction) (DecisionResult, error) {
	if s.db == nil {
		return DecisionResult{}, ErrCacheUnavailable
	}

	fallbackTotal.Inc()
	slog.Warn("redis unavailable, falling back to DB (degraded)", "userId", userID)

	active, err := s.db.IsActive(ctx, userID)
	if err != nil {
		return DecisionResult{}, err
	}
	if !active {
		r := model.ReasonPermissionDenied
		return DecisionResult{Decision: model.DecisionDeny, Reason: &r, Degraded: true}, nil
	}

	state := model.PassbackNone
	if lastDir, err := s.db.GetLastPassbackState(ctx, userID); err == nil && lastDir != "" {
		state = model.PassbackState(lastDir)
	}
	if violation := checkAntiPassback(state, direction); violation {
		r := model.ReasonAntiPassback
		return DecisionResult{Decision: model.DecisionDeny, Reason: &r, Degraded: true}, nil
	}

	return DecisionResult{Decision: model.DecisionAllow, Reason: nil, Degraded: true}, nil
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
