// Reports — how the departments actually performed.
//
// Every number here is computed from the real ledger: service requests (with
// their SLA clocks and token counts) and human-gate turnarounds. KPI
// definitions ship on the templates; measured values arrive when the
// measurement endpoint lands — until then they are honestly marked.
import { esc, rel, statCard, toast } from "../ui.js";
import { unwrapList, listDepartments, getDepartment, listDeptRequests, listHumanTasks, getDeptMeasurements } from "../api.js";

const PERIODS = { "7d": 7 * 864e5, "30d": 30 * 864e5, "all": Infinity };

export default async function viewReports(period) {
  period = PERIODS[period] ? period : "30d";
  const cutoff = Date.now() - PERIODS[period];

  const [deptR, tasksR] = await Promise.allSettled([listDepartments(1, 50), listHumanTasks()]);
  const depts = (deptR.status === "fulfilled" ? unwrapList(deptR.value) : [])
    .filter(d => d.status === "operational" || d.status === "degraded");
  const gates = (tasksR.status === "fulfilled" ? unwrapList(tasksR.value, "tasks") : [])
    .filter(t => t.responded_at && new Date(t.created_at).getTime() >= cutoff);

  const details = (await Promise.allSettled(depts.map(d => getDepartment(d.id))))
    .filter(r => r.status === "fulfilled" && r.value.ok).map(r => r.value.data);
  const reqLists = await Promise.allSettled(depts.map(d => listDeptRequests(d.id)));
  const measLists = await Promise.allSettled(depts.map(d => getDeptMeasurements(d.id)));

  // ── Per-department performance from requests ──
  const perf = depts.map((dept, i) => {
    const reqs = (reqLists[i].status === "fulfilled" && reqLists[i].value.ok ? unwrapList(reqLists[i].value) : [])
      .filter(r => new Date(r.created_at).getTime() >= cutoff);
    const completed = reqs.filter(r => r.status === "completed");
    const failed = reqs.filter(r => r.status === "failed" || r.status === "cancelled");
    const respMet = reqs.filter(r => r.first_response_at && r.sla_response_due &&
      new Date(r.first_response_at) <= new Date(r.sla_response_due));
    const respMeasured = reqs.filter(r => r.first_response_at && r.sla_response_due);
    const resMet = completed.filter(r => !r.sla_resolution_due || !r.completed_at ||
      new Date(r.completed_at) <= new Date(r.sla_resolution_due));
    const cycles = completed.filter(r => r.completed_at)
      .map(r => new Date(r.completed_at) - new Date(r.created_at)).sort((a, b) => a - b);
    const tokens = reqs.reduce((s, r) => s + (r.tokens_used || 0), 0);
    const byService = {};
    reqs.forEach(r => {
      const k = r.service_name || r.service_id || "unknown";
      byService[k] = byService[k] || { total: 0, completed: 0, tokens: 0 };
      byService[k].total++;
      if (r.status === "completed") byService[k].completed++;
      byService[k].tokens += r.tokens_used || 0;
    });
    const meas = measLists[i].status === "fulfilled" && measLists[i].value.ok ? measLists[i].value.data : null;
    return { dept, detail: details.find(d => d.id === dept.id), reqs, completed, failed,
      respMet, respMeasured, resMet, cycles, tokens, byService, meas };
  }).filter(p => p.reqs.length > 0 || (p.detail?.kpis || []).length > 0);

  const allReqs = perf.flatMap(p => p.reqs);
  const allCompleted = perf.flatMap(p => p.completed);
  const allCycles = perf.flatMap(p => p.cycles).sort((a, b) => a - b);
  const gateTimes = gates.map(t => new Date(t.responded_at) - new Date(t.created_at)).sort((a, b) => a - b);
  const slaMet = perf.reduce((s, p) => s + p.respMet.length, 0);
  const slaMeasured = perf.reduce((s, p) => s + p.respMeasured.length, 0);

  window._reportPerf = perf;
  window._reportPeriod = period;

  return `<div id="reportsRoot">
    <div class="toolbar">
      <div style="display:flex;gap:6px">
        ${Object.keys(PERIODS).map(p =>
          `<button class="${p === period ? "primary" : "ghost"} sm" onclick="window.go('reports','${p}')">${p}</button>`).join("")}
      </div>
      <div style="flex:1"></div>
      <button class="ghost sm" onclick="window.reportExportCsv()">Export CSV</button>
    </div>

    <div class="stats-grid">
      ${statCard("📨", "Requests", allReqs.length, `${period} window`)}
      ${statCard("✅", "Completed", allCompleted.length, allReqs.length ? Math.round(allCompleted.length / allReqs.length * 100) + "% of requests" : "—")}
      ${statCard("⏱", "SLA first-response", slaMeasured ? Math.round(slaMet / slaMeasured * 100) + "%" : "—", slaMeasured ? `${slaMet}/${slaMeasured} inside SLA` : "no measured responses")}
      ${statCard("🔁", "Median cycle time", allCycles.length ? dur(allCycles[Math.floor(allCycles.length / 2)]) : "—", "request → completion")}
      ${statCard("🧑‍⚖️", "Gate turnaround", gateTimes.length ? dur(gateTimes[Math.floor(gateTimes.length / 2)]) : "—", gateTimes.length ? `median of ${gateTimes.length} decisions` : "no decided gates")}
    </div>

    ${perf.length === 0
      ? `<div class="card"><div class="empty">No department activity in this window.</div></div>`
      : perf.map(p => deptReport(p)).join("")}
  </div>`;
}

