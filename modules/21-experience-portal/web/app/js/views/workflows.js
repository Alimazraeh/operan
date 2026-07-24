// Workflows — the automation ledger of the business.
//
// A workflow here is not a database row: it is a department service's SOP,
// compiled at deploy into an executable DAG (Module 03), instantiated per
// service request by the work loop (Module 05), with agents doing the steps
// and humans holding the gates (Module 09). This page shows all three layers:
//   1. The automated business logic — which services run on which SOPs.
//   2. Runs — every time that logic actually executed, and where it is now.
//   3. Control points — where the automation waited for a human decision.
import { esc, rel, toast, statCard } from "../ui.js";
import {
  unwrapList, listDepartments, getDepartment, getTemplate,
  listWorkflows, listHumanTasks, createServiceRequest,
} from "../api.js";

// View-scoped state for inline handlers (re-set on every render).
window._wf = window._wf || { runs: {}, depts: {}, open: {} };

export async function viewWorkflows() {
  clearInterval(window._wfPoll);

  // ── Gather: departments → details → SOP templates; runs; human tasks ──
  const [deptListR, runsR, tasksR] = await Promise.allSettled([
    listDepartments(1, 50),
    listWorkflows(1, 50),
    listHumanTasks(),
  ]);

  const deptSummaries = deptListR.status === "fulfilled" ? unwrapList(deptListR.value) : [];
  const deptDetails = (await Promise.allSettled(deptSummaries.map(d => getDepartment(d.id))))
    .filter(r => r.status === "fulfilled" && r.value.ok).map(r => r.value.data);

  const templateIds = [...new Set(deptDetails.map(d => d.template_id).filter(Boolean))];
  const templates = {};
  (await Promise.allSettled(templateIds.map(id => getTemplate(id))))
    .filter(r => r.status === "fulfilled" && r.value.ok)
    .forEach((r, i) => { const t = r.value.data?.data || r.value.data; if (t) templates[templateIds[i]] = t; });

  const runs = (runsR.status === "fulfilled" ? unwrapList(runsR.value, "workflows") : [])
    .slice().sort((a, b) => new Date(b.created_at || 0) - new Date(a.created_at || 0));
  const tasks = (tasksR.status === "fulfilled" ? unwrapList(tasksR.value, "tasks") : [])
    .slice().sort((a, b) => new Date(b.created_at || 0) - new Date(a.created_at || 0));

  // ── Join: service → SOP definition on the department's template ──
  const deptById = {};
  deptDetails.forEach(d => { deptById[d.id] = d; });
  const sopRows = [];
  for (const dept of deptDetails) {
    if (dept.status !== "operational" && dept.status !== "degraded") continue;
    for (const svc of dept.services || []) {
      const tmpl = templates[dept.template_id];
      const sop = svc.delivery_workflow_id && tmpl
        ? (tmpl.workflows || []).find(w => w.id === svc.delivery_workflow_id) : null;
      sopRows.push({ dept, svc, sop });
    }
  }

  // ── Stats over the real joined data ──
  const allSteps = sopRows.flatMap(r => r.sop ? r.sop.steps || [] : []);
  const gateSteps = allSteps.filter(s => stepKind(s.type) === "human_gate").length;
  const agentSteps = allSteps.filter(s => stepKind(s.type) === "agent").length;
  const running = runs.filter(r => r.status === "running" || r.status === "in_progress");
  const gated = running.filter(r => currentNode(r)?.type === "human_gate");
  const pendingTasks = tasks.filter(t => t.status === "pending");

  window._wf = { runs: {}, depts: deptById, open: window._wf.open || {} };
  runs.forEach(r => { window._wf.runs[r.id] = r; });

  // Live page while anything is executing: re-render every 12s unless the
  // user has a form or a run detail open (their state would be wiped).
  if (running.length > 0) {
    window._wfPoll = setInterval(() => {
      const root = document.getElementById("wfRoot");
      if (!root) { clearInterval(window._wfPoll); return; }
      const formOpen = [...root.querySelectorAll(".wf-form")].some(f => f.style.display !== "none");
      const detailOpen = Object.values(window._wf.open).some(Boolean);
      if (!formOpen && !detailOpen) window.go("workflows");
    }, 12000);
  }

  // ── Render ──
  return `<div id="wfRoot">
    <div class="stats-grid">
      ${statCard("📘", "Automated services", sopRows.filter(r => r.sop).length, "Running on a compiled SOP")}
      ${statCard("🤖", "Agent steps", agentSteps, "Executed without people")}
      ${statCard("🧑‍⚖️", "Control points", gateSteps, "Where humans hold the gate")}
      ${statCard("▶️", "Runs", runs.length, running.length ? `${running.length} running now` : "All settled")}
      ${statCard("⏸", "Waiting on people", pendingTasks.length, gated.length ? `${gated.length} run(s) gated` : "Nothing blocked")}
    </div>

    <div class="card" style="margin-bottom:18px">
      <h3>Automated business logic <span class="tag">SOP library</span></h3>
      <div class="hint">Each department service is delivered by a standard operating procedure, compiled into an
      executable workflow at deploy. Agents do the typed steps; <b>approval</b> steps stop the line for a human.
      Submitting a request runs the SOP for real.</div>
      ${sopRows.length === 0
        ? `<div class="empty">No operational departments yet — <a href="#" onclick="window.go('departments');return false">deploy a department</a> to install its SOPs.</div>`
        : sopRows.map(r => sopRowHtml(r)).join("")}
    </div>

    <div class="card" style="margin-bottom:18px">
      <h3>Runs <span class="tag">every execution of the logic above</span></h3>
      <div class="hint">One run per service request — the work loop compiles the SOP with the request's context,
      agents draft the work, gates wait in <a href="#" onclick="window.go('supervision');return false">Supervision</a>.
      <button class="sm ghost" style="margin-left:8px" onclick="window.go('workflows')">↻ Refresh</button></div>
      ${runs.length === 0
        ? `<div class="empty">No runs yet — trigger one from the SOP library above.</div>`
        : runs.slice(0, 20).map(r => runRowHtml(r, deptById)).join("")}
    </div>

    <div class="card">
      <h3>Human control points <span class="tag">where automation waited for people</span></h3>
      <div class="hint">Every gate a run has raised, and who decided it. Pending gates are answered from the
      Supervision queue.</div>
      ${tasks.length === 0
        ? `<div class="empty">No control points raised yet.</div>`
        : tasks.slice(0, 12).map(t => taskRowHtml(t)).join("")}
    </div>
  </div>`;
}

