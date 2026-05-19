package handler

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/tsmc/access-api/internal/cache"
	"github.com/tsmc/access-api/internal/model"
	"github.com/tsmc/access-api/internal/queue"
	"github.com/tsmc/access-api/internal/service"
)

var (
	swipeTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "access_api_swipe_total",
		Help: "Total swipe requests",
	}, []string{"decision", "reason"})

	swipeLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "access_api_swipe_duration_ms",
		Help:    "Swipe handler latency in milliseconds",
		Buckets: prometheus.ExponentialBuckets(1, 2, 12),
	})
)

type AccessHandler struct {
	decisions *service.AccessDecisionService
	cache     *cache.RedisCache
	publisher queue.EventPublisher
}

func NewAccessHandler(decisions *service.AccessDecisionService, c *cache.RedisCache, pub queue.EventPublisher) *AccessHandler {
	return &AccessHandler{decisions: decisions, cache: c, publisher: pub}
}

func (h *AccessHandler) Swipe(c *gin.Context) {
	start := time.Now()
	defer func() {
		swipeLatency.Observe(float64(time.Since(start).Milliseconds()))
	}()

	var req model.SwipeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Timestamp.IsZero() {
		req.Timestamp = time.Now().UTC()
	}

	eventID := uuid.New().String()
	result, err := h.decisions.Evaluate(c.Request.Context(), req.UserID, req.Direction)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "cache unavailable"})
		return
	}

	reasonLabel := "none"
	if result.Reason != nil {
		reasonLabel = string(*result.Reason)
	}
	swipeTotal.WithLabelValues(string(result.Decision), reasonLabel).Inc()

	event := model.InOutEvent{
		EventID:    eventID,
		EmployeeID: req.UserID,
		DoorID:     req.DoorID,
		Direction:  req.Direction,
		EventTime:  req.Timestamp,
		Status:     result.Decision,
		Reason:     result.Reason,
		CardUID:    req.CardUID,
		SourceIP:   c.ClientIP(),
	}
	go func() {
		if err := h.publisher.Publish(c.Request.Context(), event); err != nil {
			log.Printf("async publish eventId=%s: %v", eventID, err)
		}
	}()

	c.JSON(http.StatusOK, model.SwipeResponse{
		Decision: result.Decision,
		Reason:   result.Reason,
		EventID:  eventID,
	})
}

func (h *AccessHandler) EmployeeState(c *gin.Context) {
	userID := c.Param("userId")
	if _, err := uuid.Parse(userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid userId"})
		return
	}
	state, err := h.cache.GetPassback(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "cache unavailable"})
		return
	}
	c.JSON(http.StatusOK, model.EmployeeStateResponse{UserID: userID, State: state})
}

func (h *AccessHandler) DoorStatus(c *gin.Context) {
	doorID := c.Param("doorId")
	if _, err := uuid.Parse(doorID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid doorId"})
		return
	}
	status, err := h.cache.GetDoorStatus(c.Request.Context(), doorID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "cache unavailable"})
		return
	}
	c.JSON(http.StatusOK, model.DoorStatusResponse{DoorID: doorID, Status: status})
}