function deptReport(p) {
  const kpis = p.detail?.kpis || [];
  const respPct = p.respMeasured.length ? Math.round(p.respMet.length / p.respMeasured.length * 100) : null;
  const resPct = p.completed.length ? Math.round(p.resMet.length / p.completed.length * 100) : null;
  const median = p.cycles.length ? dur(p.cycles[Math.floor(p.cycles.length / 2)]) : "—";
  return `<div class="card" style="margin-bottom:18px">
    <h3>${esc(p.dept.name)} <span class="tag">${p.reqs.length} request(s)</span></h3>
    <div class="m" style="margin:6px 0 10px">
      ${p.completed.length} completed · ${p.failed.length} failed/cancelled
      ${respPct != null ? ` · first response in SLA <b>${respPct}%</b>` : ""}
      ${resPct != null ? ` · resolved in SLA <b>${resPct}%</b>` : ""}
      · median cycle <b>${median}</b> · ${p.tokens.toLocaleString()} tokens</div>
    ${Object.entries(p.byService).map(([svc, v]) => `
      <div class="row-item">
        <div class="grow"><div class="t">${esc(svc)}</div>
        <div class="m">${v.total} request(s) · ${v.completed} completed · ${v.tokens.toLocaleString()} tokens</div></div>
      </div>`).join("")}
    ${kpis.length ? `
      <div class="hint" style="margin:12px 0 4px">KPI definitions — Measured = a ledger metric genuinely backs it
      (30d window, server-computed).</div>
      ${kpis.slice(0, 8).map(k => {
        const m = ((p.meas && p.meas.kpi_measurements) || []).find(x => x.kpi_id === k.id);
        return `
        <div class="row-item">
          <div class="grow"><div class="t">📈 ${esc(k.name)}</div>
          <div class="m">${esc(k.metric_type || "")}${k.unit ? " · " + esc(k.unit) : ""}</div></div>
          <div class="actions">${m && m.measured
            ? `<span class="sla-chip ok" title="${esc(m.source || "")}">${esc(String(m.value))} ${esc(m.unit || "")}</span>`
            : `<span class="badge draft">not yet measured</span>`}</div>
        </div>`;}).join("")}` : ""}
  </div>`;
}

function dur(ms) {
  const s = Math.round(ms / 1000);
  if (s < 60) return s + "s";
  if (s < 3600) return Math.floor(s / 60) + "m " + (s % 60) + "s";
  if (s < 86400) return Math.floor(s / 3600) + "h " + Math.floor((s % 3600) / 60) + "m";
  return Math.floor(s / 86400) + "d " + Math.floor((s % 86400) / 3600) + "h";
}

// Real export: the computed per-service table, straight to a CSV download.
window.reportExportCsv = function () {
  const perf = window._reportPerf || [];
  if (perf.length === 0) { toast("Nothing to export in this window", "warn"); return; }
  const rows = [["department", "service", "requests", "completed", "tokens"]];
  perf.forEach(p => Object.entries(p.byService).forEach(([svc, v]) =>
    rows.push([p.dept.name, svc, v.total, v.completed, v.tokens])));
  const csv = rows.map(r => r.map(c => `"${String(c).replace(/"/g, '""')}"`).join(",")).join("\n");
  const a = document.createElement("a");
  a.href = URL.createObjectURL(new Blob([csv], { type: "text/csv" }));
  a.download = `operan-report-${window._reportPeriod || "30d"}.csv`;
  a.click();
  URL.revokeObjectURL(a.href);
  toast("Report exported", "ok");
};