// ── SOP library rows ────────────────────────────────────────
function sopRowHtml({ dept, svc, sop }) {
  const key = `${dept.id}::${svc.id}`;
  const sla = svc.sla ? [svc.sla.response_time && `response ${svc.sla.response_time}`,
                         svc.sla.resolution_time && `resolution ${svc.sla.resolution_time}`]
                        .filter(Boolean).join(" · ") : "";
  const chain = sop
    ? stepChain((sop.steps || []).map(s => ({ kind: stepKind(s.type), label: s.name || s.id })))
    : stepChain([{ kind: "human_gate", label: "Manual handling (fallback gate)" }]);
  return `<div class="wf-sop">
    <div class="row-item" style="cursor:default">
      <div class="grow">
        <div class="t">${esc(svc.name)} <span class="tag">${esc(dept.name)}</span></div>
        <div class="m">${sop ? esc(sop.name) : "No SOP on the template — the work loop falls back to a single approval gate"}${sla ? " · " + esc(sla) : ""}</div>
        <div style="margin-top:8px">${chain}</div>
      </div>
      <div class="actions"><button class="sm" onclick="window.wfOpenRun('${esc(key)}')">Run</button></div>
    </div>
    <div class="wf-form" id="wfForm-${esc(key)}" style="display:none">
      <div class="frow">
        <input id="wfTitle-${esc(key)}" placeholder="What do you need? (becomes the request title)" style="flex:2">
        <select id="wfPrio-${esc(key)}"><option>P3</option><option>P1</option><option>P2</option><option>P4</option></select>
        <button class="sm" onclick="window.wfSubmitRun('${esc(dept.id)}','${esc(svc.id)}','${esc(key)}')">Submit request</button>
        <button class="sm ghost" onclick="window.wfCloseRun('${esc(key)}')">Cancel</button>
      </div>
      <textarea id="wfBody-${esc(key)}" rows="2" placeholder="Details the agents should work from (optional)"></textarea>
    </div>
  </div>`;
}

