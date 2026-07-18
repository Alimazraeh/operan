// Tasks & Projects — Create, assign, track, SLA monitoring
import { $, esc, statCard, card, btn, emptyState, toast } from "./ui.js";
import { listWorkflows, createWorkflow, executeWorkflow, uuid4 } from "./api.js";

export default async function viewTasks() {
  const res = await listWorkflows(1, 50);
  const workflows = res.data?.items || res.data || [];

  const byStatus = {
    running: workflows.filter(w => w.status === "running" || w.status === "in_progress").length,
    completed: workflows.filter(w => w.status === "completed" || w.status === "done").length,
    draft: workflows.filter(w => w.status === "draft").length,
    failed: workflows.filter(w => w.status === "failed" || w.status === "error").length,
  };

  return `
    <div class="stats-grid">
      ${statCard("⚙️", "Running", byStatus.running, "Active tasks")}
      ${statCard("✅", "Completed", byStatus.completed, "Done")}
      ${statCard("📝", "Drafts", byStatus.draft, "Not yet started")}
      ${statCard("❌", "Failed", byStatus.failed, "Needs attention")}
    </div>

    ${card("Tasks & Projects", `${workflows.length} total`, `
      <div class="toolbar">
        <div class="search"><input id="taskSearch" placeholder="Search tasks..." oninput="filterTasks(this.value)"></div>
        ${btn("+ Create Task", "primary", "openCreateTaskModal()")}</div>
      <div id="task-list">
        ${workflows.length === 0
          ? emptyState("📋", "No Tasks Yet",
              "Create a task or workflow to assign work to your team agents.",
              btn("Create Task", "primary", "openCreateTaskModal()"))
          : workflows.map(taskRow).join("")}
      </div>
    `)}
  `;
}

function taskRow(w) {
  const status = esc(w.status || "draft");
  const badgeCls = status === "running" || status === "in_progress" ? "running"
    : status === "completed" || status === "done" ? "ok"
    : status === "failed" || status === "error" ? "error" : "pending";
  return `<div class="row-item" data-name="${esc((w.name || "").toLowerCase())}">
    <div class="grow">
      <div class="t">${esc(w.name || "Untitled Task")}</div>
      <div class="m">${status}${w.created_at ? ` · ${new Date(w.created_at).toLocaleDateString()}` : ""}</div>
    </div>
    <div class="actions">
      <span class="badge ${badgeCls}">${status}</span>
      ${(w.status === "draft") ? btn("Run", "primary sm", `runTask("${esc(w.id || "")}")`) : ""}
      ${btn("Details", "ghost sm", `window.go("workflows")`)}
    </div>
  </div>`;
}

window.openCreateTaskModal = function() {
  const modal = document.createElement("div");
  modal.className = "modal-overlay show";
  modal.innerHTML = `
    <div class="modal">
      <div class="modal-header"><h3>Create Task / Project</h3>
        <button class="ghost sm" onclick="this.closest('.modal-overlay').remove()">✕</button></div>
      <div class="modal-body">
        <div class="form-group">
          <label>Task Name</label>
          <input id="taskName" placeholder="e.g. Q3 Financial Review">
        </div>
        <div class="form-group">
          <label>Description</label>
          <textarea id="taskDesc" placeholder="Describe the task, expected output, SLA..."></textarea>
        </div>
        <div class="form-group">
          <label>SLA (hours)</label>
          <input id="taskSLA" type="number" value="24" min="1">
        </div>
      </div>
      <div class="modal-footer">
        <button class="ghost" onclick="this.closest('.modal-overlay').remove()">Cancel</button>
        <button class="primary" onclick="doCreateTask()">Create Task</button>
      </div>
    </div>`;
  document.body.appendChild(modal);
};

async function doCreateTask() {
  const name = document.getElementById("taskName").value.trim();
  const sla = parseInt(document.getElementById("taskSLA").value) || 24;

  if (!name) { toast("Enter a task name", "error"); return; }

  try {
    const n1 = uuid4(), n2 = uuid4(), n3 = uuid4(), n4 = uuid4();
    await createWorkflow(name, [
      { id: n1, name: "Start", type: "start", config: { sla_hours: sla } },
      { id: n2, name: "Execute", type: "agent", config: { sla_hours: sla } },
      { id: n3, name: "Review", type: "human_gate", config: { sla_hours: Math.ceil(sla * 0.25) } },
      { id: n4, name: "Complete", type: "end", config: {} },
    ], [
      { from: n1, to: n2, condition: "pass" },
      { from: n1, to: n2, condition: "fail", action: "escalate" },
      { from: n2, to: n3, condition: "complete" },
    ]);
    toast(`Task "${name}" created`, "success");
    document.querySelector(".modal-overlay").remove();
    window.go("tasks");
  } catch (e) {
    toast(e.message || "Failed to create task", "error");
  }
}

window.runTask = async function(id) {
  try {
    await executeWorkflow(id);
    toast("Task execution started", "success");
    window.go("tasks");
  } catch (e) {
    toast(e.message || "Failed to start task", "error");
  }
};

function filterTasks(query) {
  const q = query.toLowerCase();
  document.querySelectorAll("#task-list .row-item").forEach(row => {
    row.style.display = (row.dataset.name || "").includes(q) ? "" : "none";
  });
}