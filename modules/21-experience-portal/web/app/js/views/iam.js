// IAM: 3-principal identity system, RBAC, delegation, ABAC, audit (Module 02).
import { SVC, get, post, patch, del, uuid4 } from "../api.js";
import { $, esc, badge, rel, toast, rowItem } from "../ui.js";

const SVC_IAM = "/svc/iam";

const PRINCIPAL_TYPES = ["user", "service", "agent"];
const RESOURCES = ["user", "role", "service", "audit", "group", "tenant", "session", "application", "token"];
const ACTIONS = ["view", "change", "delete"];
const DELEGATION_SCOPES = ["tenant", "department", "team"];
const ABAC_RULE_TYPES = ["time", "ip", "ownership", "department", "custom"];

export async function viewIAM() {
  // Seven endpoints, settled independently. Promise.all meant one dead endpoint
  // blanked the entire console — and two of them panicked M02 on every request,
  // so this screen was never reachable at all. A partial outage is shown as a
  // partial outage: the sections that loaded render, the ones that did not say
  // so by name.
  const paths = {
    users: "/users?page_size=100",
    roles: "/roles?page_size=100",
    services: "/service-identities?page_size=50",
    agents: "/agent-identities?page_size=50",
    delegations: "/admin/delegations?page_size=20",
    abac: "/abac/policies?page_size=20",
    audit: "/audit/trails?page_size=20",
  };
  const keys = Object.keys(paths);
  const settled = await Promise.allSettled(keys.map(k => get(SVC_IAM + paths[k])));
  const data = {};
  const unavailable = [];
  keys.forEach((k, i) => {
    const s = settled[i];
    const ok = s.status === "fulfilled" && s.value && s.value.ok;
    data[k] = ok ? ((s.value.data && s.value.data.items) || []) : [];
    if (!ok) {
      const why = s.status === "rejected"
        ? (s.reason && s.reason.message) || "unreachable"
        : "HTTP " + ((s.value && s.value.status) || "?");
      unavailable.push({ key: k, why });
    }
  });
  const users = data.users, roles = data.roles, services = data.services,
        agents = data.agents, delegations = data.delegations,
        abac = data.abac, audit = data.audit;
  const missing = (k) => unavailable.find(u => u.key === k);
  const countOf = (k, arr) => missing(k) ? "—" : arr.length;
  const unavailableBanner = unavailable.length === 0 ? "" : `
    <div class="card" style="margin-bottom:18px;border-left:3px solid var(--warn,#c90)">
      <b>Some identity data could not be loaded.</b>
      <div class="hint">${unavailable.map(u => `${esc(u.key)} (${esc(u.why)})`).join(" · ")}.
      Counts shown as — are unknown, not zero.</div>
    </div>`;

  return `
    ${unavailableBanner}
    <div class="grid g4" style="margin-bottom:18px">
      <div class="card metric"><b>${countOf("users", users)}</b><span>human users</span></div>
      <div class="card metric"><b>${countOf("services", services)}</b><span>service identities</span></div>
      <div class="card metric"><b>${countOf("agents", agents)}</b><span>agent identities</span></div>
      <div class="card metric"><b>${countOf("roles", roles)}</b><span>RBAC roles</span></div>
    </div>

    <!-- Tab navigation -->
    <div style="display:flex;gap:8px;margin-bottom:18px;flex-wrap:wrap">
      <button class="sm iam-tab active" onclick="window.iamTab('users', this)">Users</button>
      <button class="sm iam-tab" onclick="window.iamTab('services', this)">Services</button>
      <button class="sm iam-tab" onclick="window.iamTab('agents', this)">Agents</button>
      <button class="sm iam-tab" onclick="window.iamTab('roles', this)">Roles</button>
      <button class="sm iam-tab" onclick="window.iamTab('delegation', this)">Delegation</button>
      <button class="sm iam-tab" onclick="window.iamTab('abac', this)">ABAC Policies</button>
      <button class="sm iam-tab" onclick="window.iamTab('audit', this)">Audit Log</button>
    </div>

    <!-- Users tab -->
    <div class="iam-panel" id="panel-users">
      <div class="card" style="margin-bottom:18px">
        <h3>Human users <span class="tag">Module 02 · Authenticak users</span></h3>
        <div class="hint">Human principals logging in via SSO, LDAP, AD, or password. Each has MFA support and role assignments.</div>
        <div class="frow" style="margin-bottom:14px">
          <input id="userName" placeholder="username / email">
          <input id="userEmail" placeholder="email">
          <select id="userRoleSelect" style="max-width:180px">
            <option value="">— assign role —</option>
            ${roles.map(r => `<option value="${esc(r.id)}">${esc(r.name)} (${esc((r.permissions||[]).length)} perms)</option>`).join("")}
          </select>
          <button class="sm" onclick="window.iamCreateUser()">Create user</button>
        </div>
        ${users.length === 0
          ? `<div class="empty">No users registered.</div>`
          : users.map(u => rowItem({
              title: `👤 ${esc(u.email || u.display_name || u.username || u.id.slice(0,8))}`,
              meta: `role: ${esc((u.role_ids||[]).map(rid=>roles.find(r=>r.id===rid)?.name||rid).join(", ") || "none")} · ${esc(u.mfa_enabled?"MFA ✅":"MFA ❌")} · created ${rel(u.created_at)}`,
              badges: badge(u.status || "active"),
              actions: u.status !== "deactivated"
                ? `<button class="ghost sm" onclick="window.iamSuspendUser('${esc(u.id)}')">Disable</button>`
                : `<button class="ok sm" onclick="window.iamActivateUser('${esc(u.id)}')">Enable</button>`,
            })).join("")}
      </div>
      <div class="card">
        <h3>Permission evaluation <span class="tag">RBAC check</span></h3>
        <div class="hint">Test whether an actor has a specific permission against the RBAC engine.</div>
        <div class="grid g3">
          <div><label>Actor ID</label><input id="evalActor" placeholder="user ID or service ID"></div>
          <div><label>Resource</label><select id="evalResource">
            ${RESOURCES.map(r=>`<option value="${esc(r)}">${esc(r)}</option>`).join("")}</select></div>
          <div><label>Action</label><select id="evalAction">
            ${ACTIONS.map(a=>`<option value="${esc(a)}">${esc(a)}</option>`).join("")}</select></div>
        </div>
        <button class="sm" style="margin-top:10px" onclick="window.iamEvaluatePermission()">Check permission</button>
        <div id="evalResult"></div>
      </div>
    </div>

    <!-- Services tab -->
    <div class="iam-panel" id="panel-services" style="display:none">
      <div class="card" style="margin-bottom:18px">
        <h3>Service identities <span class="tag">Module 02 · API keys</span></h3>
        <div class="hint">Non-human principals with API keys for automated systems. Each has role assignments and optional expiry.</div>
        <div class="frow" style="margin-bottom:14px">
          <input id="svcName" placeholder="service name">
          <select id="svcRoleSelect" style="max-width:180px">
            <option value="">— assign role —</option>
            ${roles.map(r => `<option value="${esc(r.id)}">${esc(r.name)}</option>`).join("")}
          </select>
          <button class="sm" onclick="window.iamCreateService()">Create service</button>
        </div>
        ${services.length === 0
          ? `<div class="empty">No service identities registered.</div>`
          : services.map(s => rowItem({
              title: `🔑 ${esc(s.name || s.id.slice(0,8))}`,
              meta: `role: ${esc((s.role_ids||[]).map(rid=>roles.find(r=>r.id===rid)?.name||rid).join(", ") || "none")} · API key: ${esc((s.api_key||"").slice(0,12))}${s.expires_at?" · expires "+esc(new Date(s.expires_at).toLocaleDateString()):" · never expires"}`,
              badges: badge("service"),
            })).join("")}
      </div>
    </div>

    <!-- Agents tab -->
    <div class="iam-panel" id="panel-agents" style="display:none">
      <div class="card" style="margin-bottom:18px">
        <h3>Agent identities <span class="tag">Module 02 · autonomous principals</span></h3>
        <div class="hint">Autonomous agent identities with capability, memory scope, and tool restrictions. Each agent can only perform what's explicitly allowed.</div>
        ${agents.length === 0
          ? `<div class="empty">No agent identities registered.</div>`
          : agents.map(a => rowItem({
              title: `🤖 ${esc(a.agent_name || a.id.slice(0,8))}`,
              meta: `caps: ${(a.capabilities||[]).map(esc).join(", ")} · tools: ${(a.allowed_tools||[]).map(esc).join(", ") || "none"}`,
              badges: badge(a.status || "active"),
            })).join("")}
      </div>
    </div>

    <!-- Roles tab -->
    <div class="iam-panel" id="panel-roles" style="display:none">
      <div class="card" style="margin-bottom:18px">
        <h3>RBAC roles <span class="tag">Module 02 · permission sets</span></h3>
        <div class="hint">Define permission sets. Permissions follow the pattern <code>resource.action</code> (e.g., <code>user.change</code>, <code>audit.view</code>).</div>
        <div class="frow" style="margin-bottom:14px">
          <input id="roleName" placeholder="role name (e.g. Department Manager)">
          <button class="sm" onclick="window.iamCreateRole()">Create role</button>
        </div>
        ${roles.length === 0
          ? `<div class="empty">No RBAC roles defined.</div>`
          : roles.map(r => {
              const perms = r.permissions || [];
              return `<div class="card" style="margin-bottom:12px">
                <div class="frow">
                  <h3 style="flex:1">${esc(r.name)} ${r.is_system?"<span class='tag'>system</span>":""}</h3>
                  <button class="ghost sm" onclick="window.iamEditRole('${esc(r.id)}')">Edit permissions</button>
                  ${r.is_system?"":`<button class="bad sm" onclick="window.iamDeleteRole('${esc(r.id)}')">Delete</button>`}
                </div>
                <div class="hint">${esc(r.description||"")}</div>
                <div class="frow" style="flex-wrap:wrap">
                  ${perms.map(p => {
                    const parts = p.split(".");
                    const res = parts[0], act = parts[1];
                    return `<span class="badge" style="margin:2px">${esc(res)} · ${esc(act)}</span>`;
                  }).join("") || "<span style='color:var(--text-muted);font-size:12px'>No permissions assigned</span>"}
                </div>
                <div class="hint">${esc((r.assignee_count||r.users_count||0))} user(s) assigned</div>
              </div>`;
            }).join("")}
      </div>
    </div>

    <!-- Delegation tab -->
    <div class="iam-panel" id="panel-delegation" style="display:none">
      <div class="card" style="margin-bottom:18px">
        <h3>Delegated admin roles <span class="tag">Module 02 · hierarchical permissions</span></h3>
        <div class="hint">Create admin roles that can be delegated to users with scoped authority. Supports inheritance chains via parent role + max depth.</div>
        <div class="frow" style="margin-bottom:14px">
          <input id="delName" placeholder="delegation role name">
          <select id="delScope" style="max-width:140px">
            ${DELEGATION_SCOPES.map(s=>`<option value="${esc(s)}">${esc(s)}</option>`).join("")}
          </select>
          <input id="delDepth" placeholder="max depth" type="number" value="0" style="max-width:90px">
          <button class="sm" onclick="window.iamCreateDelegation()">Create delegation</button>
        </div>
        ${delegations.length === 0
          ? `<div class="empty">No delegation roles defined.</div>`
          : delegations.map(d => rowItem({
              title: `👑 ${esc(d.name)}`,
              meta: `scope: ${esc(d.scope)} · depth: ${esc(String(d.max_delegation_depth||0))} · perms: ${(d.permissions||[]).length} · delegated to: ${(d.delegated_to_ids||[]).length}`,
              badges: badge(d.scope),
              actions: d.delegated_to_ids&&d.delegated_to_ids.length
                ? `<button class="ghost sm" onclick="window.iamShowDelegationUsers('${esc(d.id)}')">View users</button>`
                : `<button class="sm" onclick="window.iamGrantDelegation('${esc(d.id)}')">Grant to user</button>`,
            })).join("")}
        <div id="delUsers"></div>
      </div>
    </div>

    <!-- ABAC Policies tab -->
    <div class="iam-panel" id="panel-abac" style="display:none">
      <div class="card" style="margin-bottom:18px">
        <h3>ABAC policies <span class="tag">Module 02 · attribute-based conditions</span></h3>
        <div class="hint">Attribute-based access control rules. Rules can be <code>time</code>, <code>ip</code>, <code>ownership</code>, <code>department</code>, or <code>custom</code>. Effects are <code>allow</code> or <code>deny</code>.</div>
        <div class="frow" style="margin-bottom:14px">
          <input id="abacName" placeholder="policy name">
          <select id="abacRule" style="max-width:130px">
            ${ABAC_RULE_TYPES.map(r=>`<option value="${esc(r)}">${esc(r)}</option>`).join("")}
          </select>
          <select id="abacEffect" style="max-width:90px">
            <option value="allow">Allow</option>
            <option value="deny">Deny</option>
          </select>
          <button class="sm" onclick="window.iamCreateABAC()">Create policy</button>
        </div>
        ${abac.length === 0
          ? `<div class="empty">No ABAC policies defined.</div>`
          : abac.map(a => rowItem({
              title: `${esc(a.effect==="deny"?"🚫":"✅")}${esc(a.name)}`,
              meta: `rule: ${esc(a.rule_type||a.rule)} · ${esc(a.resource)} · ${esc(a.action)} · ${esc(a.effect)}`,
              badges: badge(a.effect),
              actions: `<button class="bad sm" onclick="window.iamDeleteABAC('${esc(a.id)}')">Delete</button>`,
            })).join("")}
      </div>
    </div>

    <!-- Audit tab -->
    <div class="iam-panel" id="panel-audit" style="display:none">
      <div class="card">
        <h3>Audit log <span class="tag">Module 02 · immutable events</span></h3>
        <div class="hint">Immutable record of all platform actions. Each event captures actor, resource, result, and IP address.</div>
        ${audit.length === 0
          ? `<div class="empty">No audit events recorded.</div>`
          : `<div style="max-height:500px;overflow:auto">
              <table>
                <thead><tr><th>Timestamp</th><th>Actor</th><th>Type</th><th>Action</th><th>Resource</th><th>Result</th><th>IP</th><th>Severity</th></tr></thead>
                <tbody>${audit.map(a => `<tr>
                  <td class="mono">${esc(rel(a.timestamp||a.created_at||""))}</td>
                  <td>${esc(a.actor_name||a.actor_id?.slice(0,8)||"system")}</td>
                  <td class="tag">${esc(a.actor_type||"user")}</td>
                  <td>${esc(a.action)}</td>
                  <td class="mono">${esc((a.resource_id||"").slice(0,8))}</td>
                  <td>${badge(a.result==="success"?"ok":"rejected")}</td>
                  <td class="mono">${esc(a.ip_address||"—")}</td>
                  <td>${badge(a.severity||"info")}</td>
                </tr>`).join("")}</tbody>
              </table>
            </div>`}
      </div>
    </div>`;
}

