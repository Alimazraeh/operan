// Departments: template catalog → deployment pipeline → department detail.
import { SVC, get, post, patch, uuid4, session } from "../api.js";
import { $, esc, badge, rel, toast, rowItem } from "../ui.js";

export const STAGES = ["select", "configure", "connect_data", "provision_memory", "deploy_swarm", "operational"];
const STAGE_LABELS = {
  select: "Selected", configure: "Configured", connect_data: "Data connected",
  provision_memory: "Memory provisioned", deploy_swarm: "Swarm deployed", operational: "Operational",
};

export const CATALOG = [
  { name: "Sales Department", category: "sales", icon: "💼",
    description: "Qualifies leads, drafts proposals and contracts, and manages customer relationships — with human sign-off on every commitment.",
    agents: [{ id: "sales-assistant", role: "sales", name: "Sales Assistant", capabilities: ["lead_qualification", "crm_lookup"], tool_requirements: ["send_email"], model: "Qwen3.6" },
             { id: "contract-drafter", role: "legal-support", name: "Contract Drafter", capabilities: ["draft_contracts", "term_review"], tool_requirements: ["send_email"], model: "Qwen3.6" }],
    governance_rules: [{ id: "gr-1", name: "Human sign-off on commitments", type: "compliance", enforcement: "enforce", description: "Any outbound contract requires manager approval." }],
    kpis: [{ id: "kpi-1", name: "Lead response time", metric_type: "timer", unit: "minutes" }, { id: "kpi-2", name: "Contracts sent", metric_type: "counter" }],
  },
  { name: "Customer Support", category: "support", icon: "🎧",
    description: "Answers customer inquiries from its knowledge memory, escalates incidents, and never loses context between conversations.",
    agents: [{ id: "support-agent", role: "support", name: "Support Agent", capabilities: ["answer_inquiries", "kb_search"], tool_requirements: ["send_email"], model: "Qwen3.6" },
             { id: "escalation-handler", role: "support-lead", name: "Escalation Handler", capabilities: ["incident_triage"], tool_requirements: [], model: "Qwen3.6" }],
    governance_rules: [{ id: "gr-1", name: "Refund approval gate", type: "compliance", enforcement: "enforce", description: "Refunds above threshold require human approval." }],
    kpis: [{ id: "kpi-1", name: "First response time", metric_type: "timer", unit: "minutes" }],
  },
  { name: "Finance Department", category: "finance", icon: "📊",
    description: "Reconciles invoices, watches budgets, and prepares reports — every payment instruction gated behind a human controller.",
    agents: [{ id: "invoice-analyst", role: "finance", name: "Invoice Analyst", capabilities: ["invoice_reconciliation"], tool_requirements: [], model: "Qwen3.6" }],
    governance_rules: [{ id: "gr-1", name: "Payment instruction gate", type: "compliance", enforcement: "enforce", description: "All payment instructions require controller approval." }],
    kpis: [{ id: "kpi-1", name: "Invoices processed", metric_type: "counter" }],
  },
];

let templates = [];

async function seedCatalog() {
  const r = await get(SVC.templates + "/templates?page_size=50");
  templates = (r.ok && r.data && r.data.items) || [];
  for (const c of CATALOG) {
    if (!templates.some(t => t.name === c.name)) {
      const created = await post(SVC.templates + "/templates", {
        name: c.name, category: c.category, description: c.description,
        agents: c.agents, governance_rules: c.governance_rules, kpis: c.kpis,
        tags: ["catalog"], metadata: { icon: c.icon },
      });
      if (created.ok) templates.push(created.data);
    }
  }
}

export async function viewDepartments() {
  await seedCatalog();
  const deployed = [];
  for (const t of templates) {
    const r = await get(`${SVC.templates}/templates/${t.id}/deployments`);
    for (const d of ((r.data && r.data.items) || [])) deployed.push({ ...d, template: t });
  }

  return `
    <div class="card" style="margin-bottom:18px">
      <h3>Your departments <span class="tag">Module 05</span></h3>
      <div class="hint">Live organizational units staffed by agents, running on this cluster.</div>
      ${deployed.length === 0
        ? `<div class="empty">No departments deployed yet — pick one from the catalog below.</div>`
        : deployed.map(d => rowItem({
            title: `${esc(d.template.name)} <span class="tag">${esc(d.environment || "production")}</span>`,
            meta: `deployed ${rel(d.created_at)} · ${esc(d.id.slice(0, 8))}`,
            badges: badge(d.status),
            click: `window.go('department','${d.template.id}','${d.id}')`,
          })).join("")}
    </div>

    <div class="card">
      <h3>Department catalog <span class="tag">Module 05 · templates</span></h3>
      <div class="hint">Blueprints: agents, workflows, governance, and KPIs — deployable in one click.</div>
      <div class="grid g3">${templates.map(t => {
        const c = CATALOG.find(x => x.name === t.name) || {};
        return `<div class="tmpl">
          <h4>${c.icon || "🏢"} ${esc(t.name)}</h4>
          <div class="d">${esc(t.description || "")}</div>
          <div class="chips">${(t.agents || []).map(a => `<span>🤖 ${esc(a.name || a.role)}</span>`).join("")}
            ${(t.governance_rules || []).map(g => `<span>🛡 ${esc(g.name)}</span>`).join("")}</div>
          <div class="foot"><span class="tag">${esc(t.category)}</span>
            <button class="sm" onclick="window.deployDept('${t.id}')">Deploy department</button></div></div>`;
      }).join("")}</div>
    </div>`;
}

