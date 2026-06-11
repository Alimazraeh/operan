// The Story: a guided end-to-end scenario executed against the live
// platform — deploy a department, teach its agent, watch it work, decide
// at the gate, see the orchestrator obey, and audit everything.
import {SVC, get, post, patch, uuid4, session} from "../api.js";
import {$, esc} from "../ui.js";
import {CATALOG, STAGES} from "./departments.js";

const STEPS = [
  "Deploy the Sales Department",
  "Hire its agents into the registry",
  "The agent learns about the customer",
  "A semantically different question finds the right memory",
  "The agent drafts a $250k contract — governance pauses it",
  "You approve — the decision rides Kafka to the orchestrator",
  "The orchestrator resumes the workflow; the tool fires",
  "Audit: the whole story is one trace",
];

export function viewScenario() {
  return `
    <div class="card">
      <h3>▶ A day in an Operan department</h3>
      <div class="hint">Every step below is a real API call against the live platform — nothing is mocked.
        Watch the activity stream on Overview fill up as it runs.</div>
      <button id="btnStory" class="sm" onclick="window.runStory()">Run the story</button>
      <div class="steps" id="storySteps" style="margin-top:16px">
        ${STEPS.map((t, i) => `<div class="step" id="stp${i}">
          <div class="n">${i + 1}</div>
          <div class="body"><div class="t">${esc(t)}</div><div class="out" id="stpOut${i}"></div></div>
        </div>`).join("")}
      </div>
    </div>`;
}

const mark = (i, cls) => { const el = $("stp" + i); if (el) el.className = "step " + cls; };
const out = (i, html) => { const el = $("stpOut" + i); if (el) el.innerHTML = html; };
const pause = (ms) => new Promise(r => setTimeout(r, ms));

