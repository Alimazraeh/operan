// Small DOM + formatting helpers shared by every view.

export const $ = id => document.getElementById(id);

export function esc(s) {
  return String(s ?? "").replace(/[&<>"']/g, c =>
    ({"&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;"}[c]));
}

export const badge = (s) => `<span class="badge ${esc(s)}">${esc(s)}</span>`;

export function rel(ts) {
  if (!ts) return "";
  const s = Math.max(0, (Date.now() - new Date(ts).getTime()) / 1000);
  if (s < 60) return Math.floor(s) + "s ago";
  if (s < 3600) return Math.floor(s / 60) + "m ago";
  if (s < 86400) return Math.floor(s / 3600) + "h ago";
  return new Date(ts).toLocaleString();
}

export function toast(msg, kind = "") {
  const el = document.createElement("div");
  el.className = "toast " + kind;
  el.innerHTML = msg;
  $("toasts").appendChild(el);
  setTimeout(() => el.remove(), 5200);
}

export function rowItem({title, meta, badges = "", actions = "", click = ""}) {
  return `<div class="row-item ${click ? "click" : ""}" ${click ? `onclick="${click}"` : ""}>
    <div class="grow"><div class="t">${title}</div><div class="m">${meta}</div></div>
    ${badges}${actions}</div>`;
}

export const SPAN_LABELS = {
  "operan.memory.vector.ingested": ["🧠", "Agent stored a memory"],
  "operan.memory.vector.searched": ["🔎", "Agent searched its memory"],
  "operan.memory.vector.updated": ["✏️", "Memory updated"],
  "operan.memory.vector.deleted": ["🗑", "Memory deleted"],
  "operan.memory.vector.garbage_collected": ["♻️", "Memory garbage-collected"],
  "operan.supervision.gate.raised": ["⏸", "Workflow paused for human sign-off"],
  "operan.supervision.gate.responded": ["⚖️", "Human decided — enforcement event sent"],
  "operan.supervision.gate.escalated": ["📨", "Gate delegated to another approver"],
  "operan.supervision.gate.timeout": ["⌛", "Gate expired"],
  "operan.supervision.policy.violation_detected": ["🛡", "Policy violation flagged"],
  "operan.tools.tool_registered": ["🧰", "Tool registered"],
  "operan.tools.execution.requested": ["🛠", "Tool execution requested"],
  "operan.tools.execution.started": ["▶️", "Tool execution started"],
  "operan.tools.execution.completed": ["✅", "Tool execution completed"],
  "operan.tools.execution.failed": ["❌", "Tool execution failed"],
  "operan.tenant.provisioned": ["🏢", "Tenant provisioned"],
  "operan.registry.agent.registered": ["🤖", "Agent registered"],
  "operan.templates.template.created": ["📋", "Department template created"],
  "operan.templates.template.deployed": ["🚀", "Department deployed"],
};

export function spanLabel(name, type) {
  if (SPAN_LABELS[name]) return SPAN_LABELS[name];
  const icons = {memory: "🧠", human_gate: "⚖️", tool: "🛠", policy: "🛡", orchestration: "⚙️"};
  return [icons[type] || "•", name];
}

export function eventRow(s) {
  const [icon, label] = spanLabel(s.span_name, s.span_type);
  return `<div class="ev">
    <div class="ico ${esc(s.span_type)}">${icon}</div>
    <div class="what"><b>${esc(label)}</b> <span>· trace ${esc((s.trace_id || "").slice(0, 8))}</span></div>
    ${s.status === "error" ? '<span class="badge error">error</span>' : ""}
    <time>${rel(s.start_time)}</time></div>`;
}
