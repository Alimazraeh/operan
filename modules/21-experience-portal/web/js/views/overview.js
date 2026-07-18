// Overview: the operating picture across the whole platform.
import { SVC, get } from "../api.js";
import { esc, badge, rel, rowItem, eventRow } from "../ui.js";

export async function viewOverview() {
  let agR, spR, hR, qR, tR;
  try {
    [agR, spR, hR, qR, tR] = await Promise.allSettled([
      get(SVC.registry + "/registry/agents?page_size=1"),
      get(SVC.observability + "/spans?page_size=12"),
      get(SVC.observability + "/health"),
      get(SVC.supervision + "/queue?page_size=5"),
      get(SVC.templates + "/templates?page_size=50"),
    ]);
  } catch (e) { return viewError("Failed to load overview", e.message); }

  const ag = (agR.status==="fulfilled" && agR.value.data) ? agR.value.data : null;
  const spans = (spR.status==="fulfilled" && spR.value.data && spR.value.data.items) || [];
  const health = (hR.status==="fulfilled" && hR.value.data) ? hR.value.data : {overall_status:"unknown",components:[]};
  const queue = (qR.status==="fulfilled" && qR.value.data && qR.value.data.items) || [];

  // Count deployments
  let deployments = 0;
  if (tR.status==="fulfilled" && tR.value.data && tR.value.data.items) {
    for (const t of tR.value.data.items) {
      const d = await get(`${SVC.templates}/templates/${t.id}/deployments`);
      if (d.ok && d.data && d.data.items) deployments += d.data.items.length;
    }
  }

  return `
    <div class="grid g4" style="margin-bottom:18px">
      <div class="card metric"><b>${deployments}</b><span>departments deployed</span></div>
      <div class="card metric"><b>${ag ? ag.total ?? 0 : 0}</b><span>agents employed</span></div>
      <div class="card metric"><b>${queue.length}</b><span>decisions waiting</span></div>
      <div class="card metric"><b>${badge(health.overall_status || "healthy")}</b><span>platform health</span></div>
    </div>

    <div class="grid g2">
      <div class="card">
        <h3>Needs your attention <span class="tag">Module 09</span></h3>
        <div class="hint">Agent work paused on a human decision.</div>
        ${queue.map(it => rowItem({
          title: "⏸ " + esc(it.title || it.item_type),
          meta: `${esc(it.item_type)} · ${rel(it.created_at)}`,
          badges: badge(it.status),
          click: `window.go('supervision')`,
        })).join("") || `<div class="empty">Nothing waiting — your departments are unblocked.</div>`}
      </div>

      <div class="card">
        <h3><span class="pulse-dot"></span>Platform activity <span class="tag">live</span></h3>
        <div class="hint">What your departments are doing right now.</div>
        <div style="max-height:320px;overflow:auto">${spans.map(eventRow).join("") || `<div class="empty">Quiet — deploy a department or run the story.</div>`}</div>
      </div>
    </div>

    <div class="card" style="margin-top:16px">
      <h3>Sovereign by design</h3>
      <div class="hint">Everything on this page — nine platform services, the Kafka event mesh, the Qwen models doing embeddings, and this portal — is running on <b>this machine</b>. No data leaves it. Multi-tenant isolation is enforced on every request via JWT + tenant scoping.</div>
    </div>`;
}

function viewError(title, msg) {
  return `<div class="error-box"><div class="err-title">${esc(title)}</div><div class="err-msg">${esc(msg)}</div><button onclick="window.go('overview')">Retry</button></div>`;
}