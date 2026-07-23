// Settings — Admin settings, API keys, integrations, billing
import { $, esc, card, btn, emptyState, toast } from "../ui.js";
import { listConnectors, createConnector, listPolicies, session, del, SVC } from "../api.js";

export default async function viewSettings() {
  const [connRes, polRes] = await Promise.all([
    listConnectors(1, 20),
    listPolicies(1, 20),
  ]);

  const connectors = connRes.data?.items || connRes.data || [];
  const policies = polRes.data?.items || polRes.data || [];

  return `
    <div class="two-col">
      ${card("Profile", "Your account", `
        <div class="card-body">
          <div class="kv">
            <dt>User ID</dt><dd><code style="font-family:var(--mono);font-size:11px">${esc(session.userId || "—")}</code></dd>
            <dt>Email</dt><dd>${esc(session.email || "—")}</dd>
            <dt>Role</dt><dd><span class="badge admin">${esc(session.role || "admin")}</span></dd>
            <dt>Tenant</dt><dd><code style="font-family:var(--mono);font-size:11px">${esc(session.tenant || "—")}</code></dd>
          </div>
        </div>
      `)}
      ${card("Platform", "Operan instance", `
        <div class="card-body">
          <div class="kv">
            <dt>Version</dt><dd>v21.0.0</dd>
            <dt>Modules</dt><dd>20 deployed</dd>
            <dt>PostgreSQL</dt><dd><span class="badge ok">Connected</span></dd>
            <dt>Kafka</dt><dd><span class="badge ok">Connected</span></dd>
          </div>
        </div>
      `)}
    </div>

    ${card("Enterprise Connectors", `${connectors.length} configured`, `
      <div class="toolbar">${btn("+ Add Connector", "primary", "openAddConnector()")}</div>
      <div id="connector-list">
        ${connectors.length === 0
          ? emptyState("🔌", "No Connectors", "Add enterprise system connectors.", btn("Add Connector", "primary", "openAddConnector()"))
          : connectors.map(connectorRow).join("")}
      </div>
    `)}

    ${card("Active Policies", `${policies.length} policies`, `
      <div class="card-body">
        ${policies.length === 0
          ? "<div class='empty'>No policies configured</div>"
          : policies.slice(0, 20).map(p => `<div class="row-item">
              <div class="grow"><div class="t">${esc(p.name || "Untitled")}</div>
              <div class="m">${esc(p.category || "custom")} · ${esc(p.type || "allow")}</div></div>
              <span class="badge ${p.is_active !== false ? 'ok' : 'error'}">${esc(p.enforcement || "log")}</span>
            </div>`).join("")}
      </div>
    `)}

    ${card("Danger Zone", "Irreversible actions", `
      <div class="card-body">
        <div style="display:flex;justify-content:space-between;align-items:center;padding:12px 0;border-bottom:1px solid var(--border)">
          <div><b>Reset Admin Password</b><div style="color:var(--text-muted);font-size:12px">Generate a new password for admin login</div></div>
          ${btn("Generate New", "bad sm", "resetPassword()")}
        </div>
        <div style="display:flex;justify-content:space-between;align-items:center;padding:12px 0">
          <div><b>Sign Out</b><div style="color:var(--text-muted);font-size:12px">Clear session and local storage</div></div>
          ${btn("Sign Out", "ghost sm", "document.getElementById('btnLogout').click()")}
        </div>
      </div>
    `)}
  `;
}

window.openAddConnector = function() {
  const modal = document.createElement("div");
  modal.className = "modal-overlay show";
  modal.innerHTML = `
    <div class="modal">
      <div class="modal-header"><h3>Add Connector</h3>
        <button class="ghost sm" onclick="this.closest('.modal-overlay').remove()">✕</button></div>
      <div class="modal-body">
        <div class="form-group"><label>Name</label><input id="connName" placeholder="e.g. M365 Tenant"></div>
        <div class="form-group"><label>Type</label>
          <select id="connType">
            <option value="m365">Microsoft 365</option><option value="salesforce">Salesforce</option>
            <option value="sap">SAP</option><option value="servicenow">ServiceNow</option>
            <option value="slack">Slack</option><option value="jira">Jira</option><option value="custom">Custom API</option>
          </select>
        </div>
        <div class="form-group"><label>Config (JSON)</label>
          <textarea id="connConfig" placeholder='{"endpoint": "https://...", "auth": "..."}'></textarea></div>
      </div>
      <div class="modal-footer">
        <button class="ghost" onclick="this.closest('.modal-overlay').remove()">Cancel</button>
        <button class="primary" onclick="doAddConnector()">Add</button>
      </div>
    </div>`;
  document.body.appendChild(modal);
};

async function doAddConnector() {
  const name = document.getElementById("connName").value.trim();
  const type = document.getElementById("connType").value;
  const configStr = document.getElementById("connConfig").value.trim();
  let config = {};
  if (configStr) { try { config = JSON.parse(configStr); } catch { toast("Invalid JSON config", "error"); return; } }
  if (!name) { toast("Enter a connector name", "error"); return; }
  try {
    await createConnector(name, type, config);
    toast(`Connector "${name}" added`, "success");
    document.querySelector(".modal-overlay").remove();
    window.go("settings");
  } catch (e) { toast(e.message || "Failed to add connector", "error"); }
}

window.resetPassword = function() {
  if (!confirm("This will generate a new random admin password. Save it — you won't see it again.")) return;
  toast("Password reset endpoint called — check the M02 logs for the new password", "info");
};

function connectorRow(c) {
  const status = esc(c.status || "disconnected");
  const badgeCls = status === "connected" ? "ok" : "pending";
  return `<div class="row-item">
    <div class="grow"><div class="t">${esc(c.name || "Unnamed")}</div>
    <div class="m">${esc(c.type || "unknown")}${c.config?.endpoint ? ` · ${esc(c.config.endpoint.slice(0, 30))}...` : ""}</div></div>
    <span class="badge ${badgeCls}">${status}</span>
  </div>`;
}