package model

import (
	"testing"
	"time"
)

func TestConstants(t *testing.T) {
	if DirectionIN != "IN" || DirectionOUT != "OUT" {
		t.Error("unexpected direction constants")
	}
	if DecisionAllow != "ALLOW" || DecisionDeny != "DENY" {
		t.Error("unexpected decision constants")
	}
	if ReasonAntiPassback != "ANTI_PASSBACK" || ReasonPermissionDenied != "PERMISSION_DENIED" || ReasonCardNotFound != "CARD_NOT_FOUND" {
		t.Error("unexpected deny reason constants")
	}
	if PassbackNone != "NONE" || PassbackIN != "IN" || PassbackOUT != "OUT" {
		t.Error("unexpected passback state constants")
	}
}

func TestStructInstantiation(t *testing.T) {
	req := SwipeRequest{
		UserID:    "user-1",
		DoorID:    "door-1",
		Direction: DirectionIN,
		CardUID:   "card-abc",
		Timestamp: time.Now(),
	}
	if req.UserID != "user-1" {
		t.Errorf("UserID = %q, want user-1", req.UserID)
	}

	reason := ReasonAntiPassback
	resp := SwipeResponse{
		Decision: DecisionDeny,
		Reason:   &reason,
		EventID:  "evt-1",
		Degraded: true,
	}
	if resp.Decision != DecisionDeny || *resp.Reason != ReasonAntiPassback || !resp.Degraded {
		t.Error("unexpected SwipeResponse values")
	}

	evt := InOutEvent{
		EventID:    "evt-2",
		EmployeeID: "emp-2",
		DoorID:     "door-2",
		Direction:  DirectionOUT,
		EventTime:  time.Now(),
		Status:     DecisionAllow,
		CardUID:    "card-def",
		SourceIP:   "192.168.1.1",
	}
	if evt.EventID != "evt-2" || evt.Direction != DirectionOUT || evt.Status != DecisionAllow {
		t.Error("unexpected InOutEvent values")
	}

	emp := EmployeeStateResponse{
		UserID: "user-3",
		State:  PassbackIN,
	}
	if emp.UserID != "user-3" || emp.State != PassbackIN {
		t.Error("unexpected EmployeeStateResponse values")
	}

	door := DoorStatusResponse{
		DoorID: "door-3",
		Status: "ONLINE",
	}
	if door.DoorID != "door-3" || door.Status != "ONLINE" {
		t.Error("unexpected DoorStatusResponse values")
	}
}
