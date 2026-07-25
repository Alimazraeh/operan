// Tasks & Projects — the work ledger of the operation.
//
// In this platform the trackable unit of work is not a free-floating todo:
// it is a SERVICE REQUEST — demand raised against a department, executed by
// the work loop against the service's SLA, pausing at human gates. This page
// is where that ledger lives: what's waiting on a person right now, what's
// in flight against its clocks, and what settled (met, late, or failed).
import { esc, rel, toast, statCard } from "../ui.js";
import {
  SVC, post, session, unwrapList, listDepartments, listDeptRequests,
  getDeptServices, createServiceRequest, listSupervisionQueue,
} from "../api.js";

const OPEN_STATUSES = ["submitted", "dispatched", "in_progress", "awaiting_approval"];
const KIND_ICON = {
  created: "📥", dispatched: "🚀", agent_output: "🤖", gate_raised: "🚧", action_executed: "⚙️",
  gate_responded: "✍️", completed: "✅", failed: "❌", sla_breached: "⏰", cancelled: "🚫",
};

window._tasks = window._tasks || { open: {} };

export default async function viewTasks() {
  clearInterval(window._tasksPoll);

  const [deptR, queueR] = await Promise.allSettled([
    listDepartments(1, 50),
    listSupervisionQueue(1, 25),
  ]);
  const depts = deptR.status === "fulfilled" ? unwrapList(deptR.value) : [];
  const liveDepts = depts.filter(d => d.status === "operational" || d.status === "degraded");
  const queue = (queueR.status === "fulfilled" ? unwrapList(queueR.value) : [])
    .filter(q => q.item_type === "approval" && (q.status === "pending" || q.status === "in_progress"));

  const reqLists = await Promise.allSettled(liveDepts.map(d => listDeptRequests(d.id)));
  const deptById = {};
  liveDepts.forEach(d => { deptById[d.id] = d; });
  const requests = reqLists
    .flatMap((r, i) => (r.status === "fulfilled" && r.value.ok ? unwrapList(r.value) : [])
      .map(req => ({ ...req, _dept: liveDepts[i] })))
    .sort((a, b) => new Date(b.created_at || 0) - new Date(a.created_at || 0));

  const open = requests.filter(r => OPEN_STATUSES.includes(r.status));
  const settled = requests.filter(r => !OPEN_STATUSES.includes(r.status));
  const overdue = open.filter(r => slaState(r).some(c => c.cls === "bad"));
  const completed = settled.filter(r => r.status === "completed");
  const failed = settled.filter(r => r.status === "failed" || r.status === "cancelled");

  window._tasks.depts = liveDepts;

  // Live ledger while work is in flight (skip when the user has something open).
  if (open.length > 0 || queue.length > 0) {
    window._tasksPoll = setInterval(() => {
      const root = document.getElementById("tasksRoot");
      if (!root) { clearInterval(window._tasksPoll); return; }
      const busy = document.querySelector(".modal-overlay") || Object.values(window._tasks.open).some(Boolean);
      if (!busy) window.go("tasks");
    }, 15000);
  }

  return `<div id="tasksRoot">
    <div class="stats-grid">
      ${statCard("📨", "Open requests", open.length, open.length ? "Being worked now" : "Queue is clear")}
      ${statCard("🧑‍⚖️", "Needs a human", queue.length, queue.length ? "Decisions waiting on you" : "Nothing on your desk")}
      ${statCard("⏰", "SLA at risk", overdue.length, overdue.length ? "Past a due clock" : "All clocks green")}
      ${statCard("✅", "Completed", completed.length, "Delivered with work product")}
      ${statCard("🛑", "Failed / cancelled", failed.length, failed.length ? "Read the timeline for why" : "None")}
    </div>

    <div class="card" style="margin-bottom:18px">
      <h3>Needs a human now <span class="tag">gates waiting on a decision</span></h3>
      <div class="hint">Agent work paused at a control point. Deciding here resumes (or stops) the run —
      same authority as the <a href="#" onclick="window.go('supervision');return false">Supervision</a> queue.</div>
      ${queue.length === 0
        ? `<div class="empty">Nothing waiting on a person.</div>`
        : queue.map(q => `
          <div class="row-item" data-name="${esc((q.title || "").toLowerCase())}">
            <div class="grow"><div class="t">⏸ ${esc(q.title || "approval")}</div>
            <div class="m">${esc(q.priority || "medium")} · raised ${rel(q.created_at)}</div></div>
            <div class="actions">
              <button class="ok sm" onclick="window.tasksDecide('${esc(q.item_id)}','approve')">Approve</button>
              <button class="bad sm" onclick="window.tasksDecide('${esc(q.item_id)}','reject')">Reject</button>
            </div>
          </div>`).join("")}
    </div>

    <div class="card" style="margin-bottom:18px">
      <h3>Requests in flight <span class="tag">the open ledger</span></h3>
      <div class="hint">Every open service request across your departments, against its SLA clocks.
      <button class="sm ghost" style="margin-left:8px" onclick="window.go('tasks')">↻ Refresh</button></div>
      <div class="toolbar">
        <div class="search"><input id="taskSearch" placeholder="Filter requests…" oninput="window.filterTasksLedger(this.value)"></div>
        <button class="primary" onclick="window.openNewRequestModal()">+ New request</button>
      </div>
      ${open.length === 0
        ? `<div class="empty">Nothing in flight — raise a request and the department gets to work.</div>`
        : open.map(r => requestRow(r, true)).join("")}
    </div>

    <div class="card">
      <h3>Settled recently <span class="tag">met, late, or failed — honestly</span></h3>
      ${settled.length === 0
        ? `<div class="empty">Nothing settled yet.</div>`
        : settled.slice(0, 10).map(r => requestRow(r, false)).join("")}
    </div>
  </div>`;
}

