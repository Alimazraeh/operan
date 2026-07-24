// Costs — what the automation actually costs (Module 17 + the request ledger).
//
// The hard numbers today are TOKENS: every completed run records tokens_used
// on its service request. The page leads with that truth, prices it at an
// adjustable rate, and layers Module 17 governance on top (budgets, events,
// alerts, throttle). Cost events populate when modules publish them.
import { SVC, get, post, patch, del, unwrapList, listDepartments, listDeptRequests } from "../api.js";
import { $, esc, badge, rel, toast } from "../ui.js";

const RATE_KEY = "operan.tokenRateUsdPerM";
const rate = () => parseFloat(localStorage.getItem(RATE_KEY)) || 3.0;

export async function viewCost() {
  const [summaryR, budgetsR, eventsR, alertsR, throttleR, deptR] = await Promise.allSettled([
    get(SVC.cost + "/v1/summary"),
    get(SVC.cost + "/v1/budgets"),
    get(SVC.cost + "/v1/cost-events?page_size=50"),
    get(SVC.cost + "/v1/alerts"),
    get(SVC.cost + "/v1/throttle"),
    listDepartments(1, 50),
  ]);
  const ok = r => r.status === "fulfilled" ? r.value : null;
  const budgets = unwrapList(ok(budgetsR), "budgets");
  const events = unwrapList(ok(eventsR), "events");
  const alerts = unwrapList(ok(alertsR), "alerts");
  const throttle = ok(throttleR)?.data || {};
  const depts = unwrapList(ok(deptR)).filter(d => d.status === "operational" || d.status === "degraded");

  // The truth: tokens recorded on every request the work loop executed.
  const reqLists = await Promise.allSettled(depts.map(d => listDeptRequests(d.id)));
  const spendRows = [];
  let totalTokens = 0;
  reqLists.forEach((r, i) => {
    if (r.status !== "fulfilled" || !r.value.ok) return;
    const byService = {};
    for (const req of unwrapList(r.value)) {
      if (!req.tokens_used) continue;
      totalTokens += req.tokens_used;
      const k = req.service_name || req.service_id || "unknown";
      byService[k] = byService[k] || { tokens: 0, runs: 0 };
      byService[k].tokens += req.tokens_used;
      byService[k].runs++;
    }
    Object.entries(byService).forEach(([svc, v]) => spendRows.push({ dept: depts[i], svc, ...v }));
  });
  spendRows.sort((a, b) => b.tokens - a.tokens);
  const usd = t => (t / 1e6 * rate()).toFixed(2);

  window._costAgents = null;

  return `<div id="costRoot">
    <div class="grid g4" style="margin-bottom:18px">
      <div class="card metric"><b>${totalTokens.toLocaleString()}</b><span>tokens consumed (from requests)</span></div>
      <div class="card metric"><b>$${usd(totalTokens)}</b><span>est. at $${rate()}/1M tokens</span></div>
      <div class="card metric"><b>${budgets.length}</b><span>budgets</span></div>
      <div class="card metric"><b>${alerts.length}</b><span>alerts</span></div>
    </div>

    <div class="card" style="margin-bottom:18px">
      <h3>LLM spend by service <span class="tag">real tokens from the request ledger</span></h3>
      <div class="hint">Summed from tokens_used on every executed request. Estimated in USD at an adjustable
      blended rate:
        <input id="tokenRate" type="number" step="0.1" min="0" value="${rate()}" style="width:70px;margin:0 4px"> $/1M tokens
        <button class="sm ghost" onclick="window.costSetRate()">Apply</button></div>
      ${spendRows.length === 0
        ? `<div class="empty">No token usage recorded yet — run a request from Tasks or Workflows.</div>`
        : spendRows.map(s => `
          <div class="row-item">
            <div class="grow"><div class="t">${esc(s.svc)} <span class="tag">${esc(s.dept.name)}</span></div>
            <div class="m">${s.runs} run(s) · ${s.tokens.toLocaleString()} tokens</div></div>
            <div class="actions" style="font-weight:700;color:var(--warn)">$${usd(s.tokens)}</div>
          </div>`).join("")}
    </div>

    <div style="display:flex;gap:8px;margin-bottom:18px;flex-wrap:wrap">
      <button class="sm cost-tab active" onclick="window.costTab('budgets', this)">Budgets</button>
      <button class="sm cost-tab" onclick="window.costTab('events', this)">Cost events</button>
      <button class="sm cost-tab" onclick="window.costTab('alerts', this)">Alerts</button>
      <button class="sm cost-tab" onclick="window.costTab('throttle', this)">Throttle</button>
    </div>

    <div class="cost-panel" id="panel-budgets">
      <div class="card" style="margin-bottom:18px">
        <h3>Budgets <span class="tag">Module 17</span></h3>
        <div class="hint">Spending limits with soft/hard thresholds. Utilization fills as cost events are ingested.</div>
        <div class="frow" style="margin-bottom:14px">
          <input id="budgetDesc" placeholder="what is this budget for? (e.g. IT department monthly)" style="flex:2">
          <input id="budgetAmount" placeholder="USD" type="number" min="1" style="max-width:100px">
          <select id="budgetPeriod" style="max-width:110px"><option>monthly</option><option>weekly</option><option>daily</option></select>
          <button class="sm" onclick="window.costCreateBudget()">Create budget</button>
        </div>
        ${budgets.length === 0
          ? `<div class="empty">No budgets yet.</div>`
          : budgets.map(b => {
              const amount = b.budget_amount || 0;
              const spent = b.spent || b.current_spend || 0;
              const pct = amount > 0 ? Math.round((spent / amount) * 100) : 0;
              return `<div class="row-item">
                <div class="grow"><div class="t">💰 ${esc(b.description || b.id.slice(0, 8))} <span class="tag">${esc(b.period || "monthly")}</span></div>
                <div class="m">$${spent} of $${amount} (${pct}%) · soft ${esc(String(b.soft_limit_pct ?? 80))}% · hard ${esc(String(b.hard_limit_pct ?? 95))}%</div>
                <div style="background:rgba(255,255,255,0.06);border-radius:6px;height:8px;overflow:hidden;margin-top:6px;max-width:420px">
                  <div style="width:${Math.min(pct, 100)}%;height:100%;background:${pct > 95 ? "var(--bad)" : pct > 80 ? "var(--warn)" : "var(--ok)"}"></div></div></div>
                <div class="actions"><span class="badge ${b.is_active !== false ? "active" : "expired"}">${b.is_active !== false ? "active" : "inactive"}</span>
                  <button class="bad sm" onclick="window.costDeleteBudget('${esc(b.id)}')">Delete</button></div>
              </div>`;
            }).join("")}
      </div>
    </div>

    <div class="cost-panel" id="panel-events" style="display:none">
      <div class="card">
        <h3>Cost events <span class="tag">Module 17 ingestion</span></h3>
        <div class="hint">Per-call cost records (provider, model, tokens, USD). None flow yet — token usage is
        currently tracked on requests (the card above); modules will publish events here as metering lands.</div>
        ${events.length === 0
          ? `<div class="empty">No cost events ingested.</div>`
          : events.map(e => `
            <div class="row-item">
              <div class="grow"><div class="t">${esc(e.model || e.provider || "event")}</div>
              <div class="m">${esc(String(e.tokens || e.input_tokens || 0))} tokens · ${rel(e.timestamp || e.created_at)}</div></div>
              <div class="actions" style="font-weight:700">$${esc(String(e.cost || e.total_cost || 0))}</div>
            </div>`).join("")}
      </div>
    </div>

    <div class="cost-panel" id="panel-alerts" style="display:none">
      <div class="card">
        <h3>Budget alerts <span class="tag">Module 17</span></h3>
        ${alerts.length === 0
          ? `<div class="empty">No alerts — no budget has crossed a threshold.</div>`
          : alerts.map(a => `
            <div class="row-item">
              <div class="grow"><div class="t">${a.severity === "critical" ? "🔴" : "🟡"} ${esc(a.message || a.alert_type || "alert")}</div>
              <div class="m">${rel(a.triggered_at || a.created_at)}</div></div>
              <div class="actions">${badge(a.severity || "warning")}</div>
            </div>`).join("")}
      </div>
    </div>

    <div class="cost-panel" id="panel-throttle" style="display:none">
      <div class="card">
        <h3>Emergency throttle <span class="tag">Module 17</span></h3>
        <div class="hint">Tenant-wide spending brake: <b>soft</b> warns on new spend, <b>hard</b> blocks it.</div>
        <p style="margin:10px 0">${
          (throttle.throttle_state === "hard") ? `<span class="badge rejected">HARD — spend blocked</span>`
          : (throttle.throttle_state === "soft") ? `<span class="badge pending">SOFT — spend warned</span>`
          : `<span class="badge active">none — spend allowed</span>`}</p>
        <div style="display:flex;gap:8px">
          ${throttle.throttle_state === "soft" ? "" : `<button class="sm warn" onclick="window.costSetThrottle('soft')">Soft throttle</button>`}
          ${throttle.throttle_state === "hard" ? "" : `<button class="bad sm" onclick="window.costSetThrottle('hard')">Hard throttle</button>`}
          ${throttle.throttle_state === "hard" || throttle.throttle_state === "soft"
            ? `<button class="ok sm" onclick="window.costSetThrottle('none')">Release</button>` : ""}
        </div>
      </div>
    </div>
  </div>`;
}

