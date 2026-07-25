// Supervision: the command post above the routine gates (Module 09).
// Approvals are the normal human-in-the-loop; escalations flag that something
// is wrong (security, hallucination, compliance…); interventions are the
// emergency brake on a specific agent. Both bind to the REAL workforce —
// agents come from the registry, never free-typed ids.
import { SVC, get, post, patch, session, listAgents, unwrapList } from "../api.js";
import { esc, badge, rel, toast, rowItem } from "../ui.js";

export async function viewSupervision() {
  let qR, riskR, agentsR;
  try {
    [qR, riskR, agentsR] = await Promise.all([
      get(SVC.supervision + "/queue?page_size=25"),
      get(SVC.supervision + "/risk-dashboard"),
      listAgents(1, 100),
    ]);
  } catch (e) { return viewError("Failed to load supervision", e.message); }

  const items = (qR.data && qR.data.items) || [];
  const risk = riskR.data || {};
  const agents = unwrapList(agentsR);
  const agentName = id => agents.find(a => a.id === id)?.name || id;
  // Value/label pairs for the escalate/intervene selects.
  const agentOptions = agents
    .map(a => `<option value="${esc(a.id)}">${esc(a.name)} — ${esc(a.role || "agent")}</option>`).join("");

  const inbox = items.length === 0
    ? `<div class="empty">Inbox zero — no agent work waiting on you.</div>`
    : items.map(it => {
        const acts = it.item_type === "approval" && (it.status==="pending" || it.status==="in_progress")
          ? `<button class="ok sm" onclick="window.supDecide('${it.item_id}','approve')">Approve</button><button class="bad sm" onclick="window.supDecide('${it.item_id}','reject')">Reject</button>` : "";
        const revoke = it.item_type === "intervention" && it.status === "active"
          ? `<button class="ghost sm" onclick="window.supRevoke('${it.item_id}')">Lift</button>` : "";
        const escActs = it.item_type === "escalation" && ["open", "acknowledged", "in_progress"].includes(it.status)
          ? `${it.status === "open" ? `<button class="ghost sm" onclick="window.supAckEscalation('${it.item_id}')">Acknowledge</button>` : ""}<button class="ok sm" onclick="window.supResolveEscalation('${it.item_id}')">Resolve</button>` : "";
        // Intervention titles arrive as "<action> <agent uuid>" — show the agent's name.
        let title = it.title || it.item_type;
        if (it.item_type === "intervention") {
          const parts = title.split(" ");
          title = parts.length > 1 ? parts.slice(0, -1).join(" ") + " " + agentName(parts[parts.length - 1]) : title;
        }
        return rowItem({
          title: `${it.item_type==="approval"?"⏸":it.item_type==="escalation"?"🚨":"🛑"} ${esc(title)}`,
          meta: `${esc(it.item_type)} · ${rel(it.created_at)}${it.assigned_to?" · assigned "+esc(it.assigned_to.slice(0,8)):""}`,
          badges: badge(it.priority||"medium") + badge(it.status),
          actions: acts + revoke + escActs,
        });
      }).join("");

  const sev = risk.escalation_by_severity || {};
  return `
    <div class="grid g4" style="margin-bottom:18px">
      <div class="card metric"><b>${risk.overall_risk_score ?? 0}</b><span>risk score (0–100)</span></div>
      <div class="card metric"><b>${risk.active_approvals_count ?? 0}</b><span>awaiting decision</span></div>
      <div class="card metric"><b>${risk.pending_escalations_count ?? 0}</b><span>open escalations</span></div>
      <div class="card metric"><b>${risk.active_interventions_count ?? 0}</b><span>active interventions</span></div>
    </div>
    <div class="grid g2">
      <div class="card">
        <h3>Manager inbox <span class="tag">Module 09</span></h3>
        <div class="hint">Approving or rejecting here drives the orchestrator — agents cannot proceed without you.</div>
        ${inbox}
      </div>
      <div class="card">
        <h3>Take control</h3>
        <div class="hint">Escalate an incident on an agent, or pull the brake directly. Both act on
        real registry agents.</div>
        <label>Raise an escalation</label>
        <div class="frow"><input id="escTitle" placeholder="what happened? e.g. prompt injection in triage output" style="flex:2">
          <select id="escAgent" style="max-width:220px"><option value="">— which agent —</option>${agentOptions}</select>
          <select id="escSev" style="max-width:110px"><option>low</option><option selected>medium</option><option>high</option><option>critical</option><option>p0</option></select>
          <select id="escCat" style="max-width:130px"><option>security</option><option>compliance</option><option>operational</option><option>financial</option><option>hallucination</option><option>system</option></select>
          <button class="sm warn" onclick="window.supEscalate()">Raise</button></div>
        <label style="margin-top:14px">Intervene on an agent</label>
        <div class="frow"><select id="ivAgent" style="max-width:220px"><option value="">— which agent —</option>${agentOptions}</select>
          <select id="ivAction" style="max-width:120px"><option>pause</option><option>stop</option><option>restrict</option><option>override</option><option>redirect</option><option>suspend</option></select>
          <input id="ivDur" type="number" value="60" min="1" max="10080" style="max-width:90px" title="duration in minutes">
          <span class="hint" style="margin:0">min</span>
          <button class="sm bad" onclick="window.supIntervene()">Apply</button></div>
        <div class="hint" style="margin-top:10px">By severity: ${Object.entries(sev).map(([k,v])=>badge(k)+" "+v).join(" ") || "—"}</div>
      </div>
    </div>`;
}

