// Departments: real Module 05 catalog → server-orchestrated deploy pipeline →
// department detail with the full operating model (org chart, chain of
// command, service portfolio, value chain, risk/quality, compliance).
import { SVC, get, post, del, uuid4, listTemplates, listDepartments, getDepartment,
         getDeptOrgChart, deployTemplateReal, getDeployment } from "../api.js";
import { $, esc, badge, rel, toast, rowItem } from "../ui.js";

export const STAGES = ["select", "configure", "connect_data", "provision_memory", "deploy_swarm", "deploy_workflows", "operational"];
const STAGE_LABELS = {
  select: "Selected", configure: "Validated", connect_data: "Data connected",
  provision_memory: "Memory provisioned", deploy_swarm: "Swarm deployed",
  deploy_workflows: "SOPs compiled", operational: "Operational",
};
const CATEGORY_META = {
  "it":                   { icon: "🖥", label: "Information Technology" },
  "it-operations":        { icon: "📡", label: "IT Operations" },
  "ops":                  { icon: "📡", label: "IT Operations (Unified)" },
  "hr":                   { icon: "🧑‍💼", label: "Human Resources" },
  "finance":              { icon: "📊", label: "Finance" },
  "legal":                { icon: "⚖️", label: "Legal" },
  "compliance":           { icon: "🛡", label: "Compliance" },
  "engineering":          { icon: "⚙️", label: "Engineering" },
  "software-development": { icon: "💻", label: "Software Development" },
  "marketing":            { icon: "📣", label: "Marketing" },
  "procurement":          { icon: "🧾", label: "Procurement" },
};
const TIER_LABEL = { recommend: "Recommend", analyze: "Analyze", coordinate: "Coordinate", draft: "Draft", execute: "Execute (gated)" };

let templates = [];

// ── Departments home: live instances + the real catalog ─────
export async function viewDepartments() {
  const [tr, dr] = await Promise.all([listTemplates(1, 100), listDepartments(1, 50)]);
  templates = (tr.data && tr.data.data) || [];
  const departments = (dr.data && dr.data.data) || [];

  // Group catalog by category; one card per category with size choices.
  const byCat = {};
  for (const t of templates) {
    (byCat[t.category] = byCat[t.category] || []).push(t);
  }
  const sizeOrder = { small: 0, medium: 1, large: 2, unified: 3 };

  const catalogCards = Object.entries(byCat).map(([cat, list]) => {
    const meta = CATEGORY_META[cat] || { icon: "🏢", label: cat };
    list.sort((a, b) => (sizeOrder[sizeOf(a)] ?? 9) - (sizeOrder[sizeOf(b)] ?? 9));
    const flagship = ["it", "it-operations", "ops"].includes(cat);
    const sizes = list.map(t =>
      `<button class="sm ${flagship ? "" : "ghost"}" onclick="window.deployDept('${t.id}')"
        title="${esc(t.name)} — ${t.agents_count || 0} agents">${esc(sizeOf(t))} · ${t.agents_count || 0} agents</button>`).join(" ");
    return `<div class="tmpl${flagship ? " flagship" : ""}">
      <h4>${meta.icon} ${esc(meta.label)}${flagship ? ' <span class="tag">flagship</span>' : ""}</h4>
      <div class="d">${esc((list[0].description || "").slice(0, 140))}</div>
      <div class="foot" style="flex-wrap:wrap;gap:6px">${sizes}</div>
    </div>`;
  }).join("");

  return `
    <div class="card" style="margin-bottom:18px">
      <h3>Your departments <span class="tag">Module 05</span></h3>
      <div class="hint">Living organizational units: org chart, chain of command, service portfolio, governed agents.</div>
      ${departments.length === 0
        ? `<div class="empty">No departments yet — deploy one from the catalog below.</div>`
        : departments.map(d => rowItem({
            title: d.name,
            meta: `${d.category} · ${d.environment || "production"} · ${d.positions_count || 0} positions · ${d.services_count || 0} services · ${d.agents_count || 0} agents · ${d.risks_count || 0} risks · created ${rel(d.created_at)}`,
            badges: d.status,
            onClick: `window.go('department','${d.id}')`,
          })).join("")}
    </div>

    <div class="card">
      <h3>Department catalog <span class="tag">Module 05 · ${templates.length} templates</span></h3>
      <div class="hint">Complete operating models: agents, org charts, SOPs, service portfolios, governance, KPIs — pick a size, deploy in one click.</div>
      <div class="grid g3">${catalogCards || '<div class="empty">Catalog loading… refresh in a moment.</div>'}</div>
    </div>`;
}

