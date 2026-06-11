// Overview: the operating picture across the whole platform.
import {SVC, get} from "../api.js";
import {esc, badge, rel, rowItem, eventRow} from "../ui.js";

export async function viewOverview() {
  const [agR, spR, hR, qR, tR] = await Promise.all([
    get(SVC.registry + "/registry/agents?page_size=1"),
    get(SVC.observability + "/spans?page_size=12"),
    get(SVC.observability + "/health"),
    get(SVC.supervision + "/queue?page_size=5"),
    get(SVC.templates + "/templates?page_size=50"),
  ]);
  const spans = (spR.data && spR.data.items) || [];
  const health = hR.data || {components: []};
  const queue = (qR.data && qR.data.items) || [];

  // Count deployments across templates (operational departments).
  let deployments = 0;
  for (const t of ((tR.data && tR.data.items) || [])) {
    const d = await get(`${SVC.templates}/templates/${t.id}/deployments`);
    deployments += ((d.data && d.data.items) || []).length;
  }

  return `
    <div class="grid g4" style="margin-bottom:14px">
      <div class="card metric"><b>${deployments}</b><span>departments deployed</span></div>
      <div class="card metric"><b>${agR.data ? agR.data.total ?? 0 : 0}</b><span>agents employed</span></div>
      <div class="card metric"><b>${queue.length}</b><span>decisions waiting on you</span></div>
      <div class="card metric"><b>${badge(health.overall_status || "healthy")}</b><span>platform health</span></div>
    </div>
    <div class="grid g2">
      <div class="card">
        <h3>Needs your attention <span class="tag">Module 09</span></h3>
        <div class="hint">Agent work paused on a human decision.</div>
        ${queue.map(it => rowItem({
          title: `⏸ ${esc(it.title || it.item_type)}`,
          meta: `${esc(it.item_type)} · ${rel(it.created_at)}`,
          badges: badge(it.status),
          click: `window.go('supervision')`,
        })).join("") || `<div class="empty">Nothing waiting — your departments are unblocked.</div>`}
        <div class="hint" style="margin-top:8px">Go to <a style="color:var(--accent);cursor:pointer" onclick="window.go('supervision')">Supervision</a> to decide.</div>
      </div>
      <div class="card">
        <h3><span class="pulse"></span>Platform activity <span class="tag">live</span></h3>
        <div class="hint">What your departments are doing right now.</div>
        <div style="max-height:320px;overflow:auto">${spans.map(eventRow).join("") || `<div class="empty">Quiet. Deploy a department or run the story.</div>`}</div>
      </div>
    </div>
    <div class="card" style="margin-top:14px">
      <h3>Sovereign by design</h3>
      <div class="hint">Everything on this page — nine platform services, the Kafka event mesh, the Qwen
      models doing embeddings, and this portal — is running on <b>this machine</b>. No data leaves it.
      Multi-tenant isolation is enforced on every request via JWT + tenant scoping.</div>
    </div>`;
}