// ── Tab switching ──────────────────────────────────────────
// Inline, because this view is re-rendered on every visit: a listener attached
// at module load runs once, before any of these buttons exist.
window.iamTab = function (name, btn) {
  document.querySelectorAll(".iam-tab").forEach(b => b.classList.remove("active"));
  btn.classList.add("active");
  document.querySelectorAll(".iam-panel").forEach(p => p.style.display = "none");
  const panel = document.getElementById("panel-" + name);
  if (panel) panel.style.display = "block";
};

// ── User CRUD ──────────────────────────────────────────────
window.iamCreateUser = async function () {
  const username = $("userName").value.trim();
  if (!username) { toast("Username required", "bad"); return; }
  try {
    const body = { id: uuid4(), username };
    const email = $("userEmail")?.value.trim();
    if (email) body.email = email;
    const role = $("userRoleSelect").value;
    if (role) body.role_ids = [role];
    const r = await post(SVC_IAM + "/users", body);
    if (r.ok) { toast("User " + esc(username) + " created", "ok"); window.go("iam"); }
    else toast("Failed: " + esc(JSON.stringify(r.data).slice(0, 120)), "bad");
  } catch (e) { toast("Error: " + esc(String(e)), "bad"); }
};

window.iamSuspendUser = async function (id) {
  const r = await patch(SVC_IAM + "/users/" + id, { status: "deactivated" });
  if (r.ok) { toast("User disabled", "ok"); window.go("iam"); }
  else toast("Failed", "bad");
};