function sizeOf(t) {
  return (t.tags || []).find(x => ["small", "medium", "large", "unified"].includes(x))
    || (t.name.toLowerCase().includes("small") ? "small"
      : t.name.toLowerCase().includes("medium") ? "medium"
      : t.name.toLowerCase().includes("large") ? "large"
      : t.name.toLowerCase().includes("unified") ? "unified" : "standard");
}

// ── Deploy: kick off + poll the server-side pipeline ────────
window.deployDept = async function (templateId) {
  const t = templates.find(x => x.id === templateId);
  if (!t) return;
  const view = $("view");
  const stagesHTML = STAGES.map((s, i) =>
    `<div class="stg" id="dstg${i}"><div class="b">${i + 1}</div><div class="l">${STAGE_LABELS[s]}</div></div>`).join("");
  view.innerHTML = `<div class="card"><h3>Deploying ${esc(t.name)}</h3>
    <div class="hint">Module 05 orchestrates the pipeline server-side: validation, memory provisioning (M07), agent registration (M04).</div>
    <div class="stages">${stagesHTML}</div><div id="deploylog"></div></div>`;

  const log = (msg) => {
    const el = $("deploylog");
    if (el) el.insertAdjacentHTML("beforeend",
      `<div class="ev"><div class="ico orchestration">›</div><div class="what">${msg}</div><time>${new Date().toLocaleTimeString()}</time></div>`);
  };
  const mark = (i, cls) => { const el = $("dstg" + i); if (el) el.className = "stg " + cls; };

  try {
    const dep = await deployTemplateReal(templateId);
    if (!dep.ok) {
      log(`<span style="color:var(--bad)">Deploy rejected: ${esc(JSON.stringify(dep.data).slice(0, 160))}</span>`);
      toast("Deployment failed to start", "bad");
      return;
    }
    const depId = dep.data.id;
    const departmentId = dep.data.department_id;
    log(`Deployment <b>${esc(depId.slice(0, 8))}</b> started — department <b>${esc((departmentId || "").slice(0, 8))}</b> materializing`);

    const seen = new Set();
    for (let i = 0; i < 80; i++) {
      const r = await getDeployment(templateId, depId);
      const d = r.data || {};
      for (const s of (d.stages || [])) {
        const idx = STAGES.indexOf(s.stage);
        if (idx >= 0) mark(idx, s.status === "completed" ? "ok" : s.status === "failed" ? "bad" : "on");
        const key = s.stage + ":" + s.status;
        if (!seen.has(key) && s.status !== "running") {
          seen.add(key);
          log(`${STAGE_LABELS[s.stage] || s.stage}: ${esc(s.detail || s.status)}`);
        }
      }
      if (d.status === "operational") {
        log(`<b>${esc(t.name)} is operational.</b>`);
        toast(`${esc(t.name)} deployed`, "ok");
        await new Promise(res => setTimeout(res, 800));
        window.go("department", departmentId);
        return;
      }
      if (d.status === "failed") {
        log(`<span style="color:var(--bad)">Provisioning failed: ${esc(d.error_message || "unknown")}</span>`);
        toast("Deployment failed", "bad");
        return;
      }
      await new Promise(res => setTimeout(res, 1500));
    }
    log(`<span style="color:var(--warn)">Still provisioning — check back on the departments page.</span>`);
  } catch (e) {
    log(`<span style="color:var(--bad)">Error: ${esc(String(e))}</span>`);
    toast("Deployment failed", "bad");
  }
};

// ── Department detail: the operating model, tabbed ──────────

