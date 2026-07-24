// Policies — the rulebook that bounds the automation.
//
// Two layers, honestly separated: DEPARTMENT rules ship on the template and
// are live today (they shape gates, autonomy tiers and decision rights);
// PLATFORM policies (Module 10) are groups of allow/deny/proxy rules with an
// evaluation API — consulted on demand, not yet wired into run execution.
import { SVC, get, post, del, unwrapList, listDepartments, getDepartment } from "../api.js";
import { $, esc, badge, rel, toast } from "../ui.js";

const ACTIONS = ["deny", "allow", "proxy"];
const SCOPES = ["department", "agent", "tenant", "global"];
const RESOURCE_TYPES = ["tool", "model", "workflow", "data", "all"];
const EFFECTS = ["enforce", "warn", "log"];
const ACTION_ICON = { allow: "✅", deny: "🚫", proxy: "🔁" };

export async function viewPolicies() {
  const [policiesR, groupsR, auditR, deptR] = await Promise.allSettled([
    get(SVC.policies + "/policies"),
    get(SVC.policies + "/policy-groups"),
    get(SVC.policies + "/audit?page_size=50"),
    listDepartments(1, 50),
  ]);
  const ok = r => r.status === "fulfilled" ? r.value : null;
  const policies = unwrapList(ok(policiesR), "policies");
  const groups = unwrapList(ok(groupsR), "groups");
  const audit = unwrapList(ok(auditR), "audits");
  const depts = unwrapList(ok(deptR)).filter(d => d.status === "operational" || d.status === "degraded");

  // Department rules are on the full record.
  const details = (await Promise.allSettled(depts.map(d => getDepartment(d.id))))
    .filter(r => r.status === "fulfilled" && r.value.ok).map(r => r.value.data);

  const deptRules = details.flatMap(d => (d.governance_rules || []).map(g => ({ dept: d, rule: g })));
  window._pol = { groups, depts };

  return `<div id="polRoot">
    <div class="grid g4" style="margin-bottom:18px">
      <div class="card metric"><b>${deptRules.length}</b><span>department rules (live)</span></div>
      <div class="card metric"><b>${policies.length}</b><span>platform policies</span></div>
      <div class="card metric"><b>${policies.filter(p => p.action === "deny").length}</b><span>deny rules</span></div>
      <div class="card metric"><b>${audit.length}</b><span>audit events</span></div>
    </div>

    <div class="card" style="margin-bottom:18px">
      <h3>Department rules <span class="tag">live — shipped by the template</span></h3>
      <div class="hint">These already govern the org: they define the gates, autonomy tiers and decision
      rights you see on Teams and in every run.</div>
      ${deptRules.length === 0
        ? `<div class="empty">No operational departments with governance rules.</div>`
        : deptRules.map(({ dept, rule }) => `
          <div class="row-item">
            <div class="grow"><div class="t">⚙️ ${esc(rule.name)} <span class="tag">${esc(dept.name)}</span></div>
            <div class="m">${esc(rule.description || rule.type || "")}</div></div>
            <div class="actions"><span class="badge ${esc(rule.enforcement || "enforce")}">${esc(rule.enforcement || "enforce")}</span></div>
          </div>`).join("")}
    </div>

    <div style="display:flex;gap:8px;margin-bottom:18px;flex-wrap:wrap">
      <button class="sm policy-tab active" data-tab="policies" onclick="window.polTab('policies', this)">Platform policies</button>
      <button class="sm policy-tab" data-tab="groups" onclick="window.polTab('groups', this)">Groups</button>
      <button class="sm policy-tab" data-tab="evaluate" onclick="window.polTab('evaluate', this)">Evaluate</button>
      <button class="sm policy-tab" data-tab="audit" onclick="window.polTab('audit', this)">Audit</button>
    </div>

    <div class="policy-panel" id="panel-policies">
      <div class="card" style="margin-bottom:18px">
        <h3>Platform policies <span class="tag">Module 10 · evaluation API — not yet enforced in the run path</span></h3>
        <div class="hint">Allow/deny/proxy rules over tools, models, workflows and data. Policies live inside a
        group${groups.length === 0 ? " — create one on the Groups tab first" : ""}.</div>
        <div class="frow" style="margin-bottom:14px">
          <select id="polGroup" style="max-width:180px">${groups.length === 0
            ? `<option value="">— no groups yet —</option>`
            : groups.map(g => `<option value="${esc(g.id)}">${esc(g.name)}</option>`).join("")}</select>
          <input id="policyName" placeholder="policy name (e.g. No external email tools)" style="flex:1;min-width:180px">
          <select id="polAction" style="max-width:90px">${ACTIONS.map(a => `<option>${a}</option>`).join("")}</select>
          <select id="polScope" style="max-width:120px">${SCOPES.map(s => `<option>${s}</option>`).join("")}</select>
          <select id="polResType" style="max-width:110px">${RESOURCE_TYPES.map(rt => `<option>${rt}</option>`).join("")}</select>
          <select id="polEffect" style="max-width:100px">${EFFECTS.map(e => `<option>${e}</option>`).join("")}</select>
          <button class="sm" onclick="window.polCreatePolicy()">Create</button>
        </div>
        ${policies.length === 0
          ? `<div class="empty">No platform policies yet.</div>`
          : policies.map(p => `
            <div class="row-item">
              <div class="grow"><div class="t">${ACTION_ICON[p.action] || "•"} ${esc(p.name)}
                <span class="tag">${esc(p.scope || "all")} · ${esc(p.resource_type || "all")}</span></div>
              <div class="m">${esc(p.description || "")}${p.priority != null ? ` · priority ${esc(String(p.priority))}` : ""} · ${rel(p.created_at)}</div></div>
              <div class="actions"><span class="badge ${p.is_active !== false ? "active" : "expired"}">${p.is_active !== false ? "active" : "inactive"}</span>
                <span class="badge ${esc(p.effect || "enforce")}">${esc(p.effect || "enforce")}</span>
                <button class="bad sm" onclick="window.polDeletePolicy('${esc(p.id)}')">Delete</button></div>
            </div>`).join("")}
      </div>
    </div>

    <div class="policy-panel" id="panel-groups" style="display:none">
      <div class="card" style="margin-bottom:18px">
        <h3>Policy groups <span class="tag">Module 10</span></h3>
        <div class="hint">A group bundles related policies (e.g. "Finance guardrails"). Policies require one.</div>
        <div class="frow" style="margin-bottom:14px">
          <input id="groupName" placeholder="group name (e.g. Finance guardrails)">
          <button class="sm" onclick="window.polCreateGroup()">Create group</button>
        </div>
        ${groups.length === 0
          ? `<div class="empty">No policy groups yet.</div>`
          : groups.map(g => `
            <div class="row-item">
              <div class="grow"><div class="t">📋 ${esc(g.name)}</div>
              <div class="m">${policies.filter(p => p.group_id === g.id).length} policy(ies) · created ${rel(g.created_at)}</div></div>
              <div class="actions"><button class="bad sm" onclick="window.polDeleteGroup('${esc(g.id)}')">Delete</button></div>
            </div>`).join("")}
      </div>
    </div>

    <div class="policy-panel" id="panel-evaluate" style="display:none">
      <div class="card" style="margin-bottom:18px">
        <h3>Policy evaluation <span class="tag">would this be allowed?</span></h3>
        <div class="hint">Ask the engine directly. Deny beats allow; matches are audited.</div>
        <div class="grid g2">
          <div><label>Resource</label><input id="evalResource" placeholder="e.g. tool:external-email"></div>
          <div><label>Action</label><input id="evalActionType" placeholder="e.g. execute"></div>
          <div><label>Agent role</label><select id="evalRole">
            <option value="">— any —</option><option>analyst</option><option>executor</option>
            <option>manager</option><option>support</option><option>specialist</option></select></div>
          <div><label>Department</label><select id="evalDept">
            <option value="">— any —</option>
            ${depts.map(d => `<option value="${esc(d.id)}">${esc(d.name)}</option>`).join("")}</select></div>
          <div><label>Cost (USD, optional)</label><input id="evalCost" type="number" step="0.01" placeholder="0"></div>
        </div>
        <button class="sm" style="margin-top:12px" onclick="window.polEvaluate()">Evaluate</button>
        <div id="evalResult"></div>
      </div>
    </div>

    <div class="policy-panel" id="panel-audit" style="display:none">
      <div class="card">
        <h3>Audit log <span class="tag">every evaluation, recorded</span></h3>
        ${audit.length === 0
          ? `<div class="empty">No evaluations recorded yet — try one on the Evaluate tab.</div>`
          : audit.map(a => `
            <div class="row-item">
              <div class="grow"><div class="t">${a.allowed === false ? "🚫" : "✅"} ${esc(a.resource || a.action_type || "evaluation")}</div>
              <div class="m">${esc(a.action_type || "")}${a.agent_role ? " · role " + esc(a.agent_role) : ""}${a.policy_name ? " · matched " + esc(a.policy_name) : ""}${a.reason ? " · " + esc(a.reason) : ""} · ${rel(a.created_at || a.timestamp)}</div></div>
              <div class="actions"><span class="badge ${a.allowed === false ? "rejected" : "approved"}">${a.allowed === false ? "denied" : "allowed"}</span></div>
            </div>`).join("")}
      </div>
    </div>
  </div>`;
}