// ── Run rows ────────────────────────────────────────────────
function runRowHtml(run, deptById) {
  const nodes = orderedNodes(run);
  const states = nodeStates(run, nodes);
  const done = states.filter(s => s === "done").length;
  const cur = currentNode(run);
  const isRunning = run.status === "running" || run.status === "in_progress";
  const gateWait = isRunning && cur?.type === "human_gate";
  const v = run.variables || {};
  const title = v.request_title || run.name || "Untitled run";
  const dept = deptById[run.department_id || v.department_id];
  const dur = duration(run);
  const isOpen = !!window._wf.open[run.id];

  return `<div class="row-item wf-run" onclick="window.wfToggleRun('${esc(run.id)}')">
    <div class="grow">
      <div class="t">${esc(title)} ${v.priority ? `<span class="tag">${esc(v.priority)}</span>` : ""}${dept ? ` <span class="tag">${esc(dept.name)}</span>` : ""}</div>
      <div class="m">${run.status === "completed" ? "completed" : "started"} ${rel(run.completed_at || run.started_at || run.created_at)}
        ${dur ? " · took " + dur : ""} · step ${Math.min(done + 1, nodes.length)}/${nodes.length}
        ${gateWait ? ` · <span style="color:var(--gate)">⏸ waiting on ${esc(cur.action || "approval")}</span>` : ""}
        ${isRunning && cur && !gateWait ? ` · at ${esc(cur.action || cur.id)}` : ""}</div>
      <div class="wf-run-detail" style="display:${isOpen ? "block" : "none"}" onclick="event.stopPropagation()">
        <div style="margin:10px 0 8px">${stepChain(nodes.map((n, i) => ({
          kind: n.type, label: n.action || n.id, state: states[i],
        })))}</div>
        ${v.request_body ? `<div class="hint" style="margin:0 0 6px">Request: ${esc(String(v.request_body).slice(0, 280))}${String(v.request_body).length > 280 ? "…" : ""}</div>` : ""}
        ${gateWait ? `<button class="sm" onclick="window.go('supervision')">Answer the gate in Supervision</button>` : ""}
      </div>
    </div>
    <div class="actions"><span class="badge ${esc(run.status || "draft")}">${esc(run.status || "draft")}</span></div>
  </div>`;
}

// ── Control point rows ──────────────────────────────────────
function taskRowHtml(t) {
  const line = (t.label || t.instructions || "gate").split("\n")[0];
  return `<div class="row-item" ${t.status === "pending" ? `onclick="window.go('supervision')"` : ""}>
    <div class="grow">
      <div class="t">🧑‍⚖️ ${esc(line)}</div>
      <div class="m">${esc(t.task_type || "approval")} · raised ${rel(t.created_at)}${t.responded_by ? ` · decided by ${esc(t.responded_by)}` : t.status === "pending" ? " · open in Supervision →" : ""}</div>
    </div>
    <div class="actions"><span class="badge ${esc(t.status || "pending")}">${esc(t.status || "pending")}</span></div>
  </div>`;
}