window.supDecide = async function (id, action) {
  // Attribution comes from the token server-side — see M09 actorFromToken.
  if (!session.userId) { toast("Sign in again — a decision must be attributable to you", "bad"); return; }
  const body = action === "approve" ? { comment: "Approved from the Operan portal" } : { reason: "Rejected from the Operan portal" };
  const r = await post(`${SVC.supervision}/approvals/${id}/${action}`, body);
  if (r.ok) toast(`Decision sent — the orchestrator will ${action==="approve"?"resume":"stop"} the workflow`, "ok");
  else toast("Decision failed: " + esc(r.data?.error?.message || r.status), "bad");
  window.go("supervision");
};

window.supEscalate = async function () {
  const title = $("escTitle").value.trim();
  const agentId = $("escAgent").value;
  if (!title) { toast("Describe what happened", "warn"); return; }
  if (!agentId) { toast("Pick the agent involved", "warn"); return; }
  const r = await post(SVC.supervision + "/escalations", {
    severity: $("escSev").value, category: $("escCat").value, title, source_agent_id: agentId,
  });
  if (r.ok) toast("Escalation raised" + ($("escCat").value==="security"||$("escCat").value==="compliance"?" — policy violation event published":""), "ok");
  else toast("Escalation failed: " + esc(r.data?.error?.message || r.data?.detail || r.status), "bad");
  window.go("supervision");
};

window.supIntervene = async function () {
  const target = $("ivAgent").value;
  if (!target) { toast("Pick the agent to intervene on", "warn"); return; }
  const minutes = Math.min(10080, Math.max(1, parseInt($("ivDur").value, 10) || 60));
  const action = $("ivAction").value;
  const r = await post(SVC.supervision + "/interventions", {
    action, target_agent_id: target,
    reason: "Manager intervention from the Operan portal", duration_minutes: minutes,
  });
  const name = $("ivAgent").selectedOptions[0]?.textContent.split(" — ")[0] || "agent";
  if (r.ok) toast(`${esc(name)}: ${esc(action)} for ${minutes} minutes`, "ok");
  else toast("Intervention failed: " + esc(r.data?.error?.message || r.data?.detail || r.status), "bad");
  window.go("supervision");
};

window.supRevoke = async function (id) {
  const r = await post(`${SVC.supervision}/interventions/${id}/revoke`);
  if (r.ok) toast("Intervention lifted", "ok");
  else toast("Lift failed: " + esc(r.data?.error?.message || r.status), "bad");
  window.go("supervision");
};

window.supAckEscalation = async function (id) {
  const r = await patch(`${SVC.supervision}/escalations/${id}`, { status: "acknowledged" });
  if (r.ok) toast("Escalation acknowledged — you own it now", "ok");
  else toast("Acknowledge failed: " + esc(r.data?.error?.message || r.status), "bad");
  window.go("supervision");
};

window.supResolveEscalation = async function (id) {
  if (!session.userId) { toast("Sign in again — a resolution must be attributable to you", "bad"); return; }
  const r = await post(`${SVC.supervision}/escalations/${id}/resolve`, {
    resolution_notes: "Resolved from the Operan portal",
  });
  if (r.ok) toast("Escalation resolved", "ok");
  else toast("Resolve failed: " + esc(r.data?.error?.message || r.status), "bad");
  window.go("supervision");
};

function viewError(title, msg) {
  return `<div class="error-box"><div class="err-title">${esc(title)}</div><div class="err-msg">${esc(msg)}</div><button onclick="window.go('supervision')">Retry</button></div>`;
}