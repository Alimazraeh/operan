// Dashboard — KPI overview at a glance
import { esc, statCard, card } from "../ui.js";
import { unwrapList, listAgents, listDepartments, listWorkflows, listSupervisionQueue, listCostEvents, listPolicies } from "../api.js";

export default async function viewDashboard(page) {
  // Fetch stats in parallel
  const fetches = [
    listAgents(1, 50),
    listDepartments(1, 50),
    listWorkflows(1, 20),
    listSupervisionQueue(1, 10),
    listCostEvents(1, 10),
    listPolicies(1, 20),
  ];
  const results = await Promise.allSettled(fetches);

  // Page items are for the lists below; counts come from the envelope's
  // total (or meta.total) so they aren't capped at one page.
  const items = (r, key) => r.status === "fulfilled" ? unwrapList(r.value, key) : [];
  const total = (r, key) => {
    const d = r.status === "fulfilled" ? r.value?.data : null;
    const t = d?.total ?? d?.meta?.total;
    return typeof t === "number" ? t : items(r, key).length;
  };

  const agentsCount = total(results[0]);
  const departmentsCount = total(results[1]);
  const workflowsCount = total(results[2], "workflows");
  const supervisionCount = total(results[3]);
  const costsCount = total(results[4], "events");
  const policiesCount = total(results[5], "policies");
  const agentsData = items(results[0]);
  const workflowsData = items(results[2], "workflows")
    .slice()
    .sort((a, b) => new Date(b.created_at || 0) - new Date(a.created_at || 0));
  const supervisionData = items(results[3]);
  const policiesData = items(results[5], "policies");
  const unreachable = results.filter(r => r.status !== "fulfilled" || !r.value?.ok).length;

  return `
    <div class="stats-grid">
      ${statCard("🏢", "Departments", departmentsCount, "Deployed departments")}
      ${statCard("👥", "Agents", agentsCount, "Registered agents")}
      ${statCard("⚙️", "Workflows", workflowsCount, "All runs")}
      ${statCard("🧑‍⚖️", "Pending Reviews", supervisionCount, "Awaiting approval")}
      ${statCard("🛡", "Policies", policiesCount, "Governance rules")}
      ${statCard("💰", "Cost Events", costsCount, "All time")}
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
          <dt>Platform Status</dt><dd>${unreachable === 0
            ? `<span class="badge ok">Operational</span>`
            : `<span class="badge degraded">Degraded — ${unreachable}/${results.length} services unreachable</span>`}</dd>
          <dt>Agents Online</dt><dd>${agentsData.filter(a => a.status === "active").length}</dd>
          <dt>Workflows Running</dt><dd>${workflowsData.filter(w => w.status === "running" || w.status === "in_progress").length}</dd>
          <dt>Policies Active</dt><dd>${policiesData.filter(p => p.is_active !== false).length}</dd>
          <dt>Departments</dt><dd>${departmentsCount} deployed</dd>
        </div>
      </div>
    `)}
  `;
}

function rowWorkflow(w) {
  const badges = `<span class="badge ${esc(w.status || 'draft')}">${esc(w.status || "draft")}</span>`;
  return `<div class="row-item" onclick="window.go('workflows')">
    <div class="grow"><div class="t">${esc(w.name || "Untitled")}</div>
    <div class="m">${esc(w.status || "draft")} · ${esc((w.completed_at || w.started_at || w.created_at || "").slice(0,10))}</div></div>
    <div class="actions">${badges}</div>
  </div>`;
}

function rowSupervision(s) {
  const badge = `<span class="badge ${esc(s.status || 'pending')}">${esc(s.status || "pending")}</span>`;
  return `<div class="row-item" onclick="window.go('supervision')">
    <div class="grow"><div class="t">${esc(s.title || s.subject || "Supervision Request")}</div>
    <div class="m">${esc((s.created_at || "").slice(0,10))} · ${esc(s.item_type || "approval")} · ${esc(s.priority || "medium")}</div></div>
    <div class="actions">${badge}</div>
  </div>`;
}