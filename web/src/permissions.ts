// What the signed-in user may do (SPEC-012). The list comes from the server (/api/v1/me), which
// holds the role matrix; the screens only hide what the server would refuse anyway. Without a
// server (the sample data), everything is shown, as before.
import { createContext, useContext } from 'react'

export type Can = (permission: string) => boolean

export const allowAll: Can = () => true

export function canFrom(permissions: string[]): Can {
  return (permission) => permissions.includes(permission)
}

export const PermissionContext = createContext<Can>(allowAll)

export function useCan(): Can {
  return useContext(PermissionContext)
}

// The four roles in the order the Admin tab lists them, with the name shown for each.
export const roleNames: Record<string, string> = { partner: 'Partner', manager: 'Manager', staff: 'Staff', read_only: 'Read-only' }

// One sentence per role for the role picker of the Admin tab. It restates roleMatrix below.
export const roleSummaries: Record<string, string> = {
  partner: 'Everything, including approving and disabling rules and this Admin tab.',
  manager: 'Uploads and runs checks, decides and deletes findings, drafts rules and exports. Cannot approve or disable rules or open Admin.',
  staff: 'Uploads and runs checks, decides findings, classifies the Worklist and drafts rules. Cannot delete findings or export.',
  read_only: 'Views clients, findings and the Worklist. Cannot change anything.',
}

// roleMatrix explains the twelve permissions to a Partner: which role holds each. It is a copy for
// reading only, written from the server's matrix (technical spec §B12.1, internal/auth/roles.go);
// the screens still take what a user may do from /api/v1/me, and the server enforces it.
export const roleMatrix: { permission: string; action: string; roles: string[] }[] = [
  { permission: 'view', action: 'View clients, findings and the Worklist', roles: ['partner', 'manager', 'staff', 'read_only'] },
  { permission: 'run', action: 'Upload files and run checks', roles: ['partner', 'manager', 'staff'] },
  { permission: 'decide', action: 'Decide findings and classify the Worklist', roles: ['partner', 'manager', 'staff'] },
  { permission: 'draft_rules', action: 'Draft rules and run rule tests', roles: ['partner', 'manager', 'staff'] },
  { permission: 'delete_findings', action: 'Delete findings', roles: ['partner', 'manager'] },
  { permission: 'export', action: 'Export results', roles: ['partner', 'manager'] },
  { permission: 'disable_rules', action: 'Disable rules', roles: ['partner'] },
  { permission: 'disable_clients', action: 'Disable a client or enable it again', roles: ['partner'] },
  { permission: 'activate_rules', action: 'Approve and activate rules', roles: ['partner'] },
  { permission: 'manage_users', action: 'Admin tab: users, sign-in security, the dollar rate, audit log', roles: ['partner'] },
  { permission: 'manage_formats', action: 'Formats: switch formats and parameters off or on, add parameters', roles: ['partner'] },
  { permission: 'manage_documents', action: 'Document Master: add, edit, switch and delete document types and their record structure', roles: ['partner'] },
]