// ── Human position binding: resolve human_ref against Module 02 users ──
let _iamUsers = null;
async function loadIamUsers() {
  if (_iamUsers !== null) return _iamUsers;
  try {
    const r = await get(SVC.iam + "/users");
    const items = (r.data && (r.data.items || r.data.users)) || (Array.isArray(r.data) ? r.data : []);
    _iamUsers = Array.isArray(items) ? items : [];
  } catch (_) { _iamUsers = []; }
  return _iamUsers;
}
function resolveHuman(ref) {
  if (!ref || !_iamUsers) return null;
  const norm = String(ref).toLowerCase();
  return _iamUsers.find(u =>
    String(u.id || "").toLowerCase() === norm ||
    String(u.username || "").toLowerCase() === norm ||
    String(u.email || "").toLowerCase() === norm ||
    String(u.display_name || "").toLowerCase() === norm) || null;
}

export async function viewDepartment(departmentId) {
  let dRes, orgRes, agentsRes;
  try {
    loadIamUsers(); // warm the M02 user cache for human-position binding
    [dRes, orgRes, agentsRes] = await Promise.all([
      getDepartment(departmentId),
      getDeptOrgChart(departmentId),
      get(SVC.registry + "/registry/agents?page_size=100"),
    ]);
  } catch (e) { return viewError("Failed to load department", String(e)); }
  if (!dRes.ok || !dRes.data) return viewError("Department not found", departmentId);

  const d = dRes.data;
  const org = orgRes.data || { positions: [], edges: [], root_position_id: "" };
  const allAgents = (agentsRes.data && agentsRes.data.items) || [];
  const staff = allAgents.filter(a => a.department_id === d.id);
  const posByAgentId = {};
  for (const p of (d.org_chart || [])) if (p.agent_id) posByAgentId[p.agent_id] = p;

  const lead = staff.find(a => (posByAgentId[a.id] || {}).id === org.root_position_id) || staff[0];

  const tabs = [
    ["overview", "Overview"], ["org", "Org Chart"], ["services", "Services"],
    ["governance", "Governance"], ["risk", "Risk & Quality"], ["kpis", "KPIs"], ["staff", `Staff (${staff.length})`],
  ];
  const tabBar = tabs.map(([id, label], i) =>
    `<button class="tab-btn${i === 0 ? " active" : ""}" data-tab="${id}" onclick="window.deptTab('${id}')">${label}</button>`).join("");

  return `
    <span class="back" onclick="window.go('departments')">← All departments</span>
    <div class="card" style="margin-bottom:14px">
      <h3>${esc(d.name)} ${badge(d.status)} <span class="tag">${esc(d.category)}</span>
        ${d.status !== "archived" ? `<button class="ghost sm" style="float:right" onclick="window.archiveDept('${esc(d.id)}')">Archive department</button>` : ""}</h3>
      <div class="hint">${esc(d.mission || d.description || "")}</div>
      <div class="kv">
        <dt>Positions</dt><dd>${(d.org_chart || []).length}</dd>
        <dt>Services</dt><dd>${(d.services || []).length}</dd>
        <dt>Agents</dt><dd>${staff.length}</dd>
        <dt>Environment</dt><dd>${esc(d.environment || "production")}</dd>
        <dt>Created</dt><dd>${rel(d.created_at)}</dd>
      </div>
    </div>
    <div class="tab-bar">${tabBar}</div>
    <div class="tab-panel active" data-panel="overview">${tabOverview(d)}</div>
    <div class="tab-panel" data-panel="org">${tabOrg(d, org, staff)}</div>
    <div class="tab-panel" data-panel="services">${tabServices(d)}</div>
    <div class="tab-panel" data-panel="governance">${tabGovernance(d)}</div>
    <div class="tab-panel" data-panel="risk">${tabRisk(d)}</div>
    <div class="tab-panel" data-panel="kpis">${tabKPIs(d)}</div>
    <div class="tab-panel" data-panel="staff">${tabStaff(d, staff, lead)}</div>`;
}

window.deptTab = function (id) {
  document.querySelectorAll(".tab-btn").forEach(b => b.classList.toggle("active", b.dataset.tab === id));
  document.querySelectorAll(".tab-panel").forEach(p => p.classList.toggle("active", p.dataset.panel === id));
};

