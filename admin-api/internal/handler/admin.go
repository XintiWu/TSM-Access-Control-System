package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tsmc/admin-api/internal/model"
	"github.com/tsmc/admin-api/internal/queue"
	"github.com/tsmc/admin-api/internal/repository"
)

type EmployeeRepository interface {
	Exists(ctx context.Context, userID string) (bool, error)
	SetActive(ctx context.Context, userID string, active bool) error
}

type AdminHandler struct {
	repo      EmployeeRepository
	publisher queue.PermissionPublisher
}

func NewAdminHandler(repo EmployeeRepository, pub queue.PermissionPublisher) *AdminHandler {
	return &AdminHandler{repo: repo, publisher: pub}
}

func (h *AdminHandler) Ban(c *gin.Context) {
	h.setPermission(c, false, model.ActionBan)
}

func (h *AdminHandler) Unban(c *gin.Context) {
	h.setPermission(c, true, model.ActionUnban)
}

func (h *AdminHandler) setPermission(c *gin.Context, active bool, action model.PermissionAction) {
	userID := c.Param("userId")
	if _, err := uuid.Parse(userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid userId"})
		return
	}

	ctx := c.Request.Context()
	exists, err := h.repo.Exists(ctx, userID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database unavailable"})
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "employee not found"})
		return
	}

	if err := h.repo.SetActive(ctx, userID, active); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "employee not found"})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database unavailable"})
		return
	}

	event := model.PermissionEvent{
		UserID:    userID,
		Action:    action,
		EventTime: time.Now().UTC(),
	}
	if err := h.publisher.Publish(ctx, event); err != nil {
		slog.Error("publish permission event failed", "userId", userID, "error", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "failed to publish permission event",
		})
		return
	}

	c.JSON(http.StatusOK, model.PermissionResponse{
		UserID: userID,
		Action: action,
		Status: "accepted",
	})
}
