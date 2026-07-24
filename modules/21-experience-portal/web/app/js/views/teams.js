// Teams — the workforce view of the operating model.
//
// A team in Operan is not a flat list of bots: each department deploys an
// org chart of POSITIONS (seats) with reporting lines, autonomy tiers,
// decision rights and approval gates. Seats are held by AI agents (M04),
// humans (M02), or sit vacant. This page shows who holds which seat, what
// authority each seat carries, what its holder has actually worked on, and
// where humans decide. Hiring here registers a real M04 agent into a real
// department — org seats themselves are provisioned by the template deploy.
import { esc, rel, toast, statCard, badge } from "../ui.js";
import {
  unwrapList, listDepartments, getDeptOrgChart, listAgents,
  listWorkflows, listHumanTasks, createAgent,
} from "../api.js";

const TIER_LABEL = { recommend: "recommend", draft: "draft", execute: "execute", coordinate: "coordinate" };

export default async function viewTeams() {
  // ── Gather: departments + org charts, M04 roster, work evidence ──
  const [deptR, agentsR, runsR, tasksR] = await Promise.allSettled([
    listDepartments(1, 50),
    listAgents(1, 100),
    listWorkflows(1, 50),
    listHumanTasks(),
  ]);
  const depts = deptR.status === "fulfilled" ? unwrapList(deptR.value) : [];
  const agents = agentsR.status === "fulfilled" ? unwrapList(agentsR.value) : [];
  const runs = runsR.status === "fulfilled" ? unwrapList(runsR.value, "workflows") : [];
  const tasks = tasksR.status === "fulfilled" ? unwrapList(tasksR.value, "tasks") : [];

  // Workforce = the org charts of LIVE departments; archived deploys are
  // history, not seats. Their leftover M04 agents surface under direct hires.
  const liveDepts = depts.filter(d => d.status === "operational" || d.status === "degraded");
  const charts = (await Promise.allSettled(liveDepts.map(d => getDeptOrgChart(d.id))))
    .map((r, i) => ({ dept: liveDepts[i], positions: r.status === "fulfilled" && r.value.ok ? (r.value.data?.positions || []) : [] }));

  // ── Joins: seat → live agent; agent → run involvement; human deciders ──
  const agentById = {};
  agents.forEach(a => { agentById[a.id] = a; });

  const involvement = {}; // agent_id → { count, last }
  for (const run of runs) {
    for (const n of run.graph?.nodes || []) {
      if (!n.agent_id) continue;
      const t = run.completed_at || run.started_at || run.created_at;
      const e = involvement[n.agent_id] = involvement[n.agent_id] || { count: 0, last: "" };
      e.count++;
      if (t && t > e.last) e.last = t;
    }
  }
  const deciders = {}; // user → { count, last }
  for (const t of tasks) {
    if (!t.responded_by) continue;
    const e = deciders[t.responded_by] = deciders[t.responded_by] || { count: 0, last: "" };
    e.count++;
    const ts = t.responded_at || t.created_at || "";
    if (ts > e.last) e.last = ts;
  }

  // ── Seat accounting ──
  const allPositions = charts.flatMap(c => c.positions);
  const boundSeats = allPositions.filter(p => p.holder_type === "ai_agent" && agentById[p.agent_id]).length;
  const unboundSeats = allPositions.filter(p => p.holder_type === "ai_agent" && p.agent_id && !agentById[p.agent_id]).length;
  const humanSeats = allPositions.filter(p => p.holder_type === "human").length;
  const vacantSeats = allPositions.filter(p => p.holder_type === "vacant" || (!p.agent_id && p.holder_type !== "human")).length;
  const seatAgentIds = new Set(allPositions.map(p => p.agent_id).filter(Boolean));
  const liveIds = new Set(liveDepts.map(d => d.id));
  const directHires = agents.filter(a => !seatAgentIds.has(a.id))
    .sort((a, b) => (liveIds.has(b.department_id) - liveIds.has(a.department_id)) || (a.name || "").localeCompare(b.name || ""));
  const activeAgents = agents.filter(a => a.status === "active").length;
  const decisions = Object.values(deciders).reduce((s, d) => s + d.count, 0);

  window._teamDepts = depts.filter(d => d.status === "operational" || d.status === "degraded");

  return `<div id="teamsRoot">
    <div class="stats-grid">
      ${statCard("🪑", "Org seats", allPositions.length, `${boundSeats} agent-held · ${humanSeats} human · ${vacantSeats + unboundSeats} vacant/unbound`)}
      ${statCard("🤖", "AI agents", agents.length, `${activeAgents} active in the registry`)}
      ${statCard("🧑", "Human seats", humanSeats, humanSeats ? "Bound to platform users" : "None in these org charts")}
      ${statCard("🧑‍⚖️", "Human decisions", decisions, Object.keys(deciders).length ? `by ${Object.keys(deciders).length} person(s)` : "No gates decided yet")}
      ${statCard("🏢", "Departments staffed", charts.filter(c => c.positions.length).length, `${depts.length - liveDepts.length ? depts.length - liveDepts.length + " archived excluded" : "all live"}`)}
    </div>

    <div class="card" style="margin-bottom:18px">
      <h3>Org charts <span class="tag">who holds which seat</span></h3>
      <div class="hint">Seats, reporting lines and authority come from the department template at deploy.
      Autonomy tiers bound what a seat may do alone; <b>decision rights</b> and <b>gates</b> mark where its
      holder signs off. Click an agent-held seat to open the agent.</div>
      <div class="toolbar">
        <div class="search"><input id="teamSearch" placeholder="Filter seats and agents…" oninput="window.filterTeam(this.value)"></div>
        <button class="primary" onclick="window.openHireModal()">+ Hire agent</button>
      </div>
      ${charts.filter(c => c.positions.length).map(c => orgChartHtml(c, agentById, involvement)).join("")
        || `<div class="empty">No org charts yet — <a href="#" onclick="window.go('departments');return false">deploy a department</a> to provision its seats.</div>`}
    </div>

    <div class="grid g2">
      <div class="card">
        <h3>Direct hires <span class="tag">agents outside the org charts</span></h3>
        <div class="hint">Registered in M04 but not holding a provisioned seat. They can still be assigned
        workflow steps by ID.</div>
        ${directHires.length === 0
          ? `<div class="empty">Everyone on the roster holds a seat.</div>`
          : directHires.map(a => directHireHtml(a, involvement, depts)).join("")}
      </div>
      <div class="card">
        <h3>Human deciders <span class="tag">gates answered</span></h3>
        <div class="hint">People who approved or rejected agent work at control points. Today these are
        platform users; org charts can also bind human seats directly.</div>
        ${Object.keys(deciders).length === 0
          ? `<div class="empty">No human decisions recorded yet.</div>`
          : Object.entries(deciders).map(([who, d]) => `
            <div class="row-item" data-name="${esc(who.toLowerCase())}">
              <div class="grow"><div class="t">🧑 ${esc(who)}</div>
              <div class="m">${d.count} gate decision(s) · last ${rel(d.last)}</div></div>
              <div class="actions"><button class="ghost sm" onclick="window.go('supervision')">Queue</button></div>
            </div>`).join("")}
      </div>
    </div>
  </div>`;
}

