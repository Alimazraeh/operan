// Tools: the execution layer — what agents are allowed to do (Module 08).
import {SVC, get, post} from "../api.js";
import {$, esc, badge, rel, toast, rowItem} from "../ui.js";

export async function viewTools() {
  const [tR, eR] = await Promise.all([
    get(SVC.tools + "/tools?page_size=50"),
    get(SVC.tools + "/executions?page_size=20"),
  ]);
  const tools = (tR.data && tR.data.items) || [];
  const execs = (eR.data && eR.data.items) || [];

  const tRows = tools.length === 0
    ? `<div class="empty">No tools registered for this tenant.</div>`
    : tools.map(t => rowItem({
        title: `🛠 ${esc(t.name)}`,
        meta: `${esc(t.category || "uncategorized")} · v${esc(t.version || "1.0.0")} · ${esc(t.description || "")}`,
        badges: badge(t.status || "active"),
        actions: `<button class="sm ghost" onclick="window.execTool('${esc(t.name)}')">Execute</button>`,
      })).join("");

  const eRows = execs.length === 0
    ? `<div class="empty">No executions yet.</div>`
    : execs.map(x => rowItem({
        title: `${esc(x.tool || x.tool_id || "tool")}`,
        meta: `by ${esc((x.agent_id || "").slice(0, 12))} · ${rel(x.created_at || x.requested_at)}`,
        badges: badge(x.status || "completed"),
      })).join("");

  return `
    <div class="card" style="margin-bottom:14px">
      <h3>Tool registry <span class="tag">Module 08 · execution layer</span></h3>
      <div class="hint">The actions agents may take in the world. Every execution is recorded and costed.</div>
      ${tRows}
      <div class="grid g3" style="margin-top:10px">
        <div><label>Name</label><input id="tlName" placeholder="send_email"></div>
        <div><label>Category</label><input id="tlCat" placeholder="communication"></div>
        <div><label>Description</label><input id="tlDesc" placeholder="Send an email via relay"></div>
      </div>
      <div style="margin-top:12px"><button class="sm" onclick="window.registerTool()">Register tool</button></div>
    </div>
    <div class="card"><h3>Execution log</h3>
      <div class="hint">Auditable record of every tool an agent has used.</div>${eRows}</div>`;
}

window.registerTool = async function () {
  const name = $("tlName").value.trim();
  if (!name) return;
  const r = await post(SVC.tools + "/tools/register", {
    name, category: $("tlCat").value.trim() || "general",
    description: $("tlDesc").value.trim(),
  });
  if (r.ok) { toast(`Tool ${esc(name)} registered`, "ok"); window.go("tools"); }
  else toast("Register failed: " + esc(JSON.stringify(r.data).slice(0, 100)), "bad");
};

window.execTool = async function (name) {
  const r = await post(SVC.tools + "/execute", {
    tool: name, agent_id: "portal-user", parameters: {invoked_from: "operan-portal"},
  });
  if (r.ok) toast(`Executed ${esc(name)}`, "ok"); else toast("Execution failed", "bad");
  window.go("tools");
};
