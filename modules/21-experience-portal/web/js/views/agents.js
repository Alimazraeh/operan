// Agents: the registry (Module 04) + each agent's memory (Module 07).
import {SVC, get, post, uuid4, session} from "../api.js";
import {$, esc, badge, rel, toast, rowItem} from "../ui.js";

export async function viewAgents() {
  const r = await get(SVC.registry + "/registry/agents?page_size=100");
  const agents = (r.data && r.data.items) || [];

  const rows = agents.length === 0
    ? `<div class="empty">No agents yet — deploy a department, or register one below.</div>`
    : agents.map(a => rowItem({
        title: `🤖 ${esc(a.name)}`,
        meta: `${esc(a.role)} · ${(a.capabilities || []).map(esc).join(", ") || "—"} · v${esc(a.version || "1.0.0")}`,
        badges: badge(a.status || "active"),
        click: `window.go('agent','${a.id}')`,
      })).join("");

  return `
    <div class="card" style="margin-bottom:14px">
      <h3>Workforce <span class="tag">Module 04 · agent registry</span></h3>
      <div class="hint">Every agent employed across your departments — versioned, capability-indexed, governed.</div>
      ${rows}
    </div>
    <div class="card">
      <h3>Register an agent</h3>
      <div class="hint">Usually agents arrive via department deployment; direct registration is for specialists.</div>
      <div class="grid g3">
        <div><label>Name</label><input id="agName" placeholder="research-analyst"></div>
        <div><label>Role</label><input id="agRole" placeholder="research"></div>
        <div><label>Capabilities (comma-sep)</label><input id="agCaps" placeholder="web_research, summarize"></div>
      </div>
      <div style="margin-top:12px"><button class="sm" onclick="window.registerAgent()">Register</button></div>
    </div>`;
}

window.registerAgent = async function () {
  const name = $("agName").value.trim(), role = $("agRole").value.trim();
  if (!name || !role) { toast("Name and role are required", "bad"); return; }
  const caps = $("agCaps").value.split(",").map(s => s.trim()).filter(Boolean);
  const r = await post(SVC.registry + "/registry/agents", {
    id: uuid4(), tenant_id: session.tenant, name, role,
    version: "1.0.0", capabilities: caps, tools: [],
  });
  if (r.ok) { toast(`Agent ${esc(name)} registered`, "ok"); window.go("agents"); }
  else toast("Registration failed: " + esc(JSON.stringify(r.data).slice(0, 120)), "bad");
};

export async function viewAgent(agentId) {
  const [ar, memR, vecR] = await Promise.all([
    get(`${SVC.registry}/registry/agents/${agentId}`),
    get(`${SVC.memory}/agents/${agentId}`),
    get(`${SVC.memory}/vectors?embedding_type=agent_personal&page_size=100`),
  ]);
  const a = ar.data || {};
  const mem = memR.ok ? memR.data : null;
  const myVectors = ((vecR.data && vecR.data.items) || [])
    .filter(v => (v.metadata || {}).agent_id === agentId);

  const memRows = myVectors.length === 0
    ? `<div class="empty">This agent has no personal memories yet.</div>`
    : myVectors.map(v => rowItem({
        title: esc(v.semantic_content.length > 90 ? v.semantic_content.slice(0, 90) + "…" : v.semantic_content),
        meta: `${esc(v.embedding_model || "no embedding")} · ${rel(v.created_at)}`,
      })).join("");

  return `
    <span class="back" onclick="window.go('agents')">← Workforce</span>
    <div class="card" style="margin-bottom:14px">
      <h3>🤖 ${esc(a.name || agentId)} ${badge(a.status || "active")}</h3>
      <div class="kv" style="margin-top:10px">
        <dt>Role</dt><dd>${esc(a.role || "—")}</dd>
        <dt>Capabilities</dt><dd>${(a.capabilities || []).map(esc).join(", ") || "—"}</dd>
        <dt>Tools</dt><dd>${(a.tools || []).map(esc).join(", ") || "—"}</dd>
        <dt>Version</dt><dd>${esc(a.version || "1.0.0")}</dd>
        <dt>Department</dt><dd>${esc(a.department_id || "—")}</dd>
        <dt>Memory state</dt><dd>${mem ? `${(mem.personal_memories || []).length} personal memories · window ${mem.ephemeral_window ? mem.ephemeral_window.max_tokens + " tokens" : "default"}` : "no memory recorded yet"}</dd>
      </div>
    </div>
    <div class="grid g2">
      <div class="card">
        <h3>Teach this agent <span class="tag">Module 07</span></h3>
        <div class="hint">Memories are embedded on-cluster (qwen3) and recalled by meaning.</div>
        <textarea id="agMemText" rows="2" placeholder="Customer Acme prefers Arabic-first UI and quarterly billing"></textarea>
        <div style="margin-top:10px"><button class="sm" onclick="window.agentRemember('${esc(agentId)}')">Remember</button></div>
        <label style="margin-top:16px">Ask the agent's memory</label>
        <div class="frow">
          <input id="agMemQ" placeholder="which interface language does the client like">
          <button class="sm" onclick="window.agentAsk('${esc(agentId)}')">Ask</button>
        </div>
        <div id="agMemOut"></div>
      </div>
      <div class="card">
        <h3>Personal memories</h3>
        <div class="hint">${myVectors.length} stored for this agent.</div>
        ${memRows}
      </div>
    </div>`;
}

window.agentRemember = async function (agentId) {
  const text = $("agMemText").value.trim();
  if (!text) return;
  const r = await post(SVC.memory + "/vectors", {items: [{
    document_id: uuid4(), embedding_type: "agent_personal",
    semantic_content: text, metadata: {agent_id: agentId},
  }]});
  if (r.ok) { toast("Memory stored & embedded", "ok"); window.go("agent", agentId); }
};

window.agentAsk = async function (agentId) {
  const q = $("agMemQ").value.trim();
  if (!q) return;
  const r = await post(SVC.memory + "/search", {
    query: q, embedding_type: "agent_personal", relevance_threshold: 0.3,
  });
  const items = ((r.data && r.data.items) || []).filter(i => true);
  if (items.length === 0) {
    $("agMemOut").innerHTML = `<div class="empty">Nothing relevant in memory.</div>`;
    return;
  }
  const top = items[0];
  $("agMemOut").innerHTML = `<div class="result">
    <div class="q">“${esc(q)}”</div><div class="a">${esc(top.content)}</div>
    <div class="scorebar"><div style="width:${Math.round(top.score * 100)}%"></div></div>
    <div class="meta"><span>relevance ${(top.score * 100).toFixed(0)}%</span>
      <span>${esc(top.embedding_model || "fallback")}</span></div></div>`;
};