// ── SLA state: chips for the two clocks ─────────────────────
function slaState(r) {
  const chips = [];
  const now = Date.now();
  const done = r.status === "completed" || r.status === "failed" || r.status === "cancelled";
  if (r.sla_response_due) {
    const due = new Date(r.sla_response_due).getTime();
    if (r.first_response_at) {
      const met = new Date(r.first_response_at).getTime() <= due;
      chips.push({ cls: met ? "ok" : "bad", label: met ? "responded in SLA" : "response late" });
    } else if (!done) {
      chips.push(now > due
        ? { cls: "bad", label: "response overdue " + ago(due) }
        : { cls: timeLeftCls(due, r.created_at), label: "response due " + inTime(due) });
    }
  }
  if (r.sla_resolution_due) {
    const due = new Date(r.sla_resolution_due).getTime();
    if (r.status === "completed") {
      const met = !r.completed_at || new Date(r.completed_at).getTime() <= due;
      chips.push({ cls: met ? "ok" : "bad", label: met ? "resolved in SLA" : "resolved late" });
    } else if (!done) {
      chips.push(now > due
        ? { cls: "bad", label: "resolution overdue " + ago(due) }
        : { cls: timeLeftCls(due, r.created_at), label: "resolution due " + inTime(due) });
    }
  }
  return chips;
}
function timeLeftCls(due, createdAt) {
  const total = due - new Date(createdAt || Date.now()).getTime();
  const left = due - Date.now();
  return total > 0 && left / total < 0.25 ? "warn" : "";
}
function spanTxt(ms) {
  const m = Math.max(1, Math.round(Math.abs(ms) / 60000));
  if (m < 60) return m + "m";
  if (m < 1440) return Math.floor(m / 60) + "h " + (m % 60) + "m";
  return Math.floor(m / 1440) + "d " + Math.floor((m % 1440) / 60) + "h";
}
function inTime(due) { return "in " + spanTxt(due - Date.now()); }
function ago(due) { return "by " + spanTxt(Date.now() - due); }

// ── Request rows ────────────────────────────────────────────
function requestRow(r, isOpen) {
  const chips = slaState(r).map(c =>
    `<span class="sla-chip ${c.cls}">${esc(c.label)}</span>`).join(" ");
  const gateWait = r.status === "awaiting_approval";
  const lastEvent = (r.timeline || [])[Math.max(0, (r.timeline || []).length - 1)];
  const failNote = (r.status === "failed" || r.status === "cancelled") && lastEvent ? lastEvent.detail : "";
  const expanded = !!window._tasks.open[r.id];
  const searchKey = `${r.title} ${r.service_name || ""} ${r._dept?.name || ""} ${r.status}`.toLowerCase();
  return `<div class="row-item" data-name="${esc(searchKey)}" onclick="window.tasksToggleReq('${esc(r.id)}')">
    <div class="grow">
      <div class="t">${esc(r.title || "Untitled request")}
        <span class="tag">${esc(r.priority || "P3")}</span>
        ${r._dept ? `<span class="tag">${esc(r._dept.name)}</span>` : ""}
        ${r.service_name ? `<span class="tag">${esc(r.service_name)}</span>` : ""}</div>
      <div class="m">${isOpen ? "raised" : esc(r.status)} ${rel(isOpen ? r.created_at : (r.completed_at || r.updated_at || r.created_at))}
        ${gateWait ? ` · <span style="color:var(--gate)">⏸ waiting on approval</span>` : ""}
        ${r.tokens_used ? ` · ${r.tokens_used} tokens` : ""}
        ${failNote ? ` · ${esc(String(failNote).slice(0, 90))}` : ""}
        ${chips ? " · " + chips : ""}</div>
      <div class="wf-run-detail" style="display:${expanded ? "block" : "none"}" onclick="event.stopPropagation()">
        ${(r.timeline || []).map(e => `
          <div class="m" style="margin:4px 0">${KIND_ICON[e.kind] || "•"} <b>${esc(e.kind || "event")}</b>
            · ${rel(e.at)}${e.detail ? ` — ${esc(String(e.detail).slice(0, 160))}` : ""}</div>`).join("")
          || `<div class="m">No timeline recorded.</div>`}
        ${r.workflow_run_ref ? `<button class="sm ghost" style="margin-top:6px" onclick="window.go('workflows')">View the run</button>` : ""}
        ${gateWait ? `<button class="sm" style="margin-top:6px" onclick="window.go('supervision')">Open the gate</button>` : ""}
      </div>
    </div>
    <div class="actions"><span class="badge ${esc(r.status || "submitted")}">${esc(r.status || "submitted")}</span></div>
  </div>`;
}