// ── Tab: Overview — business logic + value chain ────────────
function tabOverview(d) {
  const bl = d.business_logic || {};
  const cadence = (bl.operating_cadence || []).map(c =>
    rowItem({ title: "🗓 " + c.name, meta: c.description || "", badges: c.frequency })).join("");

  const kpiById = {};
  for (const k of (d.kpis || [])) kpiById[k.id] = k;

  const streams = (d.value_streams || []).map(vs => {
    const stages = (vs.stages || []).map((s, i) => `
      <div class="vs-stage">
        <div class="vs-num">${i + 1}</div>
        <div class="vs-body">
          <b>${esc(s.name)}</b>
          <div class="vs-io"><span class="vs-in">⇢ ${(s.inputs || []).map(esc).join(", ") || "—"}</span>
          <span class="vs-out">⇒ ${(s.outputs || []).map(esc).join(", ") || "—"}</span></div>
        </div>
      </div>`).join('<div class="vs-arrow">→</div>');
    const metrics = (vs.value_metric_kpi_refs || []).map(id => kpiById[id] ? `<span class="tag">📈 ${esc(kpiById[id].name)}</span>` : "").join(" ");
    return `<div class="card" style="margin-bottom:12px">
      <h4>${esc(vs.name)}</h4>
      <div class="hint">${esc(vs.description || "")}</div>
      <div class="vs-track">${stages}</div>
      <div class="vs-outcome"><b>Outcome:</b> ${esc(vs.outcome || "—")}<br><b>Business value:</b> ${esc(vs.business_outcome || "—")} ${metrics}</div>
    </div>`;
  }).join("");

  return `
    <div class="grid g2" style="margin-bottom:14px">
      <div class="card">
        <h3>Why this department exists</h3>
        <div class="hint">${esc(bl.purpose || d.mission || "—")}</div>
        <div style="margin-top:8px"><b>Value proposition</b><div class="hint">${esc(bl.value_proposition || "—")}</div></div>
        <div style="margin-top:8px"><b>Stakeholders</b><div class="chips">${(bl.stakeholders || []).map(s => `<span>${esc(s)}</span>`).join("") || "—"}</div></div>
      </div>
      <div class="card">
        <h3>Operating cadence</h3>
        <div class="hint">The rituals that keep the department accountable.</div>
        ${cadence || '<div class="empty">No cadence defined.</div>'}
      </div>
    </div>
    <h3 style="margin:14px 0 8px">Value chain — how work becomes value</h3>
    ${streams || '<div class="empty">No value streams defined for this department.</div>'}`;
}

// ── Tab: Org chart — tree + chain of command ────────────────
function tabOrg(d, org, staff) {
  const positions = d.org_chart || [];
  if (positions.length === 0) return '<div class="empty">No org chart defined.</div>';
  const byId = {}; const children = {};
  for (const p of positions) { byId[p.id] = p; children[p.id] = []; }
  let rootId = org.root_position_id;
  for (const p of positions) {
    if (p.reports_to && children[p.reports_to]) children[p.reports_to].push(p.id);
    else if (!p.reports_to && !rootId) rootId = p.id;
  }
  const agentName = {};
  for (const a of staff) agentName[a.id] = a.name;

  const node = (id) => {
    const p = byId[id];
    if (!p) return "";
    const iamu = p.holder_type === "human" ? resolveHuman(p.human_ref) : null;
    const holder = p.holder_type === "human"
      ? (iamu ? `🧑 ${esc(iamu.display_name || iamu.username || iamu.email || p.human_ref)} <span class="tag ok">IAM</span>`
              : `🧑 ${esc(p.human_ref || "human")} <span class="tag warn" title="No matching Module 02 user">unbound</span>`)
      : p.agent_id && agentName[p.agent_id] ? `🤖 <a onclick="window.go('agent','${p.agent_id}')" style="cursor:pointer;text-decoration:underline">${esc(agentName[p.agent_id])}</a>`
      : p.holder_type === "vacant" ? "◌ vacant" : "🤖 " + esc(p.agent_def_id || "agent");
    const rights = (p.decision_rights || []).map(r =>
      `<div class="dr"><span class="dr-a ${esc(r.authority)}">${esc(r.authority)}</span> ${esc(r.decision)}${r.limit ? ` <i>(${esc(r.limit)})</i>` : ""}</div>`).join("");
    const gates = (p.approval_gate_refs || []).length ? `<div class="dr"><span class="dr-a gate">M09 gate</span> ${p.approval_gate_refs.map(esc).join(", ")}</div>` : "";
    return `<li>
      <div class="org-node">
        <div class="org-title">${esc(p.title)} ${p.unit ? `<span class="tag">${esc(p.unit)}</span>` : ""}</div>
        <div class="org-holder">${holder}</div>
        <div class="org-meta">${esc(p.role_type)} · autonomy: <b>${TIER_LABEL[p.autonomy_tier] || esc(p.autonomy_tier || "—")}</b></div>
        ${rights}${gates}
      </div>
      ${children[id].length ? `<ul>${children[id].map(node).join("")}</ul>` : ""}
    </li>`;
  };

  return `<div class="card">
    <h3>Organization & chain of command</h3>
    <div class="hint">Reporting lines are real: they drive escalation rules on every registered agent, and the root position escalates to a human via Module 09 approval gates. Execute-tier autonomy is always gated.</div>
    <ul class="org-tree">${rootId ? node(rootId) : positions.map(p => node(p.id)).join("")}</ul>
  </div>`;
}