// ── Org chart per department: reporting tree from reports_to ─
function orgChartHtml({ dept, positions }, agentById, involvement) {
  const children = {};
  let roots = [];
  positions.forEach(p => {
    if (p.reports_to) (children[p.reports_to] = children[p.reports_to] || []).push(p);
    else roots.push(p);
  });
  if (roots.length === 0) roots = positions.slice(0, 1); // defensive: cyclic/odd data
  const rows = [];
  const walk = (p, depth) => {
    rows.push(seatHtml(p, depth, agentById, involvement));
    (children[p.id] || []).forEach(c => walk(c, depth + 1));
  };
  roots.forEach(r => walk(r, 0));
  return `<div class="team-dept">
    <div class="team-dept-h">${esc(dept.name)} ${badge(dept.status || "operational")}
      <span class="hint" style="margin:0 0 0 8px">${positions.length} seat(s)</span></div>
    ${rows.join("")}
  </div>`;
}

function seatHtml(p, depth, agentById, involvement) {
  const holder = p.holder_type === "ai_agent" ? agentById[p.agent_id] : null;
  const unbound = p.holder_type === "ai_agent" && p.agent_id && !holder;
  const icon = p.holder_type === "human" ? "🧑" : p.holder_type === "ai_agent" ? "🤖" : "🪑";
  const who = p.holder_type === "human" ? (p.human_ref || "human (unassigned)")
    : holder ? holder.name
    : unbound ? "agent missing from registry"
    : "vacant";
  const act = holder && involvement[holder.id];
  const rights = (p.decision_rights || [])
    .map(r => `${r.decision} (${r.authority}${r.limit ? " ≤ " + r.limit : ""})`).join(" · ");
  const searchKey = `${p.title} ${who} ${p.role_type || ""}`.toLowerCase();
  return `<div class="row-item team-seat" data-name="${esc(searchKey)}" style="margin-left:${depth * 26}px${unbound ? ";opacity:.75" : ""}"
      ${holder ? `onclick="window.go('agent','${esc(holder.id)}')"` : ""}>
    <div class="grow">
      <div class="t">${icon} ${esc(p.title)} <span class="tag">${esc(p.role_type || "seat")}</span>
        ${p.unit ? `<span class="tag">${esc(p.unit)}</span>` : ""}
        ${p.autonomy_tier ? `<span class="wfstep ${p.autonomy_tier === "coordinate" || p.autonomy_tier === "execute" ? "agent" : ""}" style="font-size:10px;padding:1px 7px">${esc(TIER_LABEL[p.autonomy_tier] || p.autonomy_tier)}</span>` : ""}
        ${(p.approval_gate_refs || []).length ? `<span class="wfstep human_gate" style="font-size:10px;padding:1px 7px">🚧 holds gate</span>` : ""}</div>
      <div class="m">${unbound ? "⚠️ " : ""}${esc(who)}${act ? ` · ${act.count} workflow step(s) · last active ${rel(act.last)}` : ""}${rights ? ` · decides: ${esc(rights)}` : ""}${p.escalates_to ? ` · escalates to ${esc(p.escalates_to)}` : ""}</div>
    </div>
    <div class="actions"><span class="badge ${holder ? esc(holder.status || "active") : unbound ? "expired" : p.holder_type === "human" ? "active" : "draft"}">${holder ? esc(holder.status || "active") : unbound ? "unbound" : esc(p.holder_type || "vacant")}</span></div>
  </div>`;
}

