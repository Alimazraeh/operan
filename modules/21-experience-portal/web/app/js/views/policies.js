// Policy Governance (Module 10): policy CRUD, evaluation, policy groups, audit log.
import { SVC, get, post, patch, del, uuid4, unwrapList } from "../api.js";
import { $, esc, badge, rel, toast, rowItem } from "../ui.js";

const POLICY_TYPES = ["allow", "deny"];
const OPERATORS = ["equals", "not_equals", "contains", "exists", "in", "not_in"];
const TARGET_SCOPES = ["all", "department", "team", "role", "user"];

export async function viewPolicies() {
  let policiesR, groupsR, auditR;
  try {
    [policiesR, groupsR, auditR] = await Promise.all([
      get(SVC.policies + "/policies"),
      get(SVC.policies + "/policy-groups"),
      get(SVC.policies + "/audit"),
    ]);
  } catch (e) { return viewError("Failed to load policy data", e.message); }

  const policies = unwrapList(policiesR, "policies");
  const groups = unwrapList(groupsR, "groups");
  const audit = unwrapList(auditR, "audit");

  return `
    <div class="grid g4" style="margin-bottom:18px">
      <div class="card metric"><b>${policies.length}</b><span>policies</span></div>
      <div class="card metric"><b>${groups.length}</b><span>policy groups</span></div>
      <div class="card metric"><b>${audit.length}</b><span>audit events</span></div>
      <div class="card metric"><b>${policies.filter(p=>p.type==="deny").length}</b><span>deny rules</span></div>
    </div>

    <!-- Tab navigation -->
    <div style="display:flex;gap:8px;margin-bottom:18px;flex-wrap:wrap">
      <button class="sm policy-tab active" data-tab="policies">Policies</button>
      <button class="sm policy-tab" data-tab="groups">Groups</button>
      <button class="sm policy-tab" data-tab="evaluate">Evaluate</button>
      <button class="sm policy-tab" data-tab="audit">Audit</button>
    </div>

    <!-- Policies tab -->
    <div class="policy-panel" id="panel-policies">
      <div class="card" style="margin-bottom:18px">
        <h3>Policy rules <span class="tag">Module 10</span></h3>
        <div class="hint">Create and manage governance policies. Policies enforce allow/deny rules on agent behavior, resource access, and execution limits.</div>
        <div class="frow" style="margin-bottom:14px">
          <input id="policyName" placeholder="policy name (e.g. No-External-API-Calls)">
          <select id="policyType" style="max-width:90px">
            <option value="allow">Allow</option>
            <option value="deny">Deny</option>
          </select>
          <button class="sm" onclick="window.polCreatePolicy()">Create policy</button>
        </div>
        ${policies.length === 0
          ? `<div class="empty">No policies defined. Create one above.</div>`
          : policies.map(p => {
              const conditions = (p.conditions || p.rules || []);
              return `<div class="card" style="margin-bottom:12px">
                <div class="frow">
                  <h3 style="flex:1">${p.type === "deny" ? "🚫" : "✅"} ${esc(p.name || p.id.slice(0,8))}</h3>
                  <button class="bad sm" onclick="window.polDeletePolicy('${esc(p.id)}')">Delete</button>
                </div>
                <div class="hint">${esc(p.description || p.scope || "No description")}</div>
                ${conditions.length > 0 ? `<div style="margin-top:6px" class="frow" style="flex-wrap:wrap">
                  ${conditions.map(c => {
                    const label = c.rule_type || c.operator || c.target || c.condition || "?";
                    return `<span class="badge" style="margin:2px">${esc(String(label))}</span>`;
                  }).join("")}
                </div>` : ""}
                <div class="hint" style="margin-top:4px">created ${rel(p.created_at || p.createdAt || "")}</div>
              </div>`;
            }).join("")}
      </div>
    </div>

    <!-- Groups tab -->
    <div class="policy-panel" id="panel-groups" style="display:none">
      <div class="card" style="margin-bottom:18px">
        <h3>Policy groups <span class="tag">Module 10</span></h3>
        <div class="hint">Group policies together for scoped enforcement. Each group can be assigned to a department or role.</div>
        <div class="frow" style="margin-bottom:14px">
          <input id="groupName" placeholder="group name (e.g. Finance Compliance)">
          <select id="groupScope" style="max-width:120px">
            ${TARGET_SCOPES.map(s => `<option value="${esc(s)}">${esc(s)}</option>`).join("")}
          </select>
          <button class="sm" onclick="window.polCreateGroup()">Create group</button>
        </div>
        ${groups.length === 0
          ? `<div class="empty">No policy groups defined.</div>`
          : groups.map(g => rowItem({
              title: `📋 ${esc(g.name || g.id.slice(0,8))}`,
              meta: `scope: ${esc(g.scope || "all")} · policies: ${(g.policy_ids || g.policies || []).length} · created ${rel(g.created_at || g.createdAt || "")}`,
              badges: badge(g.scope || "all"),
              actions: `<button class="bad sm" onclick="window.polDeleteGroup('${esc(g.id)}')">Delete</button>`,
            })).join("")}
      </div>
    </div>

    <!-- Evaluate tab -->
    <div class="policy-panel" id="panel-evaluate" style="display:none">
      <div class="card" style="margin-bottom:18px">
        <h3>Policy evaluation <span class="tag">Module 10</span></h3>
        <div class="hint">Test whether an action is allowed under the current policy set. The engine evaluates all matching policies and returns a combined result.</div>
        <div class="grid g2">
          <div><label>Action</label><input id="evalAction" placeholder="e.g. send_external_email"></div>
          <div><label>Target</label><input id="evalTarget" placeholder="e.g. finance-dept"></div>
          <div><label>Agent ID</label><input id="evalAgent" placeholder="agent-uuid"></div>
          <div><label>User Role</label><select id="evalRole">
            <option value="admin">admin</option>
            <option value="manager">manager</option>
            <option value="agent">agent</option>
            <option value="viewer">viewer</option>
          </select></div>
        </div>
        <button class="sm" style="margin-top:12px" onclick="window.polEvaluate()">Evaluate</button>
        <div id="evalResult"></div>
      </div>
    </div>

    <!-- Audit tab -->
    <div class="policy-panel" id="panel-audit" style="display:none">
      <div class="card">
        <h3>Policy audit log <span class="tag">Module 10</span></h3>
        <div class="hint">Record of all policy decisions, evaluations, and enforcement actions.</div>
        ${audit.length === 0
          ? `<div class="empty">No audit events recorded.</div>`
          : `<div style="max-height:500px;overflow:auto">
              <table>
                <thead><tr><th>Time</th><th>Action</th><th>Result</th><th>Agent</th><th>Policy</th><th>Reason</th></tr></thead>
                <tbody>${audit.map(a => `<tr>
                  <td class="mono">${esc(rel(a.timestamp || a.created_at || ""))}</td>
                  <td>${esc(a.action || a.event || "—")}</td>
                  <td>${badge(a.result === "allowed" || a.result === "pass" ? "ok" : "rejected")}</td>
                  <td class="mono">${esc((a.agent_id || a.actor || "").slice(0,8))}</td>
                  <td class="mono">${esc((a.policy_id || a.policy || "").slice(0,8))}</td>
                  <td>${esc(a.reason || a.details || "—")}</td>
                </tr>`).join("")}</tbody>
              </table>
            </div>`}
      </div>
    </div>`;
}

