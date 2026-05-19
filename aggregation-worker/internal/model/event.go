package model

import "time"

type InOutEvent struct {
	EventID    string     `json:"eventId"`
	EmployeeID string     `json:"employeeId"`
	DoorID     string     `json:"doorId"`
	Direction  string     `json:"direction"`
	EventTime  time.Time  `json:"eventTime"`
	Status     string     `json:"status"`
	Reason     *string    `json:"reason"`
	CardUID    string     `json:"cardUid"`
	SourceIP   string     `json:"sourceIp"`
}