window.costTab = function (name, btn) {
  document.querySelectorAll(".cost-tab").forEach(b => b.classList.remove("active"));
  btn.classList.add("active");
  document.querySelectorAll(".cost-panel").forEach(p => p.style.display = "none");
  const panel = document.getElementById("panel-" + name);
  if (panel) panel.style.display = "block";
};

window.costSetRate = function () {
  const v = parseFloat($("tokenRate").value);
  if (!(v >= 0)) { toast("Rate must be a number", "warn"); return; }
  localStorage.setItem(RATE_KEY, String(v));
  window.go("cost");
};

window.costCreateBudget = async function () {
  const description = $("budgetDesc").value.trim();
  const amount = parseFloat($("budgetAmount").value);
  if (!description) { toast("Describe the budget", "warn"); return; }
  if (!(amount > 0)) { toast("Amount must be positive", "warn"); return; }
  const r = await post(SVC.cost + "/v1/budgets", {
    description, budget_amount: amount, period: $("budgetPeriod").value,
  });
  if (r.ok) { toast("Budget created", "ok"); window.go("cost"); }
  else toast("Create failed: " + esc(r.data?.error || r.data?.error?.message || r.status), "bad");
};

window.costDeleteBudget = async function (id) {
  if (!confirm("Delete this budget?")) return;
  const r = await del(SVC.cost + "/v1/budgets/" + encodeURIComponent(id));
  if (r.ok) { toast("Budget deleted", "ok"); window.go("cost"); }
  else toast("Delete failed: " + esc(r.data?.error || r.status), "bad");
};

window.costSetThrottle = async function (level) {
  const r = await patch(SVC.cost + "/v1/throttle/" + level);
  if (r.ok) {
    toast(level === "none" ? "Throttle released" : level === "hard" ? "Hard throttle — new spend blocked" : "Soft throttle — new spend warned",
      level === "none" ? "ok" : "warn");
    window.go("cost");
  } else toast("Throttle change failed: " + esc(r.data?.error || r.status), "bad");
};
