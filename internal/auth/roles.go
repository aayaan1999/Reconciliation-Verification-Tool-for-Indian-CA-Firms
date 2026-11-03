package auth

import (
	"fmt"
	"slices"
)

// A Role is what a user may do in the tool (requirements §3, technical spec §B12.1).
type Role string

const (
	RolePartner  Role = "partner"
	RoleManager  Role = "manager"
	RoleStaff    Role = "staff"
	RoleReadOnly Role = "read_only"
)

// Roles lists every role, in the order the Admin tab shows them.
var Roles = []Role{RolePartner, RoleManager, RoleStaff, RoleReadOnly}

// ParseRole reads a role name as stored and sent by the screens.
func ParseRole(s string) (Role, error) {
	r := Role(s)
	if !slices.Contains(Roles, r) {
		return "", fmt.Errorf("the role must be one of partner, manager, staff, read_only")
	}
	return r, nil
}

// A Permission is what a handler needs. Role permissions are granted by the matrix below.
// Two more values say who may call a handler at all: PermOpen (no sign-in: the login and the
// first-start setup) and PermSignedIn (any signed-in user, also while a password change is
// pending). Every handler declares one of them; a handler with no declaration is refused.
type Permission string

const (
	PermOpen     Permission = "open"
	PermSignedIn Permission = "signed_in"

	PermView           Permission = "view"            // view clients, findings and the Worklist
	PermRun            Permission = "run"             // upload files, run reconciliation, enter client details, teach layouts
	PermDecide         Permission = "decide"          // decide findings, classify the Worklist
	PermDeleteFindings Permission = "delete_findings" // delete findings
	PermDraftRules     Permission = "draft_rules"     // create and edit draft rules, run rule tests
	PermActivateRules  Permission = "activate_rules"  // activate, enable or approve rules; switch versions; law-table rows
	PermDisableRules   Permission = "disable_rules"   // disable a rule
	PermDisableClients Permission = "disable_clients" // disable a client or enable it again
	PermExport         Permission = "export"          // export results
	PermManageUsers    Permission = "manage_users"    // the Admin tab: users, lockout, audit log
	PermManageFormats  Permission = "manage_formats"  // the Formats screen: switch formats and parameters off or on, add parameters (SPEC-016)
)

// rolePermissions is the role matrix of technical spec §B12.1, written once. The handlers,
// the UI (through /api/v1/me) and the tests all read this table; nothing keeps a copy.
var rolePermissions = map[Role][]Permission{
	RolePartner: {
		PermView, PermRun, PermDecide, PermDeleteFindings, PermDraftRules,
		PermActivateRules, PermDisableRules, PermDisableClients, PermExport, PermManageUsers, PermManageFormats,
	},
	RoleManager:  {PermView, PermRun, PermDecide, PermDeleteFindings, PermDraftRules, PermExport},
	RoleStaff:    {PermView, PermRun, PermDecide, PermDraftRules},
	RoleReadOnly: {PermView},
}

// Allowed reports whether a role holds a role permission. It is false for PermOpen and
// PermSignedIn, which the server checks itself, and for any permission not in the matrix.
func Allowed(role Role, p Permission) bool {
	return slices.Contains(rolePermissions[role], p)
}

// PermissionsOf returns the role permissions a role holds, in a fixed order.
func PermissionsOf(role Role) []Permission {
	out := slices.Clone(rolePermissions[role])
	slices.Sort(out)
	return out
}

// AllRolePermissions lists every permission the matrix can grant, in a fixed order.
func AllRolePermissions() []Permission {
	seen := map[Permission]bool{}
	var out []Permission
	for _, r := range Roles {
		for _, p := range rolePermissions[r] {
			if !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	slices.Sort(out)
	return out
}
