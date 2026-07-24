// Reports — Hourly, daily, weekly, monthly operational reports
import { $, esc, statCard, card, btn, toast } from "../ui.js";
import { listSpans, listCostEvents, unwrapList } from "../api.js";

export default async function viewReports(period) {
  period = period || "daily";
  const from = getPeriodStart(period);

  const [spansRes, costsRes] = await Promise.allSettled([
    listSpans(null, from.toISOString(), new Date().toISOString(), 1, 500),
    listCostEvents(1, 100),
  ]);

  const spanData = spansRes.status === 'fulfilled' ? unwrapList(spansRes.value, "items") : [];
  const costData = costsRes.status === 'fulfilled' ? unwrapList(costsRes.value, "events") : [];

  const byStatus = {};
  spanData.forEach(s => { const st = s.status || "unknown"; byStatus[st] = (byStatus[st] || 0) + 1; });

  const totalSpans = spanData.length;
  const errorSpans = byStatus["error"] || byStatus["failed"] || 0;
  const successRate = totalSpans > 0 ? ((totalSpans - errorSpans) / totalSpans * 100).toFixed(1) : "—";
  const totalCost = costData.reduce((sum, c) => sum + (c.amount || c.cost || c.usage || 0), 0);

  return `
    <div class="toolbar">
      <div style="display:flex;gap:6px">
        ${timeBtn(period, "hourly")}${timeBtn(period, "daily")}${timeBtn(period, "weekly")}${timeBtn(period, "monthly")}
      </div>
      <div style="flex:1"></div>
      ${btn("Export CSV", "ghost sm", "toast('Export — requires backend CSV endpoint', 'info')")}
    </div>

    <div class="stats-grid">
      ${statCard("📡", "Total Events", totalSpans, "Operational events")}
      ${statCard("✅", "Success Rate", successRate + "%", totalSpans + " events tracked")}
      ${statCard("❌", "Errors", errorSpans, "Failed events")}
      ${statCard("💰", "Total Cost", "$" + totalCost.toFixed(2), "In the selected period")}
    </div>

    ${card("Event Breakdown", `Events by status (${period})`, `
      <div class="card-body">
        ${Object.keys(byStatus).length === 0
          ? "<div class='empty'>No data for this period</div>"
          : `<div class="kv">${Object.entries(byStatus).map(([s, c]) => `<dt>${s}</dt><dd>${c}</dd>`).join("")}</div>`}
      </div>
    `)}

    ${card("Recent Cost Events", "Last 20", `
      <div class="card-body" style="padding:0">
        ${costData.slice(0, 20).length === 0
          ? "<div class='empty'>No cost data</div>"
          : costData.slice(0, 20).map(costRow).join("")}
      </div>
    `)}

    ${card("Recent Activity", "Latest events", `
      <div class="card-body" style="padding:0">
        ${spanData.slice(0, 15).length === 0
          ? "<div class='empty'>No activity data</div>"
          : spanData.slice(0, 15).map(spanRow).join("")}
      </div>
    `)}
  `;
}

function timeBtn(active, period) {
  return `<button class="${active === period ? 'primary' : 'ghost'} sm" onclick="window.go('reports', '${period}')">${period}</button>`;
}

function getPeriodStart(period) {
  const now = new Date();
  const from = new Date(now);
  if (period === "hourly") from.setHours(now.getHours() - 1);
  else if (period === "daily") from.setDate(now.getDate() - 1);
  else if (period === "weekly") from.setDate(now.getDate() - 7);
  else if (period === "monthly") from.setMonth(now.getMonth() - 1);
  return from;
}

function costRow(c) {
  const amount = c.amount || c.cost || c.usage || 0;
  return `<div class="row-item">
    <div class="grow"><div class="t">${esc(c.agent_name || c.agent_id || c.entity || "Unknown")}</div>
    <div class="m">${esc(c.event_type || c.type || "cost")}${c.description ? ` · ${esc(c.description)}` : ""}</div></div>
    <div style="font-weight:700;color:var(--gold)">$${parseFloat(amount).toFixed(2)}</div>
  </div>`;
}

function spanRow(s) {
  const statusBadge = (s.status === "ok" || s.status === "success") ? "ok"
    : (s.status === "error" || s.status === "failed") ? "error" : "pending";
  return `<div class="row-item">
    <div class="grow"><div class="t">${esc(s.span_name || s.name || "Event")}</div>
    <div class="m">${esc(s.service || "unknown")}${s.trace_id ? ` · trace ${esc(s.trace_id.slice(0, 8))}` : ""}
    ${s.start_time ? ` · ${new Date(s.start_time).toLocaleTimeString()}` : ""}</div></div>
    <span class="badge ${statusBadge}">${esc(s.status || "unknown")}</span>
  </div>`;
}