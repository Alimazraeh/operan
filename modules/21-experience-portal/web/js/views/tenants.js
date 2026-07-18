// Tenants: CRUD, quota management, lifecycle (Module 01).
import { SVC, get, post, patch, del, uuid4 } from "../api.js";
import { $, esc, badge, rel, toast, rowItem } from "../ui.js";

export async function viewTenants() {
  let tR, quotaR, usageR;
  try {
    [tR, quotaR, usageR] = await Promise.all([
      get(SVC.tenant + "/tenants?page_size=100"),
      get(SVC.tenant + "/quotas"),
      get(SVC.tenant + "/usage"),
    ]);
  } catch (e) { return viewError("Failed to load tenants", e.message); }

  const tenants = (tR.data && tR.data.items) || [];
  const quotas = quotaR.data || {};
  const usage = usageR.data || {};

  return `
    <div class="grid g4" style="margin-bottom:18px">
      <div class="card metric"><b>${tenants.length}</b><span>total tenants</span></div>
      <div class="card metric"><b>${tenants.filter(t=>t.status==='active').length}</b><span>active</span></div>
      <div class="card metric"><b>${(quotas.total_agents||0)}</b><span>max agent capacity</span></div>
      <div class="card metric"><b>${(usage.total_tokens||0)}</b><span>tokens consumed</span></div>
    </div>

    <div class="card" style="margin-bottom:18px">
      <h3>Tenant registry <span class="tag">Module 01</span></h3>
      <div class="hint">Create, configure, and manage tenant lifecycles with quota enforcement.</div>
      <div class="frow" style="margin-bottom:14px">
        <input id="tenantName" placeholder="tenant name (e.g. acme-corp)">
        <select id="tenantPlan" style="max-width:140px">
          <option>starter</option><option>enterprise</option><option>sovereign</option>
        </select>
        <button class="sm" onclick="window.createTenant()">Create tenant</button>
      </div>
      ${tenants.length === 0
        ? `<div class="empty">No tenants yet — create one above.</div>`
        : tenants.map(t => rowItem({
            title: `${esc(t.name || t.id)} <span class="tag">${esc(t.plan || "starter")}</span>`,
            meta: `created ${rel(t.created_at)} · ${esc(t.id.slice(0,8))}`,
            badges: badge(t.status || "active"),
            actions: t.status === "active"
              ? `<button class="ghost sm" onclick="window.suspendTenant('${esc(t.id)}')">Suspend</button>
                 <button class="bad sm" onclick="window.deleteTenant('${esc(t.id)}')">Delete</button>`
              : `<button class="ok sm" onclick="window.activateTenant('${esc(t.id)}')">Activate</button>`,
          })).join("")}
    </div>

    <div class="grid g2">
      <div class="card">
        <h3>Quota management <span class="tag">Module 01</span></h3>
        <div class="hint">Set per-tenant resource limits for agents, tokens, storage, and execution throughput.</div>
        <div class="grid g2">
          <div><label>Max agents</label><input id="quotaAgents" type="number" value="${esc(String(quotas.max_agents || 100))}"></div>
          <div><label>Max tokens/day</label><input id="quotaTokens" type="number" value="${esc(String(quotas.max_tokens_per_day || 100000))}"></div>
          <div><label>Max storage (MB)</label><input id="quotaStorage" type="number" value="${esc(String(quotas.max_storage_mb || 5120))}"></div>
          <div><label>Max workflows/day</label><input id="quotaWorkflows" type="number" value="${esc(String(quotas.max_workflows_per_day || 1000))}"></div>
        </div>
        <div style="margin-top:12px"><button class="sm" onclick="window.updateQuotas()">Update quotas</button></div>
      </div>
      <div class="card">
        <h3>Resource usage</h3>
        <div class="hint">Current consumption vs quota for this tenant.</div>
        <div class="kv">
          <dt>Agents used</dt><dd>${esc(String(usage.agents_used || 0))} / ${esc(String(quotas.max_agents || 100))}</dd>
          <dt>Tokens today</dt><dd>${esc(String(usage.tokens_today || 0))} / ${esc(String(quotas.max_tokens_per_day || 100000))}</dd>
          <dt>Storage used</dt><dd>${esc(String(usage.storage_used_mb || 0))} MB / ${esc(String(quotas.max_storage_mb || 5120))} MB</dd>
          <dt>Workflows today</dt><dd>${esc(String(usage.workflows_today || 0))} / ${esc(String(quotas.max_workflows_per_day || 1000))}</dd>
        </div>
        ${(usage.agents_used >= (quotas.max_agents || 100) || usage.tokens_today >= (quotas.max_tokens_per_day || 100000))
          ? `<div class="error-box" style="margin-top:12px"><div class="err-title">Quota exceeded!</div><div class="err-msg">This tenant has reached its resource limit. Consider increasing quotas or suspending inactive agents.</div></div>`
          : ""}
      </div>
    </div>`;
}

window.createTenant = async function () {
  const name = $("tenantName").value.trim();
  if (!name) { toast("Tenant name is required", "bad"); return; }
  try {
    const r = await post(SVC.tenant + "/tenants", {
      id: uuid4(), name, plan: $("tenantPlan").value || "starter",
      status: "active", created_by: "portal-admin",
    });
    if (r.ok) { toast("Tenant " + esc(name) + " created", "ok"); window.go("tenants"); }
    else toast("Failed: " + esc(JSON.stringify(r.data).slice(0, 120)), "bad");
  } catch (e) { toast("Error: " + esc(String(e)), "bad"); }
};

window.suspendTenant = async function (id) {
  const r = await patch(SVC.tenant + "/tenants/" + id, { status: "suspended" });
  if (r.ok) { toast("Tenant suspended", "ok"); window.go("tenants"); }
  else toast("Failed", "bad");
};

window.activateTenant = async function (id) {
  const r = await patch(SVC.tenant + "/tenants/" + id, { status: "active" });
  if (r.ok) { toast("Tenant activated", "ok"); window.go("tenants"); }
  else toast("Failed", "bad");
};

window.deleteTenant = async function (id) {
  if (!confirm("Delete this tenant? This action cannot be undone.")) return;
  const r = await del(SVC.tenant + "/tenants/" + id);
  if (r.ok) { toast("Tenant deleted", "ok"); window.go("tenants"); }
  else toast("Failed: " + esc(JSON.stringify(r.data).slice(0, 100)), "bad");
};

window.updateQuotas = async function () {
  const quotas = {};
  const agents = $("quotaAgents").value;
  const tokens = $("quotaTokens").value;
  const storage = $("quotaStorage").value;
  const workflows = $("quotaWorkflows").value;
  if (agents) quotas.max_agents = parseInt(agents);
  if (tokens) quotas.max_tokens_per_day = parseInt(tokens);
  if (storage) quotas.max_storage_mb = parseInt(storage);
  if (workflows) quotas.max_workflows_per_day = parseInt(workflows);
  if (!Object.keys(quotas).length) return;
  const r = await post(SVC.tenant + "/quotas", quotas);
  if (r.ok) { toast("Quotas updated", "ok"); window.go("tenants"); }
  else toast("Failed: " + esc(JSON.stringify(r.data).slice(0, 100)), "bad");
};

function viewError(title, msg) {
  return `<div class="error-box"><div class="err-title">${esc(title)}</div><div class="err-msg">${esc(msg)}</div><button onclick="window.go('tenants')">Retry</button></div>`;
}