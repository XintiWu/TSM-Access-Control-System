package auth

import "testing"

func TestParseRole(t *testing.T) {
	tests := []struct {
		input string
		want  ReportRole
	}{
		{"CEO", RoleCEO},
		{"ceo", RoleCEO},
		{"CFO", RoleCFO},
		{"VP", RoleVP},
		{"DIRECTOR", RoleDirector},
		{"TEAM_MANAGER", RoleTeamManager},
		{"MANAGER", RoleTeamManager},
		{"employee", RoleEmployee},
		{"", RoleEmployee},
		{" unknown ", RoleEmployee},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ParseRole(tt.input); got != tt.want {
				t.Errorf("ParseRole(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCanViewDepartmentReports(t *testing.T) {
	canView := []ReportRole{RoleCEO, RoleCFO, RoleVP, RoleDirector, RoleTeamManager}
	for _, r := range canView {
		if !r.CanViewDepartmentReports() {
			t.Errorf("%q should view department reports", r)
		}
	}
	if RoleEmployee.CanViewDepartmentReports() {
		t.Error("EMPLOYEE should not view department reports")
	}
}

func TestCanViewFullAudit(t *testing.T) {
	if !RoleDirector.CanViewFullAudit() {
		t.Error("DIRECTOR should view full audit")
	}
	if RoleEmployee.CanViewFullAudit() {
		t.Error("EMPLOYEE should not view full audit")
	}
}

func TestCanViewPersonalOnly(t *testing.T) {
	if !RoleEmployee.CanViewPersonalOnly() {
		t.Error("EMPLOYEE should be personal-only")
	}
	if RoleCEO.CanViewPersonalOnly() {
		t.Error("CEO should not be personal-only")
	}
}

func TestIsExecutive(t *testing.T) {
	if !RoleCEO.IsExecutive() || !RoleCFO.IsExecutive() {
		t.Error("CEO/CFO should be executive")
	}
	if RoleVP.IsExecutive() {
		t.Error("VP should not be executive")
	}
}
