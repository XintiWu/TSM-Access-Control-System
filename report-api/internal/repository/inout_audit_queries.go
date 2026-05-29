package repository

import (
	"fmt"

	"github.com/google/uuid"
)

const auditDateRange = "event_time >= toDateTime64(?, 3, 'UTC') AND event_time < addDays(toDateTime64(?, 3, 'UTC'), 1)"

type auditQuerySet struct {
	count     string
	selectSQL string
}

// auditQueries maps a filter bitmask to fully static SQL (no runtime string formatting).
// Bit 1: employee, bit 2: door, bit 4: status, bit 8: org units.
var auditQueries = map[uint8]auditQuerySet{
	0b0000: {
		count:  "SELECT COUNT(*) FROM inout_events WHERE " + auditDateRange,
		selectSQL: auditSelectCols + " WHERE " + auditDateRange + auditOrderLimit,
	},
	0b0001: {
		count:  "SELECT COUNT(*) FROM inout_events WHERE " + auditDateRange + " AND employee_id = ?",
		selectSQL: auditSelectCols + " WHERE " + auditDateRange + " AND employee_id = ?" + auditOrderLimit,
	},
	0b0010: {
		count:  "SELECT COUNT(*) FROM inout_events WHERE " + auditDateRange + " AND door_id = ?",
		selectSQL: auditSelectCols + " WHERE " + auditDateRange + " AND door_id = ?" + auditOrderLimit,
	},
	0b0011: {
		count:  "SELECT COUNT(*) FROM inout_events WHERE " + auditDateRange + " AND employee_id = ? AND door_id = ?",
		selectSQL: auditSelectCols + " WHERE " + auditDateRange + " AND employee_id = ? AND door_id = ?" + auditOrderLimit,
	},
	0b0100: {
		count:  "SELECT COUNT(*) FROM inout_events WHERE " + auditDateRange + " AND status = ?",
		selectSQL: auditSelectCols + " WHERE " + auditDateRange + " AND status = ?" + auditOrderLimit,
	},
	0b0101: {
		count:  "SELECT COUNT(*) FROM inout_events WHERE " + auditDateRange + " AND employee_id = ? AND status = ?",
		selectSQL: auditSelectCols + " WHERE " + auditDateRange + " AND employee_id = ? AND status = ?" + auditOrderLimit,
	},
	0b0110: {
		count:  "SELECT COUNT(*) FROM inout_events WHERE " + auditDateRange + " AND door_id = ? AND status = ?",
		selectSQL: auditSelectCols + " WHERE " + auditDateRange + " AND door_id = ? AND status = ?" + auditOrderLimit,
	},
	0b0111: {
		count:  "SELECT COUNT(*) FROM inout_events WHERE " + auditDateRange + " AND employee_id = ? AND door_id = ? AND status = ?",
		selectSQL: auditSelectCols + " WHERE " + auditDateRange + " AND employee_id = ? AND door_id = ? AND status = ?" + auditOrderLimit,
	},
	0b1000: {
		count:  "SELECT COUNT(*) FROM inout_events WHERE " + auditDateRange + " AND org_unit_id IN (?)",
		selectSQL: auditSelectCols + " WHERE " + auditDateRange + " AND org_unit_id IN (?)" + auditOrderLimit,
	},
	0b1001: {
		count:  "SELECT COUNT(*) FROM inout_events WHERE " + auditDateRange + " AND employee_id = ? AND org_unit_id IN (?)",
		selectSQL: auditSelectCols + " WHERE " + auditDateRange + " AND employee_id = ? AND org_unit_id IN (?)" + auditOrderLimit,
	},
	0b1010: {
		count:  "SELECT COUNT(*) FROM inout_events WHERE " + auditDateRange + " AND door_id = ? AND org_unit_id IN (?)",
		selectSQL: auditSelectCols + " WHERE " + auditDateRange + " AND door_id = ? AND org_unit_id IN (?)" + auditOrderLimit,
	},
	0b1011: {
		count:  "SELECT COUNT(*) FROM inout_events WHERE " + auditDateRange + " AND employee_id = ? AND door_id = ? AND org_unit_id IN (?)",
		selectSQL: auditSelectCols + " WHERE " + auditDateRange + " AND employee_id = ? AND door_id = ? AND org_unit_id IN (?)" + auditOrderLimit,
	},
	0b1100: {
		count:  "SELECT COUNT(*) FROM inout_events WHERE " + auditDateRange + " AND status = ? AND org_unit_id IN (?)",
		selectSQL: auditSelectCols + " WHERE " + auditDateRange + " AND status = ? AND org_unit_id IN (?)" + auditOrderLimit,
	},
	0b1101: {
		count:  "SELECT COUNT(*) FROM inout_events WHERE " + auditDateRange + " AND employee_id = ? AND status = ? AND org_unit_id IN (?)",
		selectSQL: auditSelectCols + " WHERE " + auditDateRange + " AND employee_id = ? AND status = ? AND org_unit_id IN (?)" + auditOrderLimit,
	},
	0b1110: {
		count:  "SELECT COUNT(*) FROM inout_events WHERE " + auditDateRange + " AND door_id = ? AND status = ? AND org_unit_id IN (?)",
		selectSQL: auditSelectCols + " WHERE " + auditDateRange + " AND door_id = ? AND status = ? AND org_unit_id IN (?)" + auditOrderLimit,
	},
	0b1111: {
		count:  "SELECT COUNT(*) FROM inout_events WHERE " + auditDateRange + " AND employee_id = ? AND door_id = ? AND status = ? AND org_unit_id IN (?)",
		selectSQL: auditSelectCols + " WHERE " + auditDateRange + " AND employee_id = ? AND door_id = ? AND status = ? AND org_unit_id IN (?)" + auditOrderLimit,
	},
}

const auditSelectCols = `
		SELECT id, employee_id, door_id, direction, event_time,
		       status, reason, COALESCE(card_uid,''), COALESCE(source_ip,'')
		FROM inout_events`

const auditOrderLimit = `
		ORDER BY event_time DESC
		LIMIT ? OFFSET ?`

func buildAuditQuery(f AuditFilter) (auditQuerySet, []interface{}, error) {
	var mask uint8
	args := []interface{}{f.StartDate, f.EndDate}

	if f.EmployeeID != "" {
		empUUID, err := uuid.Parse(f.EmployeeID)
		if err != nil {
			return auditQuerySet{}, nil, fmt.Errorf("invalid employee id: %w", err)
		}
		mask |= 0b0001
		args = append(args, empUUID)
	}
	if f.DoorID != "" {
		doorUUID, err := uuid.Parse(f.DoorID)
		if err != nil {
			return auditQuerySet{}, nil, fmt.Errorf("invalid door id: %w", err)
		}
		mask |= 0b0010
		args = append(args, doorUUID)
	}
	if f.Status != "" {
		if f.Status != "ALLOW" && f.Status != "DENY" {
			return auditQuerySet{}, nil, fmt.Errorf("invalid status: %s", f.Status)
		}
		mask |= 0b0100
		args = append(args, f.Status)
	}
	if len(f.OrgUnitIDs) > 0 {
		var orgUUIDs []uuid.UUID
		for _, id := range f.OrgUnitIDs {
			u, err := uuid.Parse(id)
			if err == nil {
				orgUUIDs = append(orgUUIDs, u)
			}
		}
		mask |= 0b1000
		args = append(args, orgUUIDs)
	}

	q, ok := auditQueries[mask]
	if !ok {
		return auditQuerySet{}, nil, fmt.Errorf("unsupported audit filter combination")
	}
	return q, args, nil
}
