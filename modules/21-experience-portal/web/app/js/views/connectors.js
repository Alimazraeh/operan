// Enterprise Connectors (Module 18): connector management, sync, tools, health.
import { SVC, get, post, del, unwrapList } from "../api.js";
import { $, esc, badge, rel, toast, rowItem } from "../ui.js";

// The connector types Module 18 actually accepts (DB-enforced enum).
const CONNECTOR_TYPES = [
  { id: "m365", name: "Microsoft 365", icon: "🟦", desc: "Outlook, SharePoint, Teams, OneDrive" },
  { id: "salesforce", name: "Salesforce", icon: "🔵", desc: "Accounts, Contacts, Opportunities" },
  { id: "sap", name: "SAP", icon: "🔷", desc: "ERP modules: FI, CO, SD, MM" },
  { id: "hubspot", name: "HubSpot", icon: "🟠", desc: "CRM, Marketing, Service Hub" },
  { id: "sharepoint", name: "SharePoint", icon: "📁", desc: "Sites and document libraries" },
  { id: "slack", name: "Slack", icon: "💬", desc: "Channels and messages" },
  { id: "generic_rest", name: "REST API", icon: "🔗", desc: "Custom API endpoint" },
  { id: "smtp", name: "SMTP", icon: "📧", desc: "Email connector" },
  { id: "custom", name: "Custom", icon: "🧩", desc: "Custom integration" },
];

export async function viewConnectors() {
  let connectorsR, syncR, toolsR;
  try {
    [connectorsR, syncR, toolsR] = await Promise.all([
      get(SVC.connectors + "/v1/connectors"),
      get(SVC.connectors + "/v1/sync-history?limit=30"),
      get(SVC.connectors + "/v1/tools"),
    ]);
  } catch (e) { return viewError("Failed to load connector data", e.message); }

  const connectors = unwrapList(connectorsR, "connectors");
  const syncHistory = unwrapList(syncR, "history");
  const tools = unwrapList(toolsR, "tools");

  return `
    <div class="grid g4" style="margin-bottom:18px">
      <div class="card metric"><b>${connectors.length}</b><span>connectors</span></div>
      <div class="card metric"><b>${connectors.filter(c=>c.status==="active").length}</b><span>active</span></div>
      <div class="card metric"><b>${syncHistory.length}</b><span>sync events</span></div>
      <div class="card metric"><b>${tools.length}</b><span>exposed tools</span></div>
    </div>

    <!-- Tab navigation -->
    <div style="display:flex;gap:8px;margin-bottom:18px;flex-wrap:wrap">
      <button class="sm conn-tab active" onclick="window.connTab('connectors', this)">Connectors</button>
      <button class="sm conn-tab" onclick="window.connTab('sync', this)">Sync History</button>
      <button class="sm conn-tab" onclick="window.connTab('tools', this)">Tools</button>
    </div>

    <!-- Connectors tab -->
    <div class="conn-panel" id="panel-connectors">
      <div class="card" style="margin-bottom:18px">
        <h3>Enterprise connectors <span class="tag">Module 18</span></h3>
        <div class="hint">Connect external systems (SAP, Salesforce, M365, etc.) to the Operan platform. Each connector exposes tools that agents can use.</div>
        <div class="frow" style="margin-bottom:14px">
          <select id="connType">
            ${CONNECTOR_TYPES.map(c => `<option value="${esc(c.id)}">${esc(c.icon)} ${esc(c.name)}</option>`).join("")}
          </select>
          <button class="sm" onclick="window.connCreate()">Add connector</button>
        </div>
        ${connectors.length === 0
          ? `<div class="empty">No connectors configured. Add one above.</div>`
          : connectors.map(c => {
              const typeInfo = CONNECTOR_TYPES.find(t => t.id === c.connector_type || t.id === c.type);
              return `<div class="card" style="margin-bottom:12px">
                <div class="frow">
                  <h3 style="flex:1">${esc(typeInfo?.icon || "🔗")}${esc(c.name || c.id.slice(0,8))}</h3>
                  <div class="frow">
                    <button class="sm" onclick="window.connSync('${esc(c.id)}')">Sync now</button>
                    <button class="ghost sm" onclick="window.connHealth('${esc(c.id)}')">Health</button>
                    <button class="bad sm" onclick="window.connDelete('${esc(c.id)}')">Delete</button>
                  </div>
                </div>
                <div class="frow" style="gap:12px;flex-wrap:wrap;margin-top:6px">
                  <span class="hint">${esc(typeInfo?.name || c.type || c.connector_type || "unknown")} connector</span>
                  <span class="hint">${esc(c.status || "inactive")}</span>
                  <span class="hint">last sync: ${rel(c.last_sync_at || c.lastSync || "never")}</span>
                </div>
              </div>`;
            }).join("")}
      </div>
    </div>

    <!-- Sync tab -->
    <div class="conn-panel" id="panel-sync" style="display:none">
      <div class="card">
        <h3>Sync history <span class="tag">Module 18</span></h3>
        <div class="hint">Track all sync operations across connectors — full and incremental.</div>
        ${syncHistory.length === 0
          ? `<div class="empty">No sync events recorded.</div>`
          : syncHistory.map(s => rowItem({
              title: `🔄 ${esc(s.connector_name || s.connector_id?.slice(0,8) || "sync")}`,
              meta: `${esc(s.type || "full")} sync · ${esc(String(s.records_synced || s.records || 0))} records · ${esc(rel(s.started_at || s.created_at || ""))}`,
              badges: badge(s.status || "completed"),
            })).join("")}
      </div>
    </div>

    <!-- Tools tab -->
    <div class="conn-panel" id="panel-tools" style="display:none">
      <div class="card">
        <h3>Connector-exposed tools <span class="tag">Module 18</span></h3>
        <div class="hint">Tools registered by connectors that agents can invoke. Each tool maps to a specific connector action.</div>
        ${tools.length === 0
          ? `<div class="empty">No tools exposed by connectors yet.</div>`
          : `<div style="max-height:500px;overflow:auto">
              <table>
                <thead><tr><th>Tool</th><th>Connector</th><th>Description</th><th>Status</th></tr></thead>
                <tbody>${tools.map(t => `<tr>
                  <td class="mono">${esc(t.name || t.tool_name || "—")}</td>
                  <td>${esc(t.connector_id || t.connector || "—")}</td>
                  <td>${esc(t.description || t.desc || "—")}</td>
                  <td>${badge(t.status || "active")}</td>
                </tr>`).join("")}</tbody>
              </table>
            </div>`}
      </div>
    </div>`;
}

