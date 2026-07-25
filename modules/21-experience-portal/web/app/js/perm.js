// What this person may see and do.
//
// Authority is derived from the operating model, not from a parallel
// access-control list: a role grants a class of permission, and the seats the
// person holds in a department org chart decide *where* it applies. That is
// what GET /svc/templates/me/assignments returns, and it is the only source
// this module consults besides the roles in the token.
//
// Nothing here is a security boundary — every service enforces its own
// authorization. This decides what the UI offers, so that acting under the
// wrong authority is not something you do by accident.
import { session, get } from "./api.js";

// Role → permissions. Kept deliberately small; add on evidence, not in
// anticipation.
const ROLE_PERMISSIONS = {
  platform_admin: [
    "department.read", "department.write", "department.deploy",
    "request.read.all", "request.create", "approval.read", "approval.respond",
    "kpi.read", "org.write", "people.read", "platform.admin",
  ],
  executive: [
    "department.read", "request.read.all", "approval.read", "kpi.read", "people.read",
  ],
  department_head: [
    "department.read", "department.write",
    "request.read.department", "request.create",
    "approval.read", "approval.respond", "kpi.read", "people.read",
  ],
  supervisor: [
    "department.read", "request.read.department",
    "approval.read", "approval.respond",
  ],
  employee: ["request.create", "request.read.own"],
};

// The state this module derives once per sign-in.
export const authz = {
  permissions: new Set(),
  assignments: [],
  loaded: false,
};

// loadAuthz resolves the caller's seats and permissions. Called after login and
// after a session is restored.
export async function loadAuthz() {
  authz.permissions = new Set();
  authz.assignments = [];
  authz.loaded = false;

  const roles = session.roles && session.roles.length ? session.roles : (session.role ? [session.role] : []);
  for (const role of roles) {
    for (const p of ROLE_PERMISSIONS[role] || []) authz.permissions.add(p);
  }

  // Seats decide scope, and can grant authority a bare role does not: holding
  // a seat with "decide" rights or an approval gate makes someone an approver
  // for that department even if their role list is thin.
  try {
    const r = await get("/svc/templates/me/assignments");
    if (r.ok) {
      authz.assignments = (r.data && r.data.data) || [];
      for (const a of authz.assignments) {
        authz.permissions.add("department.read");
        authz.permissions.add("request.read.department");
        authz.permissions.add("kpi.read");
        if (a.is_department_root) authz.permissions.add("department.write");
        const decides = (a.decision_rights || []).some(d => d.authority === "decide");
        if (decides || (a.approval_gate_refs || []).length) {
          authz.permissions.add("approval.read");
          authz.permissions.add("approval.respond");
        }
      }
    }
  } catch (_) {
    // An unreachable department service must not silently widen or narrow
    // authority — role-derived permissions stand, seats simply stay unknown.
  }

  // Everybody who can sign in can ask their departments for something.
  authz.permissions.add("request.create");
  authz.loaded = true;
  return authz;
}

export function can(permission) {
  if (!permission) return true;
  return authz.permissions.has(permission);
}

export function canAny(permissions) {
  if (!permissions || permissions.length === 0) return true;
  return permissions.some(can);
}

// departmentIds returns the departments this person holds a seat in. Empty
// means "no seat" — for a platform admin or executive that is normal and they
// see everything; for anyone else it is the honest unbound state.
export function departmentIds() {
  return [...new Set(authz.assignments.map(a => a.department_id))];
}

export function isUnbound() {
  return authz.assignments.length === 0;
}

// A short description of the authority in force, for the shell.
export function authorityLabel() {
  const role = (session.role || "user").replace(/_/g, " ");
  if (authz.assignments.length === 0) return role;
  if (authz.assignments.length === 1) {
    const a = authz.assignments[0];
    return `${role} · ${a.title}`;
  }
  return `${role} · ${authz.assignments.length} seats`;
}
