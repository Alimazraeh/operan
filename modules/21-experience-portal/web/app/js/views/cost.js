// Cost Governance (Module 17): budgets, cost events, alerts, summary, throttle.
import { SVC, get, post, patch, del, uuid4, unwrapList } from "../api.js";
import { $, esc, badge, rel, toast } from "../ui.js";

export async function viewCost() {
  let summaryR, budgetsR, eventsR, alertsR, throttleR;
  try {
    [summaryR, budgetsR, eventsR, alertsR, throttleR] = await Promise.all([
      get(SVC.cost + "/v1/summary"),
      get(SVC.cost + "/v1/budgets"),
      get(SVC.cost + "/v1/cost-events?page_size=50"),
      get(SVC.cost + "/v1/alerts"),
      get(SVC.cost + "/v1/throttle"),
    ]);
  } catch (e) { return viewError("Failed to load cost data", e.message); }

  const summary = summaryR.data || {};
  const budgets = unwrapList(budgetsR, "budgets");
  const events = unwrapList(eventsR, "events");
  const alerts = unwrapList(alertsR, "alerts");
  const throttle = throttleR.data || {};

  const totalSpent = summary.total_spent || summary.spend || 0;
  const totalBudget = summary.total_budget || budgets.reduce((sum, b) => sum + (b.amount || b.budget || 0), 0);
  const utilization = totalBudget > 0 ? Math.round((totalSpent / totalBudget) * 100) : 0;

  return `
    <div class="grid g4" style="margin-bottom:18px">
      <div class="card metric"><b>$${esc(String(totalSpent))}</b><span>total spent</span></div>
      <div class="card metric"><b>$${esc(String(totalBudget))}</b><span>total budget</span></div>
      <div class="card metric"><b>${esc(String(utilization))}%</b><span>budget used</span></div>
      <div class="card metric"><b>${alerts.length}</b><span>alerts</span></div>
    </div>

    <!-- Budget progress bar -->
    ${totalBudget > 0 ? `<div class="card" style="margin-bottom:18px">
      <h3>Budget utilization <span class="tag">Module 17</span></h3>
      <div style="background:var(--bg);border-radius:8px;height:24px;overflow:hidden;margin-top:8px">
        <div style="width:${Math.min(utilization, 100)}%;height:100%;background:${utilization > 90 ? "var(--bad)" : utilization > 70 ? "#f0ad4e" : "var(--ok)"};border-radius:8px;transition:width 0.5s"></div>
      </div>
      <div class="hint" style="margin-top:6px">$${esc(String(totalSpent))} of $${esc(String(totalBudget))} (${esc(String(utilization))}%)</div>
    </div>` : ""}

    <!-- Tab navigation -->
    <div style="display:flex;gap:8px;margin-bottom:18px;flex-wrap:wrap">
      <button class="sm cost-tab active" data-tab="budgets">Budgets</button>
      <button class="sm cost-tab" data-tab="events">Cost Events</button>
      <button class="sm cost-tab" data-tab="alerts">Alerts</button>
      <button class="sm cost-tab" data-tab="throttle">Throttle</button>
    </div>

    <!-- Budgets tab -->
    <div class="cost-panel" id="panel-budgets">
      <div class="card" style="margin-bottom:18px">
        <h3>Budget management <span class="tag">Module 17</span></h3>
        <div class="hint">Set per-department, per-agent, or global spending limits. The engine enforces hard and soft limits in real time.</div>
        <div class="frow" style="margin-bottom:14px">
          <input id="budgetName" placeholder="budget name (e.g. Marketing Q3)">
          <input id="budgetAmount" placeholder="amount" type="number" style="max-width:120px">
          <select id="budgetScope" style="max-width:120px">
            <option value="department">department</option>
            <option value="agent">agent</option>
            <option value="global">global</option>
          </select>
          <button class="sm" onclick="window.costCreateBudget()">Create budget</button>
        </div>
        ${budgets.length === 0
          ? `<div class="empty">No budgets defined.</div>`
          : budgets.map(b => {
              const spent = b.spent || b.spend || 0;
              const amount = b.amount || b.budget || 0;
              const pct = amount > 0 ? Math.round((spent / amount) * 100) : 0;
              return `<div class="card" style="margin-bottom:12px">
                <div class="frow">
                  <h3 style="flex:1">${esc(b.name || b.id.slice(0,8))}</h3>
                  <button class="bad sm" onclick="window.costDeleteBudget('${esc(b.id)}')">Delete</button>
                </div>
                <div class="hint">${esc(b.description || b.scope || "No description")} · scope: ${esc(b.scope || "department")}</div>
                <div style="background:var(--bg);border-radius:6px;height:16px;overflow:hidden;margin-top:6px">
                  <div style="width:${Math.min(pct,100)}%;height:100%;background:${pct > 90 ? "var(--bad)" : pct > 70 ? "#f0ad4e" : "var(--ok)"};border-radius:6px"></div>
                </div>
                <div class="hint">$${esc(String(spent))} / $${esc(String(amount))} (${esc(String(pct))}%)</div>
              </div>`;
            }).join("")}
      </div>
    </div>

    <!-- Events tab -->
    <div class="cost-panel" id="panel-events" style="display:none">
      <div class="card">
        <h3>Cost events <span class="tag">Module 17</span></h3>
        <div class="hint">Real-time token and tool usage events. Each event tracks provider, model, tokens consumed, and cost.</div>
        ${events.length === 0
          ? `<div class="empty">No cost events recorded.</div>`
          : `<div style="max-height:500px;overflow:auto">
              <table>
                <thead><tr><th>Time</th><th>Provider</th><th>Model</th><th>Tokens</th><th>Cost</th><th>Agent</th><th>Dept</th></tr></thead>
                <tbody>${events.map(e => `<tr>
                  <td class="mono">${esc(rel(e.timestamp || e.created_at || ""))}</td>
                  <td>${esc(e.provider || e.model_provider || "—")}</td>
                  <td class="mono">${esc(e.model || "—")}</td>
                  <td class="mono">${esc(String(e.tokens || e.input_tokens || 0))}</td>
                  <td>$${esc(String(e.cost || e.total_cost || 0))}</td>
                  <td class="mono">${esc((e.agent_id || e.agent || "").slice(0,8))}</td>
                  <td>${esc(e.department_id || e.dept || "—")}</td>
                </tr>`).join("")}</tbody>
              </table>
            </div>`}
      </div>
    </div>

    <!-- Alerts tab -->
    <div class="cost-panel" id="panel-alerts" style="display:none">
      <div class="card">
        <h3>Budget alerts <span class="tag">Module 17</span></h3>
        <div class="hint">Threshold-based alerts when spending approaches or exceeds budget limits.</div>
        ${alerts.length === 0
          ? `<div class="empty">No alerts triggered.</div>`
          : alerts.map(a => rowItem({
              title: `${a.severity === "critical" ? "🔴" : a.severity === "warning" ? "🟡" : "🔵"} ${esc(a.message || a.title || a.alert_type || "Alert")}`,
              meta: `budget: ${esc((a.budget_id || a.budget || "").slice(0,8))} · ${esc(rel(a.triggered_at || a.created_at || ""))}`,
              badges: badge(a.severity || "info"),
            })).join("")}
      </div>
    </div>

    <!-- Throttle tab -->
    <div class="cost-panel" id="panel-throttle" style="display:none">
      <div class="card">
        <h3>Spending throttle <span class="tag">Module 17</span></h3>
        <div class="hint">Emergency spending controls. When active, blocks all new agent executions.</div>
        <div class="grid g2">
          <div>
            <label>Throttle status</label>
            <p class="mono" style="margin-top:4px">${throttle.active ? "🔴 ACTIVE — all executions blocked" : "🟢 INACTIVE — executions allowed"}</p>
          </div>
          <div>
            <label>Max daily spend</label>
            <p class="mono" style="margin-top:4px">$${esc(String(throttle.max_daily_spend || throttle.daily_limit || "unlimited"))}</p>
          </div>
        </div>
        <div style="margin-top:12px">
          ${throttle.active
            ? `<button class="ok sm" onclick="window.costSetThrottle(false)">Disable throttle</button>`
            : `<button class="bad sm" onclick="window.costSetThrottle(true)">Emergency throttle</button>`}
        </div>
      </div>
    </div>`;
}

