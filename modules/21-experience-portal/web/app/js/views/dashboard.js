// Dashboard — KPI overview at a glance
import { $, esc, statCard, card, btn } from "../ui.js";
import { get, listAgents, listTemplates, listWorkflows, listSupervisionQueue, listCostEvents, listPolicies } from "../api.js";

export default async function viewDashboard(page) {
  // Fetch stats in parallel
  const fetches = [
    listAgents(1, 50),
    listTemplates(1, 10),
    listWorkflows(1, 20),
    listSupervisionQueue(1, 10),
    listCostEvents(1, 10),
    listPolicies(1, 20),
  ];
  const results = await Promise.allSettled(fetches);

  function items(r) {
    if (r.status !== 'fulfilled') return [];
    const d = r.value?.data;
    return d?.items || d || [];
  }

  const agentsCount = items(results[0]).length;
  const templatesCount = items(results[1]).length;
  const workflowsCount = items(results[2]).length;
  const supervisionCount = items(results[3]).length;
  const costsCount = items(results[4]).length;
  const policiesCount = items(results[5]).length;
  const agentsData = items(results[0]);
  const workflowsData = items(results[2]);
  const supervisionData = items(results[3]);
  const policiesData = items(results[5]);

  return `
    <div class="stats-grid">
      ${statCard("🏢", "Departments", templatesCount, "Templates available")}
      ${statCard("👥", "Agents", agentsCount, "Registered agents")}
      ${statCard("⚙️", "Workflows", workflowsCount, "Active workflows")}
      ${statCard("🧑‍⚖️", "Pending Reviews", supervisionCount, "Awaiting approval")}
      ${statCard("🛡", "Policies", policiesCount, "Governance rules")}
      ${statCard("💰", "Cost Events", costsCount, "Today")}
    </div>

    <div class="two-col">
      ${card("Recent Workflows", "", `
        <div class="card-body">
          ${workflowsData.length > 0
            ? workflowsData.slice(0, 5).map(w => rowWorkflow(w)).join("")
            : "<div class='empty'>No workflows yet</div>"}
        </div>
      `)}
      ${card("Pending Supervision", "", `
        <div class="card-body">
          ${supervisionData.length > 0
            ? supervisionData.slice(0, 5).map(s => rowSupervision(s)).join("")
            : "<div class='empty'>All clear — no pending approvals</div>"}
        </div>
      `)}
    </div>

    ${card("Operational Summary", "High-level KPIs", `
      <div class="card-body">
        <div class="kv">
          <dt>Platform Status</dt><dd><span class="badge ok">Operational</span></dd>
          <dt>Agents Online</dt><dd>${agentsCount}</dd>
          <dt>Workflows Running</dt><dd>${workflowsData.filter(w => w.status === "running" || w.status === "in_progress").length}</dd>
          <dt>Policies Active</dt><dd>${policiesData.filter(p => p.is_active !== false).length || policiesCount}</dd>
          <dt>Department Templates</dt><dd>${templatesCount} available</dd>
        </div>
      </div>
    `)}
  `;
}

function rowWorkflow(w) {
  const badges = `<span class="badge ${esc(w.status || 'draft')}">${esc(w.status || "draft")}</span>`;
  return `<div class="row-item" onclick="window.go('workflows')">
    <div class="grow"><div class="t">${esc(w.name || "Untitled")}</div>
    <div class="m">${esc(w.status || "draft")} · ${esc((w.updated_at || w.created_at || "").slice(0,10))}</div></div>
    <div class="actions">${badges}</div>
  </div>`;
}

function rowSupervision(s) {
  const badge = `<span class="badge ${esc(s.status || 'pending')}">${esc(s.status || "pending")}</span>`;
  return `<div class="row-item" onclick="window.go('supervision')">
    <div class="grow"><div class="t">${esc(s.title || s.subject || "Supervision Request")}</div>
    <div class="m">${esc((s.created_at || "").slice(0,10))} · ${esc(s.requester || "system")}</div></div>
    <div class="actions">${badge}</div>
  </div>`;
}