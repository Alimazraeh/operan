// Dashboard — KPI overview at a glance
import { esc, rel, statCard, card } from "../ui.js";
import { get, unwrapList, listAgents, listDepartments, listWorkflows, listSupervisionQueue, listCostEvents, listPolicies, listDeptRequests } from "../api.js";

const OPEN_REQUEST_STATUSES = ["submitted", "dispatched", "in_progress", "awaiting_approval"];

export default async function viewDashboard(page) {
  // Fetch stats in parallel
  const fetches = [
    listAgents(1, 50),
    listDepartments(1, 50),
    listWorkflows(1, 20),
    listSupervisionQueue(1, 10),
    listCostEvents(1, 10),
    listPolicies(1, 20),
    get("/svc/templates/briefings?limit=1"),
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
  const briefing = items(results[6])[0] || null;
  const unreachable = results.slice(0, 6).filter(r => r.status !== "fulfilled" || !r.value?.ok).length;

  // Open requests across live departments — the demand the operation owes.
  const liveDepts = items(results[1]).filter(d => d.status === "operational" || d.status === "degraded");
  const reqLists = await Promise.allSettled(liveDepts.map(d => listDeptRequests(d.id)));
  const openRequests = reqLists.reduce((n, r) =>
    n + (r.status === "fulfilled" && r.value.ok
      ? unwrapList(r.value).filter(q => OPEN_REQUEST_STATUSES.includes(q.status)).length : 0), 0);

  return `
    <div class="stats-grid">
      ${statCard("🏢", "Departments", departmentsCount, "Deployed departments")}
      ${statCard("📨", "Open Requests", openRequests, openRequests ? "Being worked now" : "Queue is clear")}
      ${statCard("👥", "Agents", agentsCount, "Registered agents")}
      ${statCard("⚙️", "Workflows", workflowsCount, "All runs")}
      ${statCard("🧑‍⚖️", "Pending Reviews", supervisionCount, "Awaiting approval")}
      ${statCard("🛡", "Policies", policiesCount, "Governance rules")}
    </div>

    ${briefing ? card("Latest briefing", `${esc(briefing.department_name || "")} · ${esc(briefing.cadence_name || "")} · ${rel(briefing.created_at)}`, `
      <div class="card-body">
        <div class="m" style="white-space:pre-wrap;line-height:1.65">${esc(String(briefing.content || "").slice(0, 1200))}${String(briefing.content || "").length > 1200 ? "…" : ""}</div>
        <div style="margin-top:10px;display:flex;gap:6px;flex-wrap:wrap">
          <span class="sla-chip">${esc(String(briefing.stats?.open_requests ?? 0))} open</span>
          <span class="sla-chip ${briefing.stats?.awaiting_approval ? "warn" : ""}">${esc(String(briefing.stats?.awaiting_approval ?? 0))} awaiting approval</span>
          <span class="sla-chip ${briefing.stats?.sla_breached ? "bad" : "ok"}">${esc(String(briefing.stats?.sla_breached ?? 0))} SLA breached</span>
          <span class="sla-chip ok">${esc(String(briefing.stats?.completed_last_24h ?? 0))} completed 24h</span>
          ${briefing.model ? `<span class="sla-chip">${esc(briefing.model)} · ${esc(String(briefing.tokens || 0))} tokens</span>` : ""}
          <button class="ghost sm" style="margin-left:auto" onclick="window.go('tasks')">Open the ledger →</button>
        </div>
      </div>
    `) : ""}

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