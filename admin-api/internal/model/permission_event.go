package model

import "time"

type PermissionAction string

const (
	ActionBan   PermissionAction = "BAN"
	ActionUnban PermissionAction = "UNBAN"
)

type PermissionEvent struct {
	UserID    string           `json:"userId"`
	Action    PermissionAction `json:"action"`
	EventTime time.Time        `json:"eventTime"`
}

type PermissionResponse struct {
	UserID string           `json:"userId"`
	Action PermissionAction `json:"action"`
	Status string           `json:"status"`
}
