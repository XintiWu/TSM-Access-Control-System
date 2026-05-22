package auth

import "strings"

// ReportRole mirrors hierarchical report access in the design spec.
type ReportRole string

const (
	RoleCEO         ReportRole = "CEO"
	RoleCFO         ReportRole = "CFO"
	RoleVP          ReportRole = "VP"
	RoleDirector    ReportRole = "DIRECTOR"
	RoleTeamManager ReportRole = "TEAM_MANAGER"
	RoleEmployee    ReportRole = "EMPLOYEE"
)

// ParseRole normalizes stored role strings.
func ParseRole(s string) ReportRole {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "CEO":
		return RoleCEO
	case "CFO":
		return RoleCFO
	case "VP":
		return RoleVP
	case "DIRECTOR":
		return RoleDirector
	case "TEAM_MANAGER", "MANAGER":
		return RoleTeamManager
	default:
		return RoleEmployee
	}
}

// CanViewDepartmentReports returns whether the role may call department/export analytics APIs.
func (r ReportRole) CanViewDepartmentReports() bool {
	switch r {
	case RoleCEO, RoleCFO, RoleVP, RoleDirector, RoleTeamManager:
		return true
	default:
		return false
	}
}

// CanViewFullAudit returns whether the role may query org-wide audit logs (subtree).
func (r ReportRole) CanViewFullAudit() bool {
	return r.CanViewDepartmentReports()
}

// CanViewPersonalOnly is true for individual contributors.
func (r ReportRole) CanViewPersonalOnly() bool {
	return r == RoleEmployee
}

// IsExecutive returns CEO/CFO (company-wide scope at root org unit).
func (r ReportRole) IsExecutive() bool {
	return r == RoleCEO || r == RoleCFO
}