// ── Panel switching (inline, so it works after every render) ─
window.polTab = function (name, btn) {
  document.querySelectorAll(".policy-tab").forEach(b => b.classList.remove("active"));
  btn.classList.add("active");
  document.querySelectorAll(".policy-panel").forEach(p => p.style.display = "none");
  const panel = document.getElementById("panel-" + name);
  if (panel) panel.style.display = "block";
};

// ── CRUD with the real Module 10 schema ─────────────────────
window.polCreatePolicy = async function () {
  const groupId = $("polGroup").value;
  const name = $("policyName").value.trim();
  if (!groupId) { toast("Policies need a group — create one on the Groups tab", "warn"); return; }
  if (!name) { toast("Give the policy a name", "warn"); return; }
  const r = await post(SVC.policies + "/policies", {
    group_id: groupId, name,
    action: $("polAction").value, scope: $("polScope").value,
    resource_type: $("polResType").value, effect: $("polEffect").value,
  });
  if (r.ok) { toast("Policy created", "ok"); window.go("policies"); }
  else toast("Create failed: " + esc(r.data?.message || r.data?.error?.message || r.status), "bad");
};

window.polDeletePolicy = async function (id) {
  if (!confirm("Delete this policy?")) return;
  const r = await del(SVC.policies + "/policies/" + encodeURIComponent(id));
  if (r.ok) { toast("Policy deleted", "ok"); window.go("policies"); }
  else toast("Delete failed: " + esc(r.data?.message || r.status), "bad");
};

