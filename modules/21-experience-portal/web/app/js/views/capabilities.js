// Capabilities — the doing layer (Module 08).
//
// SOPs name business verbs; bindings decide which system performs each verb
// for this tenant; every execution goes through one governed funnel — binding,
// input contract, policy, authority — and lands in an immutable audit trail.
// This console shows all four layers, and it never hides the word SIMULATED:
// a simulated action is real evidence of the process working, and a lie if
// dressed as a live one.
import { SVC, get } from "../api.js";
import { esc, rel, toast, statCard, badge } from "../ui.js";

const STATUS_LABEL = {
  completed: "completed",
  blocked_no_binding: "blocked — no binding",
  invalid_input: "invalid input",
  denied_policy: "denied by policy",
  denied_authority: "denied — authority",
  failed: "failed",
};

export default async function viewCapabilities() {
  const [cR, pR, bR, iR] = await Promise.allSettled([
    get(SVC.tools + "/capabilities"),
    get(SVC.tools + "/providers"),
    get(SVC.tools + "/bindings"),
    get(SVC.tools + "/invocations?limit=40"),
  ]);
  const caps = ok(cR) ? (cR.value.data.capabilities || []) : [];
  const providers = ok(pR) ? (pR.value.data.providers || []) : [];
  const bindings = ok(bR) ? (bR.value.data.bindings || []) : [];
  const invocations = ok(iR) ? (iR.value.data.invocations || []) : [];
  const unavailable = [cR, pR, bR, iR].some(r => !ok(r));

  const byProvider = {};
  for (const p of providers) byProvider[p.id] = p;
  const simulatedBindings = bindings.filter(b => b.simulated).length;
  const refusals = invocations.filter(i => i.status !== "completed").length;

  return `<div id="capRoot">
    ${unavailable ? `<div class="card" style="margin-bottom:18px;border-left:3px solid var(--warn,#c90)">
      <b>The capability service could not be fully reached.</b>
      <div class="hint">Sections that loaded are shown; the rest are absent, not empty.</div></div>` : ""}

    <div class="stats-grid">
      ${statCard("🗣", "Capabilities", caps.length, "Business verbs SOPs can bind to")}
      ${statCard("🔌", "Providers", providers.length, providers.length ? "Systems that perform verbs" : "None registered")}
      ${statCard("🔗", "Bindings", bindings.length, simulatedBindings ? simulatedBindings + " simulated" : "Which system performs each verb")}
      ${statCard("🧾", "Recent invocations", invocations.length, refusals ? refusals + " refused — refusals are records too" : "The audit trail")}
    </div>

    <div class="card" style="margin-bottom:18px">
      <h3>Vocabulary <span class="tag">Module 08</span></h3>
      <div class="hint">The stable set of business verbs. SOPs bind to these, never to vendors —
      which system performs a verb is a per-tenant binding, so the same SOP runs against
      different stacks without changing. Verbs are added on evidence from the SOP catalogue;
      each one is a permanent contract.</div>
      ${caps.map(c => `
        <div class="row-item">
          <div class="grow">
            <div class="t">🗣 <code>${esc(c.id)}</code> — ${esc(c.name)}
              ${sideEffectChip(c.side_effect)}
              <span class="wfstep agent" style="font-size:10px;padding:1px 7px">needs ≥ ${esc(c.min_autonomy)}</span></div>
            <div class="m">${esc(c.description || "")}</div>
          </div>
        </div>`).join("")}
    </div>

    <div class="grid g2" style="margin-bottom:18px">
      <div class="card">
        <h3>Providers</h3>
        <div class="hint">Only the simulated provider executes today. It answers realistically,
        touches nothing, and is flagged on every record it produces — swapping it for a live
        system is a binding change, not a code change.</div>
        ${providers.length === 0 ? `<div class="empty">No providers registered for this tenant.</div>` :
          providers.map(p => `
          <div class="row-item">
            <div class="grow">
              <div class="t">🔌 ${esc(p.name)} ${p.kind === "simulated" ? simTag() : `<span class="tag">${esc(p.kind)}</span>`}</div>
              <div class="m">${esc(p.endpoint || "no endpoint — in-process")} · ${p.credential_ref ? "credential: " + esc(p.credential_ref) : "no credential needed"}</div>
            </div>
            <div class="actions">${badge(p.status || "active")}</div>
          </div>`).join("")}
      </div>
      <div class="card">
        <h3>Bindings</h3>
        <div class="hint">The customer-specific join: which provider performs each verb.
        A department's own binding overrides the tenant default. An unbound verb blocks
        honestly — SOPs never pass through it silently.</div>
        ${bindings.length === 0 ? `<div class="empty">No bindings. Every capability-bearing SOP step will block until bound.</div>` :
          bindings.map(b => {
            const prov = byProvider[b.provider_id] || {};
            return `
          <div class="row-item">
            <div class="grow">
              <div class="t"><code>${esc(b.capability_id)}</code> → ${esc(prov.name || b.provider_id)} ${b.simulated ? simTag() : ""}</div>
              <div class="m">${b.department_id ? "department override: " + esc(b.department_id.slice(0, 8)) + "…" : "tenant default"} · tool ${esc(b.provider_tool)}</div>
            </div>
            <div class="actions">${badge(b.enabled ? "enabled" : "disabled")}</div>
          </div>`;
          }).join("")}
      </div>
    </div>

    <div class="card">
      <h3>Invocation trail <span class="tag">immutable</span></h3>
      <div class="hint">Every attempt, refusals included: a denied action is as much a fact as a
      completed one. Each record carries the actor, the seat whose authority it ran under, the
      policy decision, and where the result lives.</div>
      ${invocations.length === 0 ? `<div class="empty">No invocations yet. Raise a request whose SOP carries a capability step.</div>` :
        invocations.map(i => `
        <div class="row-item">
          <div class="grow">
            <div class="t">${i.status === "completed" ? "⚙️" : "⛔"} <code>${esc(i.capability_id)}</code>
              ${i.simulated ? simTag() : ""}
              ${i.external_ref ? `<span class="tag">${esc(i.external_ref.system)}/${esc(i.external_ref.kind)} ${esc(i.external_ref.id)}</span>` : ""}</div>
            <div class="m">${esc(actorLine(i))} · ${esc(i.policy_decision || "no policy decision recorded")}
              ${i.error ? " · " + esc(i.error) : ""} · ${rel(i.created_at)}</div>
          </div>
          <div class="actions"><span class="badge ${i.status === "completed" ? "active" : "failed"}">${esc(STATUS_LABEL[i.status] || i.status)}</span></div>
        </div>`).join("")}
    </div>
  </div>`;
}

function ok(r) { return r.status === "fulfilled" && r.value && r.value.ok && r.value.data; }

function simTag() {
  return `<span class="wfstep human_gate" style="font-size:10px;padding:1px 7px;letter-spacing:.05em">SIMULATED</span>`;
}

function sideEffectChip(se) {
  const tone = se === "destructive" ? "background:#a33;color:#fff" : se === "write" ? "background:#a80;color:#fff" : "";
  return `<span class="tag" style="${tone}">${esc(se || "read")}</span>`;
}

function actorLine(i) {
  const a = i.actor || {};
  const who = a.id ? `${a.type || "actor"} ${String(a.id).slice(0, 12)}` : "unattributed";
  const tier = a.autonomy_tier ? ` under ${a.autonomy_tier} authority` : " under no established authority";
  const req = i.correlation && i.correlation.request_id ? ` · request ${String(i.correlation.request_id).slice(0, 8)}…` : "";
  return who + tier + req;
}