// ── Handlers ────────────────────────────────────────────────
window.tasksToggleReq = function (id) {
  window._tasks.open[id] = !window._tasks.open[id];
  const row = [...document.querySelectorAll("#tasksRoot .row-item")].find(el => el.getAttribute("onclick")?.includes(id));
  const d = row?.querySelector(".wf-run-detail");
  if (d) d.style.display = window._tasks.open[id] ? "block" : "none";
};

window.filterTasksLedger = function (query) {
  const q = (query || "").toLowerCase();
  document.querySelectorAll("#tasksRoot [data-name]").forEach(row => {
    row.style.display = (row.dataset.name || "").includes(q) ? "" : "none";
  });
};

// Same authority as Supervision's inbox, staying on the ledger afterwards.
window.tasksDecide = async function (id, action) {
  // The server attributes the decision to the authenticated caller; nothing
  // here can nominate an approver. A session without a user id cannot decide —
  // recording a random one is how the audit trail used to log a decision by
  // nobody.
  if (!session.userId) { toast("Sign in again — a decision must be attributable to you", "bad"); return; }
  const body = action === "approve"
    ? { comment: "Approved from the work ledger" }
    : { reason: "Rejected from the work ledger" };
  const r = await post(`${SVC.supervision}/approvals/${id}/${action}`, body);
  if (r.ok) toast(`Decision sent — the run will ${action === "approve" ? "resume" : "stop"}`, "ok");
  else toast("Decision failed: " + esc(r.data?.error?.message || r.status), "bad");
  setTimeout(() => window.go("tasks"), 800);
};

// ── New request: the real way work enters the platform ──────
window.openNewRequestModal = function () {
  const depts = window._tasks.depts || [];
  const modal = document.createElement("div");
  modal.className = "modal-overlay show";
  modal.innerHTML = `
    <div class="modal">
      <div class="modal-header"><h3>New service request</h3>
        <button class="ghost sm" onclick="this.closest('.modal-overlay').remove()">✕</button></div>
      <div class="modal-body">
        <div class="form-group"><label>Department</label>
          <select id="reqDept" onchange="window.tasksLoadServices(this.value)">
            <option value="">— choose —</option>
            ${depts.map(d => `<option value="${esc(d.id)}">${esc(d.name)}</option>`).join("")}
          </select></div>
        <div class="form-group"><label>Service</label>
          <select id="reqService" disabled><option value="">Pick a department first</option></select>
          <div class="hint">The service's SOP becomes the run; its SLA starts the clocks.</div></div>
        <div class="form-group"><label>Title</label><input id="reqTitle" placeholder="What do you need?"></div>
        <div class="form-group"><label>Details</label><textarea id="reqBody" rows="3" placeholder="Context the agents should work from"></textarea></div>
        <div class="form-group"><label>Priority</label>
          <select id="reqPrio"><option>P3</option><option>P1</option><option>P2</option><option>P4</option></select></div>
      </div>
      <div class="modal-footer">
        <button class="ghost" onclick="this.closest('.modal-overlay').remove()">Cancel</button>
        <button class="primary" onclick="window.doSubmitRequest()">Submit request</button>
      </div>
    </div>`;
  document.body.appendChild(modal);
};

window.tasksLoadServices = async function (deptId) {
  const sel = document.getElementById("reqService");
  if (!sel) return;
  if (!deptId) { sel.disabled = true; sel.innerHTML = `<option value="">Pick a department first</option>`; return; }
  sel.disabled = true; sel.innerHTML = `<option value="">Loading services…</option>`;
  const r = await getDeptServices(deptId);
  const services = unwrapList(r).filter(s => s.status !== "retired");
  sel.innerHTML = services.length
    ? services.map(s => `<option value="${esc(s.id)}">${esc(s.name)}</option>`).join("")
    : `<option value="">No services on this department</option>`;
  sel.disabled = services.length === 0;
};

window.doSubmitRequest = async function () {
  const deptId = document.getElementById("reqDept").value;
  const serviceId = document.getElementById("reqService").value;
  const title = document.getElementById("reqTitle").value.trim();
  const body = document.getElementById("reqBody").value.trim();
  const priority = document.getElementById("reqPrio").value;
  if (!deptId || !serviceId) { toast("Choose a department and service", "warn"); return; }
  if (!title) { toast("Give the request a title", "warn"); return; }
  const r = await createServiceRequest(deptId, serviceId, title, body, priority);
  if (r.ok) {
    toast("Request raised — SLA clocks running, the department is on it", "ok");
    document.querySelector(".modal-overlay")?.remove();
    setTimeout(() => window.go("tasks"), 1000);
  } else {
    toast("Request failed: " + esc(r.data?.detail || r.data?.error?.message || r.status), "bad");
  }
};
