// Settings — Admin settings, API keys, integrations, billing
import { $, esc, card, btn, emptyState, toast } from "../ui.js";
import { listConnectors, createConnector, listPolicies, session, unwrapList, generateAdminPassword } from "../api.js";

export default async function viewSettings() {
  const [connRes, polRes] = await Promise.all([
    listConnectors(1, 20),
    listPolicies(1, 20),
  ]);
  // Fill in the portal build after render — from its own healthz, not a hardcoded claim.
  setTimeout(async () => {
    try {
      const h = await (await fetch("/healthz")).json();
      const el = document.getElementById("setPortalVer");
      if (el) el.textContent = `${h.module || "experience-portal"} v${h.version || "?"} · ${h.status || "?"}`;
    } catch (_) { /* leave the placeholder */ }
  }, 50);

  const connectors = unwrapList(connRes, "connectors");
  const policies = unwrapList(polRes, "policies");

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
            <dt>Portal</dt><dd id="setPortalVer">checking…</dd>
            <dt>Services healthy</dt><dd>${document.querySelectorAll("#healthdots .dot.ok").length}/${document.querySelectorAll("#healthdots .dot").length} <span class="hint" style="margin:0">(header probes)</span></dd>
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
              <div class="m">${esc(p.action || "allow")} · ${esc(p.scope || "all")} · ${esc(p.resource_type || "all")}</div></div>
              <span class="badge ${p.is_active !== false ? 'ok' : 'expired'}">${esc(p.effect || "enforce")}</span>
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
            <option value="sap">SAP</option><option value="sharepoint">SharePoint</option>
            <option value="slack">Slack</option><option value="hubspot">HubSpot</option>
            <option value="generic_rest">REST API</option><option value="smtp">SMTP</option>
            <option value="custom">Custom</option>
          </select>
        </div>
        <div class="form-group"><label>Config (JSON)</label>
          <textarea id="connConfig" placeholder='{"endpoint": "https://...", "auth": "..."}'></textarea></div>
      </div>
      <div class="modal-footer">
        <button class="ghost" onclick="this.closest('.modal-overlay').remove()">Cancel</button>
        <button class="primary" onclick="window.doAddConnector()">Add</button>
      </div>
    </div>`;
  document.body.appendChild(modal);
};

window.doAddConnector = async function () {
  const name = document.getElementById("connName").value.trim();
  const type = document.getElementById("connType").value;
  const configStr = document.getElementById("connConfig").value.trim();
  let config = {};
  if (configStr) { try { config = JSON.parse(configStr); } catch { toast("Invalid JSON config", "warn"); return; } }
  if (!name) { toast("Enter a connector name", "warn"); return; }
  try {
    const r = await createConnector(name, type, config);
    if (!r.ok) throw new Error(r.data?.message || r.data?.error?.message || "status " + r.status);
    toast(`Connector "${esc(name)}" added`, "ok");
    document.querySelector(".modal-overlay").remove();
    window.go("settings");
  } catch (e) { toast(esc(e.message || "Failed to add connector"), "bad"); }
};

window.resetPassword = async function () {
  if (!confirm("This ROTATES the platform admin password for every workspace login. The new password is shown exactly once. Continue?")) return;
  try {
    const data = await generateAdminPassword();
    const modal = document.createElement("div");
    modal.className = "modal-overlay show";
    modal.innerHTML = `<div class="modal"><div class="modal-header"><h3>New admin password</h3></div>
      <div class="modal-body"><p>Store this now — it is not shown again, and the old password no longer works:</p>
      <p><code style="font-family:var(--mono);font-size:14px;user-select:all">${esc(data.password || "(no password in response)")}</code></p></div>
      <div class="modal-footer"><button class="primary" onclick="this.closest('.modal-overlay').remove()">I saved it</button></div></div>`;
    document.body.appendChild(modal);
  } catch (e) { toast("Rotation failed: " + esc(String(e.message || e)), "bad"); }
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