// ── Tab: Services — the portfolio with SLAs ─────────────────
function tabServices(d) {
  const services = d.services || [];
  if (services.length === 0) return '<div class="empty">No service portfolio defined.</div>';
  const posById = {};
  for (const p of (d.org_chart || [])) posById[p.id] = p;
  const kpiById = {};
  for (const k of (d.kpis || [])) kpiById[k.id] = k;

  const rows = services.map(s => {
    const owner = posById[s.owner_position_id];
    const sla = s.sla || {};
    const kpis = (s.kpi_refs || []).map(id => kpiById[id] ? `<span class="tag">${esc(kpiById[id].name)}</span>` : "").join(" ");
    return `<tr>
      <td><b>${esc(s.name)}</b><div class="hint" style="font-size:11px">${esc(s.description || "")}</div></td>
      <td>${owner ? esc(owner.title) : "—"}</td>
      <td>${(s.consumers || []).map(esc).join(", ") || "—"}</td>
      <td class="mono" style="font-size:11px">${esc(sla.response_time || "—")}<br>${esc(sla.resolution_time || "")}${sla.coverage ? `<br><span class="tag">${esc(sla.coverage)}</span>` : ""}</td>
      <td>${kpis || "—"}</td>
    </tr>`;
  }).join("");

  return `<div class="card">
    <h3>Service portfolio <span class="tag">${services.length} services</span></h3>
    <div class="hint">What this department delivers, to whom, at what service level — each service delivered by a workflow and measured by KPIs.</div>
    <div style="overflow-x:auto"><table class="svc-table">
      <thead><tr><th>Service</th><th>Owner</th><th>Consumers</th><th>SLA</th><th>Measured by</th></tr></thead>
      <tbody>${rows}</tbody>
    </table></div>
  </div>`;
}

// ── Tab: Governance & compliance ────────────────────────────
function tabGovernance(d) {
  const controls = d.compliance_controls || [];
  const frameworks = {};
  for (const c of controls) (frameworks[c.framework] = frameworks[c.framework] || []).push(c);
  const fwBlocks = Object.entries(frameworks).map(([fw, list]) => `
    <div class="card" style="margin-bottom:12px">
      <h4>🛡 ${esc(fw)} <span class="tag">${list.length} controls</span></h4>
      ${list.map(c => rowItem({
        title: c.control_id ? `${c.name} [${c.control_id}]` : c.name,
        meta: c.description || "",
        badges: c.status || "implemented",
      })).join("")}
    </div>`).join("");

  const rules = (d.governance_rules || []).map(g => rowItem({
    title: "⚙️ " + g.name,
    meta: g.description || g.type || "",
    badges: g.enforcement || "enforce",
  })).join("");

  return `
    ${fwBlocks || '<div class="empty">No compliance controls mapped.</div>'}
    <div class="card">
      <h3>Governance rules</h3>
      <div class="hint">The enforced rules that implement the controls above.</div>
      ${rules || '<div class="empty">No governance rules.</div>'}
    </div>`;
}