// ── Tab switching ──────────────────────────────────────────
document.querySelectorAll(".cost-tab").forEach(btn => {
  btn.addEventListener("click", () => {
    document.querySelectorAll(".cost-tab").forEach(b => b.classList.remove("active"));
    btn.classList.add("active");
    document.querySelectorAll(".cost-panel").forEach(p => p.style.display = "none");
    const panel = document.getElementById("panel-" + btn.dataset.tab);
    if (panel) panel.style.display = "block";
  });
});

// ── Budget CRUD ────────────────────────────────────────────
window.costCreateBudget = async function () {
  const name = $("budgetName").value.trim();
  const amount = $("budgetAmount").value;
  if (!name || !amount) { toast("Name and amount required", "bad"); return; }
  try {
    const r = await post(SVC.cost + "/v1/budgets", {
      id: uuid4(), name, amount: parseFloat(amount),
      scope: $("budgetScope").value,
      description: `Budget: ${name}`,
    });
    if (r.ok) { toast("Budget " + esc(name) + " created", "ok"); window.go("cost"); }
    else toast("Failed: " + esc(JSON.stringify(r.data).slice(0, 120)), "bad");
  } catch (e) { toast("Error: " + esc(String(e)), "bad"); }
};

window.costDeleteBudget = async function (id) {
  if (!confirm("Delete this budget?")) return;
  const r = await del(SVC.cost + "/v1/budgets/" + encodeURIComponent(id));
  if (r.ok) { toast("Budget deleted", "ok"); window.go("cost"); }
  else toast("Failed: " + esc(JSON.stringify(r.data).slice(0, 100)), "bad");
};

// ── Throttle ───────────────────────────────────────────────
window.costSetThrottle = async function (active) {
  try {
    const status = active ? "active" : "inactive";
    const r = await patch(SVC.cost + "/v1/throttle/" + status);
    if (r.ok) { toast("Throttle set to " + status, "ok"); window.go("cost"); }
    else toast("Failed: " + esc(JSON.stringify(r.data).slice(0, 100)), "bad");
  } catch (e) { toast("Error: " + esc(String(e)), "bad"); }
};

function viewError(title, msg) {
  return `<div class="error-box"><div class="err-title">${esc(title)}</div><div class="err-msg">${esc(msg)}</div><button onclick="window.go('cost')">Retry</button></div>`;
}