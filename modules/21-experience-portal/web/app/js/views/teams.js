// Teams — Agent roster, roles, hiring flow
import { $, esc, statCard, card, btn, emptyState, toast } from "../ui.js";
import { listAgents, createAgent, uuid4, unwrapList } from "../api.js";

export default async function viewTeams() {
  const res = await listAgents(1, 100);
  const agents = unwrapList(res, "agents");
  const active = agents.filter(a => a.status === "active").length;
  const totalCaps = agents.reduce((sum, a) => sum + (a.capabilities?.length || 0), 0);

  return `
    <div class="stats-grid">
      ${statCard("👥", "Total Agents", agents.length, "All registered")}
      ${statCard("🟢", "Active", active, "Currently running")}
      ${statCard("📋", "Capabilities", totalCaps, "Across all agents")}
    </div>
    ${card("Agent Roster", `${agents.length} agents`, `
      <div class="toolbar">
        <div class="search"><input id="teamSearch" placeholder="Search agents..." oninput="filterAgents(this.value)"></div>
        ${btn("+ Hire Agent", "primary", "openHireModal()")}</div>
      <div id="agent-list">
        ${agents.length === 0
          ? emptyState("👥", "No Agents Hired Yet",
              "Deploy a department first, or hire agents manually.",
              btn("Deploy Department", "primary", "window.go('departments')") +
              btn("Hire Manually", "ghost", "openHireModal()"))
          : agents.map(agentRow).join("")}
      </div>
    `)}
  `;
}

function agentRow(a) {
  const caps = (a.capabilities || []).map(c =>
    `<span class="badge">${esc(c.capability || c.name || "unknown")}</span>`
  ).join("");
  return `<div class="row-item" data-name="${esc((a.name||"").toLowerCase())}">
    <div class="grow">
      <div class="t">${esc(a.name||"Unnamed Agent")}</div>
      <div class="m">${esc(a.role||"agent")} ${a.department_id?"· "+esc(a.department_id.slice(0,8)):""} ${a.version?"· v"+esc(a.version):""}</div>
      <div style="margin-top:6px">${caps}</div>
    </div>
    <div class="actions">
      <span class="badge ${esc(a.status||"active")}">${esc(a.status||"active")}</span>
      ${btn("Hire", "ghost sm", `hireNew("${esc(a.role||"agent")}", "${esc(a.department_id||"")}")`)}
    </div>
  </div>`;
}

window.openHireModal = function() {
  const modal = document.createElement("div");
  modal.className = "modal-overlay show";
  modal.innerHTML = `
    <div class="modal">
      <div class="modal-header"><h3>Hire New Agent</h3>
        <button class="ghost sm" onclick="this.closest('.modal-overlay').remove()">✕</button></div>
      <div class="modal-body">
        <div class="form-group"><label>Agent Name</label><input id="agentName" placeholder="e.g. Contracts Reviewer"></div>
        <div class="form-group"><label>Role</label>
          <select id="agentRole">
            <option value="analyst">Analyst</option><option value="executor">Executor</option>
            <option value="manager">Manager</option><option value="qa">QA Reviewer</option>
            <option value="compliance">Compliance Agent</option><option value="custom">Custom</option>
          </select>
        </div>
        <div class="form-group" id="customRoleGroup" style="display:none"><label>Custom Role</label><input id="agentCustomRole"></div>
        <div class="form-group"><label>Capabilities (comma-separated)</label><input id="agentCaps" placeholder="doc_review, contract_analysis"></div>
        <div class="form-group"><label>Department</label><select id="agentDept"><option value="">— No department —</option></select></div>
      </div>
      <div class="modal-footer">
        <button class="ghost" onclick="this.closest('.modal-overlay').remove()">Cancel</button>
        <button class="primary" onclick="doHire()">Hire Agent</button>
      </div>
    </div>`;
  document.body.appendChild(modal);
  modal.querySelector("#agentRole").addEventListener("change", (e) => {
    document.getElementById("customRoleGroup").style.display = e.target.value === "custom" ? "" : "none";
  });
};

async function doHire() {
  const name = document.getElementById("agentName").value.trim();
  let role = document.getElementById("agentRole").value;
  if (role === "custom") role = document.getElementById("agentCustomRole").value.trim();
  const capsStr = document.getElementById("agentCaps").value.trim();
  const deptId = document.getElementById("agentDept").value;
  if (!name) { toast("Enter an agent name", "error"); return; }
  const capabilities = capsStr ? capsStr.split(",").map(s=>({capability:s.trim(),score:80})) : [{capability:role,score:80}];
  try {
    await createAgent(name, role, capabilities, deptId || uuid4());
    toast(name + " hired as " + role, "success");
    document.querySelector(".modal-overlay").remove();
    window.go("teams");
  } catch (e) { toast(e.message || "Hiring failed", "error"); }
}

window.hireNew = function(role, deptId) {
  openHireModal();
  document.getElementById("agentName").value = "New " + role.charAt(0).toUpperCase() + role.slice(1);
  document.getElementById("agentRole").value = role;
  if (deptId) document.getElementById("agentDept").value = deptId;
};

function filterAgents(query) {
  const q = query.toLowerCase();
  document.querySelectorAll("#agent-list .row-item").forEach(row => {
    row.style.display = (row.dataset.name||"").includes(q) ? "" : "none";
  });
}