window.iamActivateUser = async function (id) {
  const r = await patch(SVC_IAM + "/users/" + id, { status: "active" });
  if (r.ok) { toast("User enabled", "ok"); window.go("iam"); }
  else toast("Failed", "bad");
};

// ── Service identity CRUD ──────────────────────────────────
window.iamCreateService = async function () {
  const name = $("svcName").value.trim();
  if (!name) { toast("Service name required", "bad"); return; }
  try {
    const body = { id: uuid4(), name };
    const role = $("svcRoleSelect").value;
    if (role) body.role_ids = [role];
    const r = await post(SVC_IAM + "/service-identities", body);
    if (r.ok) { toast("Service " + esc(name) + " created", "ok"); window.go("iam"); }
    else toast("Failed: " + esc(JSON.stringify(r.data).slice(0, 120)), "bad");
  } catch (e) { toast("Error: " + esc(String(e)), "bad"); }
};

// ── Role CRUD ──────────────────────────────────────────────
window.iamCreateRole = async function () {
  const name = $("roleName").value.trim();
  if (!name) { toast("Role name required", "bad"); return; }
  try {
    // Pre-generate a sensible default permission set
    const defaultPerms = ["user.view", "user.change", "agent.view", "department.view", "workflow.view"];
    const r = await post(SVC_IAM + "/roles", { id: uuid4(), name, permissions: defaultPerms });
    if (r.ok) { toast("Role " + esc(name) + " created with default permissions", "ok"); window.go("iam"); }
    else toast("Failed: " + esc(JSON.stringify(r.data).slice(0, 120)), "bad");
  } catch (e) { toast("Error: " + esc(String(e)), "bad"); }
};

