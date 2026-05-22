package model

import "time"

type Direction string

const (
	DirectionIN  Direction = "IN"
	DirectionOUT Direction = "OUT"
)

type Decision string

const (
	DecisionAllow Decision = "ALLOW"
	DecisionDeny  Decision = "DENY"
)

type DenyReason string

const (
	ReasonAntiPassback     DenyReason = "ANTI_PASSBACK"
	ReasonPermissionDenied DenyReason = "PERMISSION_DENIED"
	ReasonCardNotFound     DenyReason = "CARD_NOT_FOUND"
)

type PassbackState string

const (
	PassbackNone PassbackState = "NONE"
	PassbackIN   PassbackState = "IN"
	PassbackOUT  PassbackState = "OUT"
)

type SwipeRequest struct {
	UserID    string    `json:"userId" binding:"required,uuid"`
	DoorID    string    `json:"doorId" binding:"required,uuid"`
	Direction Direction `json:"direction" binding:"required,oneof=IN OUT"`
	CardUID   string    `json:"cardUid"`
	Timestamp time.Time `json:"timestamp"`
}

type SwipeResponse struct {
	Decision Decision    `json:"decision"`
	Reason   *DenyReason `json:"reason"`
	EventID  string      `json:"eventId"`
	Degraded bool        `json:"degraded,omitempty"` // true when Redis down and DB fallback used
}

type InOutEvent struct {
	EventID    string     `json:"eventId"`
	EmployeeID string     `json:"employeeId"`
	DoorID     string     `json:"doorId"`
	Direction  Direction  `json:"direction"`
	EventTime  time.Time  `json:"eventTime"`
	Status     Decision   `json:"status"`
	Reason     *DenyReason `json:"reason"`
	CardUID    string     `json:"cardUid"`
	SourceIP   string     `json:"sourceIp"`
}

type EmployeeStateResponse struct {
	UserID string        `json:"userId"`
	State  PassbackState `json:"state"`
}

type DoorStatusResponse struct {
	DoorID string `json:"doorId"`
	Status string `json:"status"`
}
