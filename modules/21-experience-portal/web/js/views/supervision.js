// Supervision: the manager's office — approvals inbox, escalations,
// interventions, risk (Module 09, enforced by Module 03 over Kafka).
import {SVC, get, post, uuid4} from "../api.js";
import {$, esc, badge, rel, toast, rowItem} from "../ui.js";

export async function viewSupervision() {
  const [qR, riskR] = await Promise.all([
    get(SVC.supervision + "/queue?page_size=25"),
    get(SVC.supervision + "/risk-dashboard"),
  ]);
  const items = (qR.data && qR.data.items) || [];
  const risk = riskR.data || {};

  const inbox = items.length === 0
    ? `<div class="empty">Inbox zero — no agent work waiting on you.</div>`
    : items.map(it => {
        const acts = it.item_type === "approval" && (it.status === "pending" || it.status === "in_progress")
          ? `<button class="ok sm" onclick="window.supDecide('${it.item_id}','approve')">Approve</button>
             <button class="bad sm" onclick="window.supDecide('${it.item_id}','reject')">Reject</button>` : "";
        const revoke = it.item_type === "intervention" && it.status === "active"
          ? `<button class="ghost sm" onclick="window.supRevoke('${it.item_id}')">Lift</button>` : "";
        return rowItem({
          title: `${it.item_type === "approval" ? "⏸" : it.item_type === "escalation" ? "🚨" : "🛑"} ${esc(it.title || it.item_type)}`,
          meta: `${esc(it.item_type)} · ${rel(it.created_at)}${it.assigned_to ? " · assigned " + esc(it.assigned_to.slice(0, 8)) : ""}`,
          badges: badge(it.priority || "medium") + badge(it.status),
          actions: acts + revoke,
        });
      }).join("");

  const sev = risk.escalation_by_severity || {};
  return `
    <div class="grid g4" style="margin-bottom:14px">
      <div class="card metric"><b>${risk.overall_risk_score ?? 0}</b><span>risk score (0–100)</span></div>
      <div class="card metric"><b>${risk.active_approvals_count ?? 0}</b><span>awaiting your decision</span></div>
      <div class="card metric"><b>${risk.pending_escalations_count ?? 0}</b><span>open escalations</span></div>
      <div class="card metric"><b>${risk.active_interventions_count ?? 0}</b><span>active interventions</span></div>
    </div>
    <div class="grid g2">
      <div class="card">
        <h3>Manager inbox <span class="tag">Module 09 · decisions enforced via Kafka</span></h3>
        <div class="hint">Approving or rejecting here drives the orchestrator — agents cannot proceed without you.</div>
        ${inbox}
      </div>
      <div class="card">
        <h3>Take control</h3>
        <div class="hint">Escalate an incident or intervene on an agent directly.</div>
        <label>Raise an escalation</label>
        <div class="frow"><input id="escTitle" placeholder="e.g. prompt injection on support agent">
          <select id="escSev" style="max-width:110px">
            <option>low</option><option>medium</option><option selected>high</option>
            <option>critical</option><option>p0</option></select>
          <select id="escCat" style="max-width:130px">
            <option>security</option><option>compliance</option><option>operational</option>
            <option>financial</option><option>hallucination</option><option>system</option></select>
          <button class="sm warn" onclick="window.supEscalate()">Raise</button></div>
        <label style="margin-top:14px">Intervene on an agent</label>
        <div class="frow"><input id="ivAgent" placeholder="agent id or name">
          <select id="ivAction" style="max-width:120px">
            <option>pause</option><option>stop</option><option>restrict</option><option>suspend</option></select>
          <button class="sm bad" onclick="window.supIntervene()">Apply</button></div>
        <div class="hint" style="margin-top:10px">By severity: ${Object.entries(sev).map(([k, v]) => `${badge(k)} ${v}`).join(" ") || "—"}</div>
      </div>
    </div>`;
}

window.supDecide = async function (id, action) {
  const body = action === "approve"
    ? {approver_id: uuid4(), comment: "Approved from the Operan portal"}
    : {rejector_id: uuid4(), reason: "Rejected from the Operan portal"};
  const r = await post(`${SVC.supervision}/approvals/${id}/${action}`, body);
  if (r.ok) toast(`Decision sent — the orchestrator will ${action === "approve" ? "resume" : "stop"} the workflow`, "ok");
  window.go("supervision");
};

window.supEscalate = async function () {
  const title = $("escTitle").value.trim();
  if (!title) return;
  const r = await post(SVC.supervision + "/escalations", {
    severity: $("escSev").value, category: $("escCat").value, title,
    source_agent_id: uuid4(),
  });
  if (r.ok) toast("Escalation raised" + ($("escCat").value === "security" || $("escCat").value === "compliance"
    ? " — policy violation event published" : ""), "ok");
  window.go("supervision");
};

window.supIntervene = async function () {
  const target = $("ivAgent").value.trim();
  if (!target) return;
  const r = await post(SVC.supervision + "/interventions", {
    action: $("ivAction").value, target_agent_id: target,
    reason: "Manager intervention from the Operan portal", duration_minutes: 60,
  });
  if (r.ok) toast(`Agent ${esc(target)} ${$("ivAction").value}d for 60 minutes`, "ok");
  window.go("supervision");
};

window.supRevoke = async function (id) {
  await post(`${SVC.supervision}/interventions/${id}/revoke`);
  toast("Intervention lifted", "ok");
  window.go("supervision");
};
