// Observability: traces, alerts, component health (Module 11).
import {SVC, get, post} from "../api.js";
import {esc, badge, rel, toast, rowItem, eventRow} from "../ui.js";

export async function viewObservability() {
  const [spR, hR, aR, mR] = await Promise.all([
    get(SVC.observability + "/spans?page_size=40"),
    get(SVC.observability + "/health"),
    get(SVC.observability + "/alerts?page_size=15"),
    get(SVC.observability + "/metrics?metric_name=operan.events.consumed&page_size=1"),
  ]);
  const spans = (spR.data && spR.data.items) || [];
  const health = hR.data || {components: []};
  const alerts = (aR.data && aR.data.items) || [];

  // Group spans into traces for the trace list.
  const traces = {};
  for (const s of spans) {
    (traces[s.trace_id] = traces[s.trace_id] || []).push(s);
  }
  const traceRows = Object.entries(traces).slice(0, 8).map(([tid, ss]) => rowItem({
    title: `⛓ trace ${esc(tid.slice(0, 8))}`,
    meta: ss.map(s => esc(s.span_type)).join(" → ") + ` · ${ss.length} span(s)`,
    badges: ss.some(s => s.status === "error") ? badge("error") : badge("ok"),
  })).join("") || `<div class="empty">No traces yet.</div>`;

  const alertRows = alerts.length === 0
    ? `<div class="empty">No alerts — quiet skies.</div>`
    : alerts.map(a => rowItem({
        title: `🚨 ${esc(a.alert_name)}`,
        meta: `${esc(a.condition_description || "")} · ${rel(a.triggered_at)}`,
        badges: badge(a.severity) + (a.resolved_at ? badge("resolved") : badge("open")),
        actions: a.resolved_at ? "" :
          `<button class="sm ghost" onclick="window.resolveAlert('${a.id}')">Resolve</button>`,
      })).join("");

  return `
    <div class="grid g4" style="margin-bottom:14px">
      <div class="card metric"><b>${spR.data ? spR.data.total : 0}</b><span>spans recorded</span></div>
      <div class="card metric"><b>${mR.data ? mR.data.total : 0}</b><span>events consumed off the bus</span></div>
      <div class="card metric"><b>${alerts.filter(a => !a.resolved_at).length}</b><span>open alerts</span></div>
      <div class="card metric"><b>${badge(health.overall_status || "healthy")}</b><span>overall health</span></div>
    </div>
    <div class="grid g2" style="margin-bottom:14px">
      <div class="card">
        <h3>Component health <span class="tag">derived from event flow</span></h3>
        <div class="hint">Nothing self-reports: a component is healthy because its events keep arriving.</div>
        ${(health.components || []).map(c => rowItem({
          title: esc(c.component_id), meta: esc(c.component_type) + (c.reason ? " · " + esc(c.reason) : ""),
          badges: badge(c.new_status),
        })).join("") || `<div class="empty">Components appear as their events flow.</div>`}
      </div>
      <div class="card"><h3>Alerts</h3>
        <div class="hint">Fired automatically on failure events; resolve when handled.</div>${alertRows}</div>
    </div>
    <div class="grid g2">
      <div class="card"><h3>Recent traces</h3>
        <div class="hint">Cross-service flows grouped by correlation id.</div>${traceRows}</div>
      <div class="card"><h3><span class="pulse"></span>Activity stream <span class="tag">Module 11 consumer</span></h3>
        <div class="hint">Every platform action, as observed on the Kafka bus.</div>
        <div style="max-height:340px;overflow:auto">${spans.map(eventRow).join("") || `<div class="empty">Waiting for activity…</div>`}</div></div>
    </div>`;
}

window.resolveAlert = async function (id) {
  await post(`${SVC.observability}/alerts/${id}/resolve`);
  toast("Alert resolved", "ok");
  window.go("observability");
};