// ── Tab switching ──────────────────────────────────────────
document.querySelectorAll(".policy-tab").forEach(btn => {
  btn.addEventListener("click", () => {
    document.querySelectorAll(".policy-tab").forEach(b => b.classList.remove("active"));
    btn.classList.add("active");
    document.querySelectorAll(".policy-panel").forEach(p => p.style.display = "none");
    const panel = document.getElementById("panel-" + btn.dataset.tab);
    if (panel) panel.style.display = "block";
  });
});

// ── Policy CRUD ────────────────────────────────────────────
window.polCreatePolicy = async function () {
  const name = $("policyName").value.trim();
  if (!name) { toast("Policy name required", "bad"); return; }
  try {
    const r = await post(SVC.policies + "/policies", {
      id: uuid4(), name, type: $("policyType").value,
      description: `Policy: ${name}`,
      conditions: [], scope: "all",
    });
    if (r.ok) { toast("Policy " + esc(name) + " created", "ok"); window.go("policies"); }
    else toast("Failed: " + esc(JSON.stringify(r.data).slice(0, 120)), "bad");
  } catch (e) { toast("Error: " + esc(String(e)), "bad"); }
};

window.polDeletePolicy = async function (id) {
  if (!confirm("Delete this policy?")) return;
  const r = await del(SVC.policies + "/policies/" + encodeURIComponent(id));
  if (r.ok) { toast("Policy deleted", "ok"); window.go("policies"); }
  else toast("Failed: " + esc(JSON.stringify(r.data).slice(0, 100)), "bad");
};

