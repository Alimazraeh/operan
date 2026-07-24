// Tenants: CRUD, quota management, lifecycle (Module 01).
import { SVC, get, patch, del, session, registerTenant } from "../api.js";
import { $, esc, badge, rel, toast, rowItem } from "../ui.js";

export async function viewTenants() {
  let tR, aR;
  try {
    [tR, aR] = await Promise.all([
      get(SVC.tenant + "/tenants?page_size=100"),
      get(SVC.registry + "/registry/agents?page_size=1"),
    ]);
  } catch (e) { return viewError("Failed to load tenants", e.message); }

  const tenants = (tR.data && tR.data.items) || [];
  // The signed-in tenant's own record drives the quota card.
  const current = tenants.find(t => t.id === session.tenant || t.slug === session.tenant || t.name === session.tenant) || null;
  const quotas = (current && current.quota) || {};
  const agentsUsed = (aR.data && aR.data.total) || 0;
  window._currentTenantRecordId = current ? current.id : null;
  // Quota lives on each tenant record — aggregate rather than calling
  // endpoints M01 doesn't have.
  const maxAgents = tenants.reduce((sum, t) => sum + ((t.quota && t.quota.max_agents) || 0), 0);

  return `
    <div class="grid g4" style="margin-bottom:18px">
      <div class="card metric"><b>${tenants.length}</b><span>total tenants</span></div>
      <div class="card metric"><b>${tenants.filter(t=>t.status==='active').length}</b><span>active</span></div>
      <div class="card metric"><b>${maxAgents}</b><span>max agent capacity</span></div>
      <div class="card metric"><b>${tenants.filter(t=>t.status==='provisioning').length}</b><span>provisioning</span></div>
    </div>

    <div class="card" style="margin-bottom:18px">
      <h3>Tenant registry <span class="tag">Module 01</span></h3>
      <div class="hint">Create, configure, and manage tenant lifecycles with quota enforcement.</div>
      <div class="frow" style="margin-bottom:14px">
        <input id="tenantName" placeholder="tenant name (e.g. acme-corp)">
        <select id="tenantPlan" style="max-width:140px">
          <option selected>saas</option><option>enterprise</option><option>sovereign</option>
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
          <div><label>Max tokens/month</label><input id="quotaTokens" type="number" value="${esc(String(quotas.max_monthly_tokens || 5000000))}"></div>
          <div><label>Max storage (GB)</label><input id="quotaStorage" type="number" value="${esc(String(quotas.max_storage_gb || 20))}"></div>
          <div><label>Max workflows/day</label><input id="quotaWorkflows" type="number" value="${esc(String(quotas.max_workflows_per_day || 1000))}"></div>
        </div>
        <div style="margin-top:12px"><button class="sm" onclick="window.updateQuotas()">Update quotas</button></div>
      </div>
      <div class="card">
        <h3>Resource usage</h3>
        <div class="hint">Live consumption vs quota for this tenant (Module 04 registry).</div>
        <div class="kv">
          <dt>Agents registered</dt><dd>${esc(String(agentsUsed))} / ${esc(String(quotas.max_agents || "—"))}</dd>
        </div>
        ${(quotas.max_agents && agentsUsed >= quotas.max_agents)
          ? `<div class="error-box" style="margin-top:12px"><div class="err-title">Agent quota reached</div><div class="err-msg">This tenant is at its agent limit — raise the quota or retire agents.</div></div>`
          : ""}
      </div>
    </div>`;
}

window.createTenant = async function () {
  const name = $("tenantName").value.trim();
  if (!name) { toast("Tenant name is required", "warn"); return; }
  try {
    // Same payload shape the login flow provisions with — M01 assigns the
    // UUID id; the slug is how workspaces reference the tenant.
    const r = await registerTenant(name, name.toLowerCase().replace(/\s+/g, "-"), $("tenantPlan").value || "medium");
    if (r.ok) { toast("Tenant " + esc(name) + " created", "ok"); window.go("tenants"); }
    else toast("Failed: " + esc(r.data?.detail || r.data?.error?.message || JSON.stringify(r.data || {}).slice(0, 120)), "bad");
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
  if (tokens) quotas.max_monthly_tokens = parseInt(tokens);
  if (storage) quotas.max_storage_gb = parseInt(storage);
  if (workflows) quotas.max_workflows_per_day = parseInt(workflows);
  if (!Object.keys(quotas).length) return;
  if (!window._currentTenantRecordId) { toast("No tenant record for this session tenant", "bad"); return; }
  const r = await patch(SVC.tenant + "/tenants/" + window._currentTenantRecordId, { quota: quotas });
  if (r.ok) { toast("Quotas updated", "ok"); window.go("tenants"); }
  else toast("Failed: " + esc(JSON.stringify(r.data).slice(0, 100)), "bad");
};

function viewError(title, msg) {
  return `<div class="error-box"><div class="err-title">${esc(title)}</div><div class="err-msg">${esc(msg)}</div><button onclick="window.go('tenants')">Retry</button></div>`;
}