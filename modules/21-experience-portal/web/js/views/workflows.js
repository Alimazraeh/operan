// Workflows: pipelines, executions, and human tasks (Module 03).
import {SVC, get, post} from "../api.js";
import {$, esc, badge, rel, toast, rowItem} from "../ui.js";

export async function viewWorkflows() {
  const [pR, eR, htR] = await Promise.all([
    get(SVC.orchestration + "/pipeline"),
    get(SVC.orchestration + "/executions"),
    get(SVC.orchestration + "/human-tasks"),
  ]);
  const pipelines = (pR.data && (pR.data.items || pR.data.pipelines)) || [];
  const execs = (eR.data && (eR.data.items || eR.data.executions)) || [];
  const tasks = (htR.data && (htR.data.items || htR.data.tasks)) || [];

  const pRows = pipelines.length === 0
    ? `<div class="empty">No pipelines defined yet.</div>`
    : pipelines.slice(0, 12).map(p => rowItem({
        title: `⚙️ ${esc(p.name)}`,
        meta: `${(p.steps || []).length} step(s)${(p.steps || []).some(s => s.type === "human_gate") ? " · includes human gate" : ""} · ${rel(p.created_at)}`,
        badges: badge(p.status || "active"),
        actions: `<button class="sm ghost" onclick="window.runPipeline('${p.id}')">Run</button>`,
      })).join("");

  const eRows = execs.length === 0
    ? `<div class="empty">No executions yet.</div>`
    : execs.slice(0, 12).map(x => rowItem({
        title: `▶ ${esc((x.pipeline_id || "").slice(0, 8))}`,
        meta: `started ${rel(x.created_at || x.started_at)}`,
        badges: badge(x.status || "running"),
      })).join("");

  const tRows = tasks.length === 0
    ? `<div class="empty">No human tasks.</div>`
    : tasks.slice(0, 12).map(t => rowItem({
        title: `🧑‍⚖️ ${esc(t.instructions || t.label || "task")}`,
        meta: `${esc((t.pipeline_execution_id || "").slice(0, 8))} · ${rel(t.created_at)}${t.responded_by ? " · decided by " + esc(t.responded_by.slice(0, 8)) : ""}`,
        badges: badge(t.status),
      })).join("");

  return `
    <div class="card" style="margin-bottom:14px">
      <h3>Pipelines <span class="tag">Module 03 · orchestration</span></h3>
      <div class="hint">Department workflows. Steps typed <code>human_gate</code> pause for supervision before continuing.</div>
      ${pRows}
      <div class="frow" style="margin-top:10px">
        <input id="wfName" placeholder="new pipeline name (created with agent-work + human-signoff steps)">
        <button class="sm" onclick="window.createPipeline()">Create</button>
      </div>
    </div>
    <div class="grid g2">
      <div class="card"><h3>Executions</h3><div class="hint">Runs of the pipelines above.</div>${eRows}</div>
      <div class="card"><h3>Human tasks</h3>
        <div class="hint">Where executions wait for people. Decisions arrive from Supervision via Kafka.</div>${tRows}</div>
    </div>`;
}

window.createPipeline = async function () {
  const name = $("wfName").value.trim();
  if (!name) return;
  const r = await post(SVC.orchestration + "/pipeline", {
    name, steps: [
      {id: "s1", name: "agent-work", type: "agent"},
      {id: "s2", name: "human-signoff", type: "human_gate"}],
  });
  if (r.ok) { toast("Pipeline created", "ok"); window.go("workflows"); }
};

window.runPipeline = async function (pipelineId) {
  const r = await post(SVC.orchestration + "/executions", {pipeline_id: pipelineId});
  if (r.ok) { toast("Execution started", "ok"); window.go("workflows"); }
  else toast("Run failed: " + esc(JSON.stringify(r.data).slice(0, 100)), "bad");
};