window.iamDeleteRole = async function (id) {
  if (!confirm("Delete this role? Users with this role will be affected.")) return;
  const r = await del(SVC_IAM + "/roles/" + id);
  if (r.ok) { toast("Role deleted", "ok"); window.go("iam"); }
  else toast("Failed", "bad");
};

window.iamEditRole = async function (id) {
  // M02 has no PATCH for roles, so we show current permissions read-only
  const r = await get(SVC_IAM + "/roles/" + encodeURIComponent(id));
  if (!r.ok) { toast("Failed to load role", "bad"); return; }
  const role = r.data || {};
  const perms = (role.permissions || []).map(p => `  ${esc(p)}`).join("\n") || "  (none)";
  alert(`Role: ${esc(role.name)}\nDescription: ${esc(role.description || "none")}\n\nPermissions:\n${perms}`);
};

// ── Delegation ─────────────────────────────────────────────
window.iamCreateDelegation = async function () {
  const name = $("delName").value.trim();
  if (!name) { toast("Delegation name required", "bad"); return; }
  try {
    const r = await post(SVC_IAM + "/admin/delegation", {
      id: uuid4(), name, scope: $("delScope").value,
      max_delegation_depth: parseInt($("delDepth").value || "0"),
    });
    if (r.ok) { toast("Delegation role created", "ok"); window.go("iam"); }
    else toast("Failed: " + esc(JSON.stringify(r.data).slice(0, 120)), "bad");
  } catch (e) { toast("Error: " + esc(String(e)), "bad"); }
};