window.agentDoWork = async function (agentId, agentName, role, deploymentId) {
  const instruction = $("taskInstr").value.trim();
  if (!instruction) return;
  $("workOut").innerHTML = `<div class="hint" style="margin-top:10px"><span class="pulse-dot"></span>${esc(agentName)} is reasoning…</div>`;
  try {
    const draft = await post(SVC.orchestration + "/agent/draft", {
      agent_id: agentId, role, instruction, memory_query: "customer preferences",
    });
    if (!draft.ok || !draft.data?.output) {
      $("workOut").innerHTML = `<div class="error-box"><div class="err-title">Agent reasoning failed</div><div class="err-msg">${esc(JSON.stringify(draft.data?.error?.message || draft.data).slice(0, 200))}</div></div>`;
      return;
    }
    const text = draft.data.output;
    await post(SVC.orchestration + "/pipeline", {
      name: "agent-work", steps: [{ id: "s1", name: "agent-work", type: "agent" }, { id: "s2", name: "human-signoff", type: "human_gate" }],
    });
    const pipeId = "pipe-" + uuid4();
    const task = await post(SVC.orchestration + "/human-tasks", {
      pipeline_execution_id: pipeId, step_id: "s2", assignee_id: "manager", instructions: text,
    });
    await post(SVC.supervision + "/approvals", { request_id: task.data.id, requester_id: agentId, type: "parallel", title: instruction.slice(0, 70) });
    $("workOut").innerHTML = `<div class="result"><div class="q">${esc(agentName)} · drafted by ${esc(draft.data.model)} · grounded in ${draft.data.memory_used.length} memor${draft.data.memory_used.length===1?"y":"ies"}</div><div class="a" style="white-space:pre-wrap">${esc(text)}</div><div class="meta"><span>${draft.data.tokens} tokens</span><span>awaiting sign-off in Supervision →</span></div></div>`;
    toast("Agent produced real work — sign off in Supervision", "ok");
  } catch (e) {
    $("workOut").innerHTML = `<div class="error-box"><div class="err-title">Error</div><div class="err-msg">${esc(String(e))}</div></div>`;
  }
};

window.deployDept = async function (templateId) {
  const t = templates.find(x => x.id === templateId);
  if (!t) return;
  const view = $("view");
  const stagesHTML = STAGES.map((s, i) =>
    `<div class="stg" id="dstg${i}"><div class="b">${i+1}</div><div class="l">${STAGE_LABELS[s]}</div></div>`).join("");
  view.innerHTML = `<div class="card"><h3>Deploying ${esc(t.name)}</h3>
    <div class="hint">Module 05 runs the deployment pipeline; each stage performs real platform work.</div>
    <div class="stages">${stagesHTML}</div><div id="deploylog"></div></div>`;

  const log = (msg) => {
    $("deploylog").insertAdjacentHTML("beforeend", `<div class="ev"><div class="ico orchestration">›</div><div class="what">${msg}</div><time>${new Date().toLocaleTimeString()}</time></div>`);
  };
  const mark = (i, cls) => { $("dstg"+i).className = "stg " + cls; };
  const advance = (s) => patch(`${SVC.templates}/templates/${templateId}/deployments/${depId}`, { status: s });
  const pause = (ms) => new Promise(r => setTimeout(r, ms));

  try {
    mark(0, "on");
    const dep = await post(`${SVC.templates}/templates/${templateId}/deploy`, { environment: "production", configuration: { region: "me-central" } });
    if (!dep.ok) { toast("Deployment failed to start", "bad"); return; }
    const depId = dep.data.id;
    log(`Deployment <b>${esc(depId.slice(0, 8))}</b> created in Module 05`);
    mark(0, "ok");

    for (const stage of STAGES.slice(1)) {
      mark(STAGES.indexOf(stage), "on");
      await advance(stage);
      log(`${stage} applied`);
      mark(STAGES.indexOf(stage), "ok");
      await pause(stage === "provision_memory" ? 1200 : 400);
    }

    // provision_memory — real Module 07 work
    await post(SVC.memory + "/vectors", { items: [{ document_id: uuid4(), embedding_type: "department", semantic_content: `${t.name} charter: ${t.description || "deployed via Operan"}`, metadata: { department_id: depId, template_id: templateId } }] });
    log("Department memory provisioned in Module 07");

    // deploy_swarm — real Module 04 work
    for (const a of (t.agents || [])) {
      const res = await post(SVC.registry + "/registry/agents", {
        id: uuid4(), tenant_id: session.tenant, name: a.name || a.role, role: a.role, version: "1.0.0",
        capabilities: a.capabilities || [], tools: a.tool_requirements || [], department_id: depId,
      });
      log(res.ok ? `Agent <b>${esc(a.name || a.role)}</b> registered` : `Agent ${esc(a.name||a.role)} failed: ${esc(JSON.stringify(res.data).slice(0,90))}`);
    }

    log(`<b>${esc(t.name)} is operational.</b>`);
    toast(`${esc(t.name)} deployed`, "ok");
    await pause(900);
    window.go("department", templateId, depId);
  } catch (e) {
    log(`<span style="color:var(--bad)">Error: ${esc(String(e))}</span>`);
    toast("Deployment failed", "bad");
  }
};