// ── Group CRUD ─────────────────────────────────────────────
window.polCreateGroup = async function () {
  const name = $("groupName").value.trim();
  if (!name) { toast("Group name required", "bad"); return; }
  try {
    const r = await post(SVC.policies + "/policy-groups", {
      id: uuid4(), name, scope: $("groupScope").value,
      policy_ids: [],
    });
    if (r.ok) { toast("Group " + esc(name) + " created", "ok"); window.go("policies"); }
    else toast("Failed: " + esc(JSON.stringify(r.data).slice(0, 120)), "bad");
  } catch (e) { toast("Error: " + esc(String(e)), "bad"); }
};

window.polDeleteGroup = async function (id) {
  if (!confirm("Delete this group?")) return;
  const r = await del(SVC.policies + "/policy-groups/" + encodeURIComponent(id));
  if (r.ok) { toast("Group deleted", "ok"); window.go("policies"); }
  else toast("Failed: " + esc(JSON.stringify(r.data).slice(0, 100)), "bad");
};

// ── Policy evaluation ──────────────────────────────────────
window.polEvaluate = async function () {
  const action = $("evalAction").value.trim();
  if (!action) { toast("Action required", "bad"); return; }
  try {
    const r = await post(SVC.policies + "/policies/evaluate", {
      action,
      target: $("evalTarget").value.trim() || "default",
      agent_id: $("evalAgent").value.trim() || "unknown",
      role: $("evalRole").value,
      context: {
        department: $("evalTarget").value.trim() || "default",
      },
    });
    const result = r.data || {};
    $("evalResult").innerHTML = `<div class="result" style="margin-top:10px">
      <div class="q">${esc(action)} → ${esc($("evalTarget").value || "default")}</div>
      <div class="a" style="color:${result.allowed !== false ? "var(--ok)" : "var(--bad)"}">
        ${result.allowed !== false ? "✅ ALLOWED" : "🚫 DENIED"}
      </div>
      ${result.policy_match ? `<div class="meta"><span>Matched: ${esc(result.policy_match)}</span></div>` : ""}
      ${result.reason ? `<div class="meta"><span>Reason: ${esc(result.reason)}</span></div>` : ""}
      ${result.evaluated_at ? `<div class="meta"><span>Evaluated: ${esc(result.evaluated_at)}</span></div>` : ""}
      ${result.policies_evaluated ? `<div class="meta"><span>Policies checked: ${esc(String(result.policies_evaluated))}</span></div>` : ""}
    </div>`;
  } catch (e) { $("evalResult").innerHTML = `<div class="error-box"><div class="err-msg">${esc(String(e))}</div></div>`; }
};

function viewError(title, msg) {
  return `<div class="error-box"><div class="err-title">${esc(title)}</div><div class="err-msg">${esc(msg)}</div><button onclick="window.go('policies')">Retry</button></div>`;
}