window.iamGrantDelegation = async function (id) {
  const userId = prompt("Enter user ID to grant this delegation to:");
  if (!userId) return;
  const r = await post(SVC_IAM + "/admin/delegations/" + id + "/grant", { user_id: userId });
  if (r.ok) { toast("Delegation granted", "ok"); window.go("iam"); }
  else toast("Failed: " + esc(JSON.stringify(r.data).slice(0, 120)), "bad");
};

window.iamShowDelegationUsers = async function (id) {
  const el = $("delUsers");
  el.innerHTML = `<div class="card" style="margin-top:12px"><h3>Users with this delegation</h3>
    <div class="hint">Click Revoke to remove access.</div>`;
  try {
    const r = await get(SVC_IAM + "/admin/delegations/" + id + "/delegations");
    const users = (r.data && r.data.items) || r.data || [];
    if (users.length === 0) { el.innerHTML += `<div class="empty">No users granted this delegation.</div>`; }
    else {
      el.innerHTML += users.map(u => rowItem({
        title: esc(u.name || u.user_id || u.id?.slice(0,8) || "user"),
        meta: `granted ${rel(u.granted_at||"")}`,
        actions: `<button class="bad sm" onclick="window.iamRevokeDelegation('${id}','${esc(u.user_id||u.id)}')">Revoke</button>`,
      })).join("");
    }
  } catch (e) { el.innerHTML += `<div class="error-box"><div class="err-msg">${esc(String(e))}</div></div>`; }
  el.innerHTML += "</div>";
};