window.polCreateGroup = async function () {
  const name = $("groupName").value.trim();
  if (!name) { toast("Give the group a name", "warn"); return; }
  const r = await post(SVC.policies + "/policy-groups", { name, metadata: {} });
  if (r.ok) { toast("Group created", "ok"); window.go("policies"); }
  else toast("Create failed: " + esc(r.data?.message || r.status), "bad");
};

window.polDeleteGroup = async function (id) {
  if (!confirm("Delete this group?")) return;
  const r = await del(SVC.policies + "/policy-groups/" + encodeURIComponent(id));
  if (r.ok) { toast("Group deleted", "ok"); window.go("policies"); }
  else toast("Delete failed: " + esc(r.data?.message || r.status), "bad");
};

window.polEvaluate = async function () {
  const resource = $("evalResource").value.trim();
  if (!resource) { toast("Name the resource to evaluate", "warn"); return; }
  const body = {
    resource,
    action_type: $("evalActionType").value.trim() || "execute",
  };
  if ($("evalRole").value) body.agent_role = $("evalRole").value;
  if ($("evalDept").value) body.department_id = $("evalDept").value;
  if ($("evalCost").value) body.cost = parseFloat($("evalCost").value);
  const r = await post(SVC.policies + "/policies/evaluate", body);
  if (!r.ok) {
    $("evalResult").innerHTML = `<div class="error-box" style="margin-top:10px"><div class="err-msg">${esc(r.data?.message || "evaluation failed")}</div></div>`;
    return;
  }
  const res = r.data || {};
  $("evalResult").innerHTML = `<div class="card" style="margin-top:12px">
    <div class="t" style="color:${res.allowed !== false ? "var(--ok)" : "var(--bad)"};font-weight:700">
      ${res.allowed !== false ? "✅ ALLOWED" : "🚫 DENIED"}</div>
    <div class="m" style="margin-top:4px">${esc(resource)} · ${esc(body.action_type)}
      ${res.policy_name ? ` · matched <b>${esc(res.policy_name)}</b>` : ""}
      ${res.reason ? ` · ${esc(res.reason)}` : ""}</div>
    ${(res.warnings || []).length ? `<div class="m" style="margin-top:4px;color:var(--warn)">⚠ ${res.warnings.map(w => esc(typeof w === "string" ? w : w.policy_name || JSON.stringify(w))).join(" · ")}</div>` : ""}
  </div>`;
};