function directHireHtml(a, involvement, depts) {
  const act = involvement[a.id];
  const dept = depts.find(d => d.id === a.department_id);
  const archived = dept && dept.status !== "operational" && dept.status !== "degraded";
  const caps = (a.capabilities || []).slice(0, 4)
    .map(c => esc(typeof c === "string" ? c : c.capability || c.name || "")).filter(Boolean).join(", ");
  return `<div class="row-item" data-name="${esc(`${a.name} ${a.role}`.toLowerCase())}" onclick="window.go('agent','${esc(a.id)}')"${archived ? ' style="opacity:.65"' : ""}>
    <div class="grow">
      <div class="t">🤖 ${esc(a.name || "Unnamed")}</div>
      <div class="m">${esc(a.role || "agent")}${dept ? ` · ${esc(dept.name)}${archived ? " (department " + esc(dept.status) + ")" : ""}` : a.department_id ? ` · dept ${esc(a.department_id.slice(0, 8))} (unknown)` : ""}${caps ? ` · ${caps}` : ""}${act ? ` · ${act.count} workflow step(s)` : ""}</div>
    </div>
    <div class="actions"><span class="badge ${esc(a.status || "active")}">${esc(a.status || "active")}</span></div>
  </div>`;
}

// ── Search filter across seats, direct hires and deciders ──
window.filterTeam = function (query) {
  const q = (query || "").toLowerCase();
  document.querySelectorAll("#teamsRoot [data-name]").forEach(row => {
    row.style.display = (row.dataset.name || "").includes(q) ? "" : "none";
  });
};

// ── Hire flow: a real M04 registration into a real department ─
window.openHireModal = function () {
  const depts = window._teamDepts || [];
  const modal = document.createElement("div");
  modal.className = "modal-overlay show";
  modal.innerHTML = `
    <div class="modal">
      <div class="modal-header"><h3>Hire an agent</h3>
        <button class="ghost sm" onclick="this.closest('.modal-overlay').remove()">✕</button></div>
      <div class="modal-body">
        <div class="form-group"><label>Agent name</label><input id="agentName" placeholder="e.g. Contracts Reviewer"></div>
        <div class="form-group"><label>Role</label>
          <select id="agentRole">
            <option value="analyst">Analyst</option><option value="executor">Executor</option>
            <option value="manager">Manager</option><option value="qa">QA Reviewer</option>
            <option value="compliance">Compliance Agent</option><option value="custom">Custom</option>
          </select>
        </div>
        <div class="form-group" id="customRoleGroup" style="display:none"><label>Custom role</label><input id="agentCustomRole"></div>
        <div class="form-group"><label>Capabilities (comma-separated)</label><input id="agentCaps" placeholder="doc_review, contract_analysis"></div>
        <div class="form-group"><label>Department</label>
          <select id="agentDept">
            <option value="">— unassigned (registry only) —</option>
            ${depts.map(d => `<option value="${esc(d.id)}">${esc(d.name)}</option>`).join("")}
          </select>
          <div class="hint">Hiring joins the department's roster. Org-chart seats are provisioned by the
          template at deploy — they are not created here.</div>
        </div>
      </div>
      <div class="modal-footer">
        <button class="ghost" onclick="this.closest('.modal-overlay').remove()">Cancel</button>
        <button class="primary" onclick="window.doHire()">Hire agent</button>
      </div>
    </div>`;
  document.body.appendChild(modal);
  modal.querySelector("#agentRole").addEventListener("change", (e) => {
    document.getElementById("customRoleGroup").style.display = e.target.value === "custom" ? "" : "none";
  });
};

window.doHire = async function () {
  const name = document.getElementById("agentName").value.trim();
  let role = document.getElementById("agentRole").value;
  if (role === "custom") role = document.getElementById("agentCustomRole").value.trim() || "agent";
  const capsStr = document.getElementById("agentCaps").value.trim();
  const deptId = document.getElementById("agentDept").value;
  if (!name) { toast("Give the agent a name", "warn"); return; }
  const capabilities = capsStr ? capsStr.split(",").map(s => s.trim()).filter(Boolean) : [role];
  try {
    const r = await createAgent(name, role, capabilities, deptId || "");
    if (!r.ok) throw new Error(r.data?.detail || r.data?.error?.message || "status " + r.status);
    toast(esc(name) + " registered" + (deptId ? " into the department roster" : " (unassigned)"), "ok");
    document.querySelector(".modal-overlay")?.remove();
    window.go("teams");
  } catch (e) {
    toast("Hiring failed: " + esc(e.message), "bad");
  }
};