// ── Tab switching (inline handlers survive re-renders) ─────
window.connTab = function (name, btn) {
  document.querySelectorAll(".conn-tab").forEach(b => b.classList.remove("active"));
  btn.classList.add("active");
  document.querySelectorAll(".conn-panel").forEach(p => p.style.display = "none");
  const panel = document.getElementById("panel-" + name);
  if (panel) panel.style.display = "block";
};

// ── Connector CRUD ─────────────────────────────────────────
window.connCreate = async function () {
  const type = $("connType").value;
  const typeInfo = CONNECTOR_TYPES.find(t => t.id === type);
  try {
    const r = await post(SVC.connectors + "/v1/connectors", {
      name: typeInfo?.name || type,
      connector_type: type,
      auth_method: "api_key",
      config: {}, credentials: {},
      sync_frequency: "manual",
    });
    if (r.ok) { toast("Connector " + esc(typeInfo?.name || type) + " added", "ok"); window.go("connectors"); }
    else toast("Failed: " + esc(JSON.stringify(r.data).slice(0, 120)), "bad");
  } catch (e) { toast("Error: " + esc(String(e)), "bad"); }
};

window.connDelete = async function (id) {
  if (!confirm("Delete this connector? Active sync jobs will be cancelled.")) return;
  const r = await del(SVC.connectors + "/v1/connectors/" + encodeURIComponent(id));
  if (r.ok) { toast("Connector deleted", "ok"); window.go("connectors"); }
  else toast("Failed: " + esc(JSON.stringify(r.data).slice(0, 100)), "bad");
};

window.connSync = async function (id) {
  toast("Triggering sync...", "ok");
  try {
    const r = await post(SVC.connectors + "/v1/connectors/" + encodeURIComponent(id) + "/sync");
    if (r.ok) toast("Sync triggered", "ok");
    else toast("Failed: " + esc(JSON.stringify(r.data).slice(0, 100)), "bad");
  } catch (e) { toast("Error: " + esc(String(e)), "bad"); }
};

window.connHealth = async function (id) {
  toast("Checking health...", "ok");
  try {
    const r = await get(SVC.connectors + "/v1/connectors/" + encodeURIComponent(id) + "/health");
    const data = r.data || {};
    const status = data.status || data.health || "unknown";
    const icon = status === "healthy" || status === "active" ? "✅" : "❌";
    toast(icon + " " + esc(String(data.message || status)), status === "healthy" || status === "active" ? "ok" : "bad");
  } catch (e) { toast("Health check failed: " + esc(String(e)), "bad"); }
};

function viewError(title, msg) {
  return `<div class="error-box"><div class="err-title">${esc(title)}</div><div class="err-msg">${esc(msg)}</div><button onclick="window.go('connectors')">Retry</button></div>`;
}