// ── Tab: Risk & quality ─────────────────────────────────────
function tabRisk(d) {
  const sevColor = { low: "ok", medium: "warn", high: "bad", critical: "bad" };
  const risks = (d.risks || []).map(r => `
    <div class="risk-row">
      <span class="risk-sev ${sevColor[r.severity] || ""}">${esc(r.severity)}</span>
      <div class="risk-body">
        <b>${esc(r.name)}</b> <span class="tag">${esc(r.category || "")}</span> <span class="tag">${esc(r.likelihood)}</span>
        <div class="hint" style="font-size:11px">${esc(r.description || "")}</div>
        <div class="hint" style="font-size:11px"><b>Mitigation:</b> ${esc(r.mitigation || "—")}</div>
      </div>
      ${badge(r.status || "open")}
    </div>`).join("");

  const kpiById = {};
  for (const k of (d.kpis || [])) kpiById[k.id] = k;
  const quality = (d.quality_standards || []).map(q => rowItem({
    title: "✅ " + q.name,
    meta: `Target: ${q.target}${q.measure_kpi_ref && kpiById[q.measure_kpi_ref] ? " · measured by " + kpiById[q.measure_kpi_ref].name : ""}`,
    badges: q.type || "slo",
  })).join("");

  return `
    <div class="card" style="margin-bottom:14px">
      <h3>Risk register <span class="tag">${(d.risks || []).length} risks</span></h3>
      <div class="hint">Severity × likelihood, each with an owner and a mitigation tied to platform controls.</div>
      ${risks || '<div class="empty">No risks recorded.</div>'}
    </div>
    <div class="card">
      <h3>Quality standards</h3>
      <div class="hint">The measurable bars this department is held to.</div>
      ${quality || '<div class="empty">No quality standards defined.</div>'}
    </div>`;
}

// ── Tab: KPIs ───────────────────────────────────────────────
function tabKPIs(d) {
  const kpis = (d.kpis || []).map(k => {
    const th = k.thresholds || {};
    const parts = [];
    if (th.target !== undefined) parts.push("target " + th.target);
    if (th.target_label) parts.push(String(th.target_label));
    if (th.warning !== undefined) parts.push("warn " + th.warning);
    if (th.critical !== undefined) parts.push("crit " + th.critical);
    return rowItem({
      title: "📈 " + k.name,
      meta: `${k.metric_type}${k.unit ? " · " + k.unit : ""}${parts.length ? " · " + parts.join(" / ") : ""}${k.aggregation_period ? " · per " + k.aggregation_period : ""}`,
    });
  }).join("");
  return `<div class="card">
    <h3>KPI definitions <span class="tag">${(d.kpis || []).length}</span></h3>
    <div class="hint">Measured through Module 11 observability; thresholds drive dashboards and alerts.</div>
    ${kpis || '<div class="empty">No KPIs defined.</div>'}
  </div>`;
}

// ── Tab: Staff + give-work card ─────────────────────────────
function tabStaff(d, staff, lead) {
  const posByAgentId = {};
  for (const p of (d.org_chart || [])) if (p.agent_id) posByAgentId[p.agent_id] = p;

  const cards = staff.length === 0
    ? `<div class="empty">No agents registered for this department yet.</div>`
    : staff.map(a => {
        const p = posByAgentId[a.id];
        return rowItem({
          title: "🤖 " + a.name + (p ? ` — ${p.title}` : ""),
          meta: `${a.role}${p && p.unit ? " · " + p.unit : ""} · autonomy: ${p ? (p.autonomy_tier || "—") : "—"}`,
          badges: a.status || "active",
          onClick: `window.go('agent','${a.id}')`,
        });
      }).join("");

  const taskCard = !lead ? "" : `
    <div class="card" style="margin-top:14px">
      <h3>Give the department real work <span class="tag">Module 03 + LLM</span></h3>
      <div class="hint">Hand <b>${esc(lead.name)}</b> a task. It drafts with the LLM, grounded in memory, then routes to you for sign-off per the chain of command.</div>
      <textarea id="taskInstr" rows="2">Draft a concise incident report (4 sentences max) for this morning's 22-minute email outage, including root cause and follow-up actions.</textarea>
      <div style="margin-top:10px"><button class="sm" onclick="window.agentDoWork('${esc(lead.id)}','${esc(lead.name)}','${esc(lead.role || "agent")}','${esc(d.id)}')">Let the agent work</button></div>
      <div id="workOut"></div>
    </div>`;

  return `<div class="card"><h3>Staff <span class="tag">Module 04</span></h3>
    <div class="hint">Each agent holds a position in the org chart; its autonomy tier and escalation path were registered from the operating model.</div>
    ${cards}</div>${taskCard}`;
}