window.iamRevokeDelegation = async function (delId, userId) {
  const r = await post(SVC_IAM + "/admin/delegations/" + delId + "/revoke", { user_id: userId });
  if (r.ok) { toast("Delegation revoked", "ok"); window.go("iam"); }
  else toast("Failed: " + esc(JSON.stringify(r.data).slice(0, 120)), "bad");
};

// ── ABAC Policies ──────────────────────────────────────────
window.iamCreateABAC = async function () {
  const name = $("abacName").value.trim();
  if (!name) { toast("Policy name required", "bad"); return; }
  try {
    const r = await post(SVC_IAM + "/abac/policies", {
      id: uuid4(), name,
      rule_type: $("abacRule").value,
      effect: $("abacEffect").value,
      resource: "any",
      action: "any",
      conditions: {},
    });
    if (r.ok) { toast("ABAC policy created", "ok"); window.go("iam"); }
    else toast("Failed: " + esc(JSON.stringify(r.data).slice(0, 120)), "bad");
  } catch (e) { toast("Error: " + esc(String(e)), "bad"); }
};

window.iamDeleteABAC = async function (id) {
  const r = await del(SVC_IAM + "/abac/policies/" + id);
  if (r.ok) { toast("Policy deleted", "ok"); window.go("iam"); }
  else toast("Failed", "bad");
};

// ── Permission evaluation ──────────────────────────────────
window.iamEvaluatePermission = async function () {
  const actor = $("evalActor").value.trim();
  if (!actor) { toast("Actor ID required", "bad"); return; }
  const res = $("evalResource").value;
  const act = $("evalAction").value;
  try {
    const r = await post(SVC_IAM + "/rbac/evaluate", {
      actor_id: actor, resource: res, action: act,
      attributes: {},
    });
    const result = r.data || {};
    $("evalResult").innerHTML = `<div class="result" style="margin-top:10px">
      <div class="q">${esc(actor)} → ${esc(res)}.${esc(act)}</div>
      <div class="a" style="color:${result.allowed ? "var(--ok)" : "var(--bad)"}">
        ${result.allowed ? "✅ ALLOWED" : "🚫 DENIED"}
      </div>
      ${result.reason ? `<div class="meta"><span>Reason: ${esc(result.reason)}</span></div>` : ""}
      ${result.policy_match ? `<div class="meta"><span>Matched: ${esc(result.policy_match)}</span></div>` : ""}
      ${result.evaluated_at ? `<div class="meta"><span>Evaluated: ${esc(result.evaluated_at)}</span></div>` : ""}
    </div>`;
  } catch (e) { $("evalResult").innerHTML = `<div class="error-box"><div class="err-msg">${esc(String(e))}</div></div>`; }
};

function viewError(title, msg) {
  return `<div class="error-box"><div class="err-title">${esc(title)}</div><div class="err-msg">${esc(msg)}</div><button onclick="window.go('iam')">Retry</button></div>`;
}