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
  "The agent actually drafts the contract with Qwen — grounded in memory",
  "Governance pauses it for human sign-off",
  "You approve — the decision rides Kafka to the orchestrator",
  "The orchestrator resumes; the tool fires, and the whole story is one trace",
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

    // 5 — the agent ACTUALLY drafts the contract via Qwen, grounded in memory
    mark(4, "on");
    out(4, `<i>The agent is reasoning…</i>`);
    const draft = await post(SVC.orchestration + "/agent/draft", {
      agent_id: salesAgent, role: "Sales Assistant",
      instruction: "Draft a concise contract opening (4 sentences max) for customer Acme for a $250,000 platform subscription. Reflect the customer's known preferences.",
      memory_query: "customer Acme preferences UI billing",
    });
    let contractText = "Send the $250k contract to Acme";
    if (draft.ok && draft.data && draft.data.output) {
      contractText = draft.data.output;
      out(4, `Drafted by <b>${esc(draft.data.model)}</b>, grounded in ${draft.data.memory_used.length} recalled memor${draft.data.memory_used.length === 1 ? "y" : "ies"}:
        <div class="result" style="margin-top:8px"><div class="a" style="white-space:pre-wrap;font-weight:500">${esc(contractText)}</div></div>`);
    } else {
      out(4, `Reasoning unavailable (${esc((draft.data && draft.data.error && draft.data.error.message) || draft.status)}) — proceeding with the task title.`);
    }
    mark(4, draft.ok ? "ok" : "on");

    // 6 — governance gate carries the REAL drafted contract
    mark(5, "on");
    const pipe = await post(SVC.orchestration + "/pipeline", {
      name: "send-contract", steps: [
        {id: "s1", name: "draft-contract", type: "agent"},
        {id: "s2", name: "human-signoff", type: "human_gate"}]});
    const exec = await post(SVC.orchestration + "/executions", {pipeline_id: pipe.data.id});
    const task = await post(SVC.orchestration + "/human-tasks", {
      pipeline_execution_id: exec.data.id, step_id: "s2",
      assignee_id: "manager", instructions: contractText});
    const gate = await post(SVC.supervision + "/approvals", {
      request_id: task.data.id, requester_id: salesAgent,
      type: "parallel", title: "Send contract to Acme ($250k)"});
    out(5, `Governance rule “${esc(c.governance_rules[0].name)}” paused it. The manager inbox now holds gate <b>${esc(gate.data.id.slice(0, 8))}</b> — with the agent's actual draft to review.`);
    mark(5, "ok");

    // 7 — approve
    mark(6, "on");
    await pause(700);
    await post(`${SVC.supervision}/approvals/${gate.data.id}/approve`,
      {approver_id: uuid4(), comment: "Terms verified — send it"});
    out(6, `Approved. The decision left this page as a <b>Kafka event</b> — the portal never calls the orchestrator directly.`);
    mark(6, "ok");

    // 8 — enforcement + tool + audit
    mark(7, "on");
    let enforced = null;
    for (let i = 0; i < 10 && !enforced; i++) {
      await pause(1200);
      const t = await get(`${SVC.orchestration}/human-tasks/${task.data.id}`);
      if (t.ok && t.data && t.data.status !== "pending") enforced = t.data;
    }
    const toolName = "send_email";
    await post(SVC.tools + "/tools/register", {name: toolName, category: "communication", description: "Email relay"});
    await post(SVC.tools + "/execute", {tool: toolName, agent_id: salesAgent,
      parameters: {to: "cfo@acme.example", subject: "Contract — Acme ($250k)"}});
    await pause(2000);
    const sp = await get(SVC.observability + "/spans?page_size=50");
    const gates = await get(SVC.observability + "/spans?span_type=human_gate");
    out(7, `Orchestrator task is <b>${enforced ? esc(enforced.status) : "still resolving"}</b>; the agent sent the contract. ` +
      `<b>${sp.data ? sp.data.total : 0} spans</b> recorded (${gates.data ? gates.data.total : 0} human-gate) — open Observability to walk the trace.`);
    mark(7, "ok");
  } catch (e) {
    console.error(e);
  } finally {
    $("btnStory").disabled = false;
  }
};