// ── Step chain rendering ────────────────────────────────────
const KIND_ICON = { agent: "🤖", human_gate: "🧑‍⚖️", condition: "🔀", action: "⚙️" };
function stepKind(t) {
  if (t === "agent" || t === "agent_call") return "agent";
  if (t === "human_gate" || t === "approval") return "human_gate";
  if (t === "condition" || t === "conditional") return "condition";
  return "action";
}
function stepChain(steps) {
  return `<div class="wfsteps">${steps.map(s =>
    `<span class="wfstep ${esc(s.kind)}${s.state ? " " + esc(s.state) : ""}" title="${esc(s.kind)}">
      ${s.state === "done" ? "✓" : s.state === "failed" ? "✕" : KIND_ICON[s.kind] || "⚙️"} ${esc(s.label)}</span>`
  ).join(`<span class="wfarrow">→</span>`)}</div>`;
}

// ── Run graph helpers ───────────────────────────────────────
function orderedNodes(run) {
  const nodes = run.graph?.nodes || [];
  const edges = run.graph?.edges || [];
  if (nodes.length < 2 || edges.length === 0) return nodes;
  const next = {}, hasIncoming = {};
  edges.forEach(e => { next[e.from] = e.to; hasIncoming[e.to] = true; });
  let start = nodes.find(n => !hasIncoming[n.id]);
  if (!start) return nodes;
  const byId = Object.fromEntries(nodes.map(n => [n.id, n]));
  const out = [];
  for (let id = start.id; id && byId[id] && out.length <= nodes.length; id = next[id]) out.push(byId[id]);
  return out.length === nodes.length ? out : nodes;
}
function currentNode(run) {
  const id = (run.current_nodes || [])[0];
  return (run.graph?.nodes || []).find(n => n.id === id) || null;
}
function nodeStates(run, ordered) {
  if (run.status === "completed") return ordered.map(() => "done");
  const curId = (run.current_nodes || [])[0];
  const idx = ordered.findIndex(n => n.id === curId);
  return ordered.map((n, i) => {
    if (idx === -1) return run.status === "failed" ? "failed" : "pending";
    if (i < idx) return "done";
    if (i === idx) return run.status === "failed" ? "failed" : "active";
    return "pending";
  });
}
function duration(run) {
  if (!run.started_at || !run.completed_at) return "";
  const ms = new Date(run.completed_at) - new Date(run.started_at);
  if (ms <= 0) return "";
  const s = Math.round(ms / 1000);
  if (s < 60) return s + "s";
  if (s < 3600) return Math.floor(s / 60) + "m " + (s % 60) + "s";
  return Math.floor(s / 3600) + "h " + Math.floor((s % 3600) / 60) + "m";
}

// ── Inline handlers (global scope, codebase idiom) ─────────
window.wfOpenRun = function (key) {
  const f = document.getElementById("wfForm-" + key);
  if (f) { f.style.display = "block"; document.getElementById("wfTitle-" + key)?.focus(); }
};
window.wfCloseRun = function (key) {
  const f = document.getElementById("wfForm-" + key);
  if (f) f.style.display = "none";
};
window.wfToggleRun = function (id) {
  window._wf.open[id] = !window._wf.open[id];
  const row = [...document.querySelectorAll(".wf-run")].find(el => el.getAttribute("onclick")?.includes(id));
  const d = row?.querySelector(".wf-run-detail");
  if (d) d.style.display = window._wf.open[id] ? "block" : "none";
};
window.wfSubmitRun = async function (deptId, svcId, key) {
  const title = document.getElementById("wfTitle-" + key)?.value.trim();
  const body = document.getElementById("wfBody-" + key)?.value.trim();
  const priority = document.getElementById("wfPrio-" + key)?.value || "P3";
  if (!title) { toast("Give the request a title", "warn"); return; }
  const r = await createServiceRequest(deptId, svcId, title, body, priority);
  if (r.ok) {
    toast("Request submitted — the work loop is compiling the SOP into a run", "ok");
    setTimeout(() => window.go("workflows"), 1200);
  } else {
    toast("Request failed: " + esc(r.data?.detail || r.data?.error?.message || r.status), "bad");
  }
};