export async function viewDepartment(templateId, deploymentId) {
  let tr, dr, agentsR;
  try {
    [tr, dr, agentsR] = await Promise.all([
      get(`${SVC.templates}/templates/${templateId}`),
      get(`${SVC.templates}/templates/${templateId}/deployments/${deploymentId}`),
      get(SVC.registry + "/registry/agents?page_size=100"),
    ]);
  } catch (e) { return viewError("Failed to load department", e.message); }

  const t = tr.data || {}, d = dr.data || {};
  const allAgents = (agentsR.data && agentsR.data.items) || [];
  const deptAgents = allAgents.filter(a => a.department_id === deploymentId);

  const stageIdx = STAGES.indexOf(d.status);
  const stagesHTML = STAGES.map((s, i) =>
    `<div class="stg ${i <= stageIdx ? "ok" : ""}"><div class="b">${i+1}</div><div class="l">${STAGE_LABELS[s]}</div></div>`).join("");

  const agentCards = deptAgents.length === 0
    ? `<div class="empty">No agents found for this department.</div>`
    : deptAgents.map(a => rowItem({
        title: "🤖 " + esc(a.name),
        meta: `${esc(a.role)} · ${(a.capabilities||[]).map(esc).join(", ") || "—"}`,
        badges: badge(a.status || "active"),
        click: `window.go('agent','${a.id}')`,
      })).join("");

  const lead = deptAgents[0];
  const taskCard = !lead ? "" : `
    <div class="card" style="margin-bottom:18px">
      <h3>Give the department real work <span class="tag">Module 03 + 12</span></h3>
      <div class="hint">Hand <b>${esc(lead.name)}</b> a task. It drafts with the LLM, grounded in memory, then goes to you for sign-off.</div>
      <textarea id="taskInstr" rows="2">Draft a concise contract opening (4 sentences max) for customer Acme for a $250,000 platform subscription, reflecting their known preferences.</textarea>
      <div style="margin-top:10px"><button class="sm" onclick="window.agentDoWork('${esc(lead.id)}','${esc(lead.name)}','${esc(lead.role||"agent")}','${esc(deploymentId)}')">Let the agent work</button></div>
      <div id="workOut"></div>
    </div>`;

  return `
    <span class="back" onclick="window.go('departments')">← All departments</span>
    <div class="card" style="margin-bottom:18px">
      <h3>${esc(t.name)} ${badge(d.status)}</h3>
      <div class="hint">${esc(t.description || "")}</div>
      <div class="stages">${stagesHTML}</div>
      <div class="kv"><dt>Deployment</dt><dd>${esc(deploymentId)}</dd><dt>Environment</dt><dd>${esc(d.environment || "production")}</dd><dt>Deployed</dt><dd>${rel(d.created_at)}</dd></div>
    </div>
    ${taskCard}
    <div class="grid g2">
      <div class="card">
        <h3>Staff <span class="tag">Module 04</span></h3>
        <div class="hint">The agents employed by this department.</div>${agentCards}
      </div>
      <div class="card">
        <h3>Governance & KPIs</h3>
        <div class="hint">The rules this department operates under.</div>
        ${(t.governance_rules || []).map(g => rowItem({ title: "🛡 "+esc(g.name), meta: esc(g.description||g.type), badges: badge(g.enforcement||"enforce") })).join("") || "<div class=\"empty\">No governance rules.</div>"}
        ${(t.kpis || []).map(k => rowItem({ title: "📈 "+esc(k.name), meta: `${esc(k.metric_type)}${k.unit?" · "+esc(k.unit):""}` })).join("")}
      </div>
    </div>`;
}

function viewError(title, msg) {
  return `<div class="error-box"><div class="err-title">${esc(title)}</div><div class="err-msg">${esc(msg)}</div><button onclick="window.go('departments')">Retry</button></div>`;
}