// ── Agent work flow (unchanged behavior) ────────────────────
window.archiveDept = async function (id) {
  if (!confirm("Archive this department? Its agents stay registered in Module 04.")) return;
  const r = await del("/svc/templates/departments/" + id);
  if (r.ok) { toast("Department archived", "ok"); window.go("departments"); }
  else toast("Archive failed: " + esc(JSON.stringify(r.data).slice(0, 100)), "bad");
};

window.agentDoWork = async function (agentId, agentName, role, departmentId) {
  const instruction = $("taskInstr").value.trim();
  if (!instruction) return;
  $("workOut").innerHTML = `<div class="hint" style="margin-top:10px"><span class="pulse-dot"></span>${esc(agentName)} is reasoning…</div>`;
  try {
    const draft = await post(SVC.orchestration + "/agent/draft", {
      agent_id: agentId, role, instruction, department_id: departmentId,
      memory_query: "department services and preferences",
    });
    if (!draft.ok || !draft.data?.output) {
      $("workOut").innerHTML = `<div class="error-box"><div class="err-title">Agent reasoning failed</div><div class="err-msg">${esc(JSON.stringify(draft.data?.error?.message || draft.data).slice(0, 200))}</div></div>`;
      return;
    }
    const text = draft.data.output;
    // Real chain: pipeline definition → execution → human task on that
    // execution → supervision approval referencing the task.
    const pipe = await post(SVC.orchestration + "/pipeline", {
      name: "agent-work", steps: [{ id: "s1", name: "agent-work", type: "agent" }, { id: "s2", name: "human-signoff", type: "human_gate" }],
    });
    if (!pipe.ok) throw new Error("pipeline create failed: " + JSON.stringify(pipe.data).slice(0, 120));
    const exec = await post(SVC.orchestration + "/executions", { pipeline_id: pipe.data.id });
    if (!exec.ok) throw new Error("execution start failed: " + JSON.stringify(exec.data).slice(0, 120));
    const task = await post(SVC.orchestration + "/human-tasks", {
      pipeline_execution_id: exec.data.id, step_id: "s2", assignee_id: "manager", instructions: text,
    });
    if (!task.ok) throw new Error("human task failed: " + JSON.stringify(task.data).slice(0, 120));
    const appr = await post(SVC.supervision + "/approvals", { request_id: task.data.id, requester_id: agentId, type: "parallel", title: instruction.slice(0, 70) });
    if (!appr.ok) throw new Error("approval gate failed: " + JSON.stringify(appr.data).slice(0, 120));
    $("workOut").innerHTML = `<div class="result"><div class="q">${esc(agentName)} · drafted by ${esc(draft.data.model)} · grounded in ${draft.data.memory_used.length} memor${draft.data.memory_used.length === 1 ? "y" : "ies"}</div><div class="a" style="white-space:pre-wrap">${esc(text)}</div><div class="meta"><span>${draft.data.tokens} tokens</span><span>awaiting sign-off in Supervision →</span></div></div>`;
    toast("Agent produced real work — sign off in Supervision", "ok");
  } catch (e) {
    $("workOut").innerHTML = `<div class="error-box"><div class="err-title">Error</div><div class="err-msg">${esc(String(e))}</div></div>`;
  }
};

function viewError(title, msg) {
  return `<div class="error-box"><div class="err-title">${esc(title)}</div><div class="err-msg">${esc(msg)}</div><button onclick="window.go('departments')">Back</button></div>`;
}