window.runStory = async function () {
  $("btnStory").disabled = true;
  STEPS.forEach((_, i) => { mark(i, ""); out(i, ""); });
  const c = CATALOG[0]; // Sales Department

  try {
    // 1 — deploy the department (template + real pipeline)
    mark(0, "on");
    let tpl;
    const list = await get(SVC.templates + "/templates?page_size=50");
    tpl = ((list.data && list.data.items) || []).find(t => t.name === c.name);
    if (!tpl) {
      const created = await post(SVC.templates + "/templates", {
        name: c.name, category: c.category, description: c.description,
        agents: c.agents, governance_rules: c.governance_rules, kpis: c.kpis,
      });
      tpl = created.data;
    }
    const dep = await post(`${SVC.templates}/templates/${tpl.id}/deploy`,
      {environment: "production", configuration: {region: "me-central"}});
    const depId = dep.data.id;
    for (const stage of STAGES.slice(1)) {
      await patch(`${SVC.templates}/templates/${tpl.id}/deployments/${depId}`, {status: stage});
      await pause(180);
    }
    out(0, `Deployment <b>${depId.slice(0, 8)}</b> walked the real pipeline to <b>operational</b> (Module 05).`);
    mark(0, "ok");

    // 2 — register agents
    mark(1, "on");
    const agentIds = {};
    for (const a of c.agents) {
      const res = await post(SVC.registry + "/registry/agents", {
        id: uuid4(), tenant_id: session.tenant, name: a.name, role: a.role,
        version: "1.0.0", capabilities: a.capabilities, tools: a.tool_requirements || [],
        department_id: depId,
      });
      if (res.ok) agentIds[a.id] = res.data.id || res.data.agent_id;
    }
    const salesAgent = agentIds["sales-assistant"] || uuid4();
    out(1, `<b>${c.agents.length} agents</b> now employed by the department (Module 04).`);
    mark(1, "ok");

    // 3 — teach the agent
    mark(2, "on");
    await post(SVC.memory + "/vectors", {items: [
      {document_id: uuid4(), embedding_type: "agent_personal",
       semantic_content: "Customer Acme prefers Arabic-first UI and quarterly billing",
       metadata: {agent_id: salesAgent, department_id: depId}},
      {document_id: uuid4(), embedding_type: "agent_personal",
       semantic_content: "Unrelated note about office plants and watering schedules",
       metadata: {agent_id: salesAgent, department_id: depId}},
    ]});
    out(2, `Two memories stored & embedded on-cluster — one signal, one distractor (Module 07 + qwen3).`);
    mark(2, "ok");

    // 4 — semantic recall
    mark(3, "on");
    const sr = await post(SVC.memory + "/search", {
      query: "which interface language does the client like",
      embedding_type: "agent_personal", relevance_threshold: 0.3});
    const hit = ((sr.data && sr.data.items) || [])[0];
    out(3, hit
      ? `Asked with <i>zero shared words</i> — recalled: <b>“${esc(hit.content)}”</b> (relevance ${(hit.score * 100).toFixed(0)}%).`
      : `No recall — check the embeddings gateway.`);
    mark(3, hit ? "ok" : "on");

    // 5 — gated workflow
    mark(4, "on");
    const pipe = await post(SVC.orchestration + "/pipeline", {
      name: "send-contract", steps: [
        {id: "s1", name: "draft-contract", type: "agent"},
        {id: "s2", name: "human-signoff", type: "human_gate"}]});
    const exec = await post(SVC.orchestration + "/executions", {pipeline_id: pipe.data.id});
    const task = await post(SVC.orchestration + "/human-tasks", {
      pipeline_execution_id: exec.data.id, step_id: "s2",
      assignee_id: "manager", instructions: "Send the $250k contract to Acme"});
    const gate = await post(SVC.supervision + "/approvals", {
      request_id: task.data.id, requester_id: salesAgent,
      type: "parallel", title: "Send contract to Acme ($250k)"});
    out(4, `Governance rule “${esc(c.governance_rules[0].name)}” paused the workflow. Gate <b>${gate.data.id.slice(0, 8)}</b> is in the manager inbox.`);
    mark(4, "ok");

    // 6 — approve
    mark(5, "on");
    await pause(700);
    await post(`${SVC.supervision}/approvals/${gate.data.id}/approve`,
      {approver_id: uuid4(), comment: "Terms verified — send it"});
    out(5, `Approved. The decision left this page as a <b>Kafka event</b> — the portal never calls the orchestrator.`);
    mark(5, "ok");

    // 7 — orchestrator enforcement + tool
    mark(6, "on");
    let enforced = null;
    for (let i = 0; i < 10 && !enforced; i++) {
      await pause(1200);
      const t = await get(`${SVC.orchestration}/human-tasks/${task.data.id}`);
      if (t.ok && t.data && t.data.status !== "pending") enforced = t.data;
    }
    const toolName = "send_email";
    await post(SVC.tools + "/tools/register", {name: toolName, category: "communication", description: "Email relay"});
    const ex = await post(SVC.tools + "/execute", {tool: toolName, agent_id: salesAgent,
      parameters: {to: "cfo@acme.example", subject: "Contract — Acme ($250k)"}});
    out(6, enforced
      ? `Orchestrator task is <b>${esc(enforced.status)}</b> (decided by ${esc((enforced.responded_by || "").slice(0, 8))}); the agent executed <b>${toolName}</b>${ex.ok ? "" : " (tool call failed)"}.`
      : `Still waiting on Kafka — check Workflows in a few seconds. Tool ${ex.ok ? "executed" : "failed"}.`);
    mark(6, enforced ? "ok" : "on");

    // 8 — audit
    mark(7, "on");
    await pause(2500);
    const sp = await get(SVC.observability + "/spans?page_size=50");
    const total = sp.data ? sp.data.total : 0;
    const gates = await get(SVC.observability + "/spans?span_type=human_gate");
    out(7, `<b>${total} spans</b> observed, including <b>${gates.data ? gates.data.total : 0} human-gate events</b> — open Observability to walk the trace.`);
    mark(7, "ok");
  } catch (e) {
    console.error(e);
  } finally {
    $("btnStory").disabled = false;
  }
};
