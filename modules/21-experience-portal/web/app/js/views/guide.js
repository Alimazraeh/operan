// The Guide — how Operan works, drawn rather than described.
//
// Every chapter leads with a diagram in the product's own design language,
// because the operating model IS a structure: a loop, a graph, a funnel. The
// diagrams are hand-authored SVG on the app's CSS variables, so they theme
// with the portal and cost nothing to load. Visible to every role — the guide
// explains authority, it does not require any.
import { esc } from "../ui.js";
import { session } from "../api.js";

// ─── SVG building blocks ────────────────────────────────────────────────────
// One visual language for every diagram: rounded boxes, labeled arrows,
// tinted by meaning (accent=platform, gate=human, ok=done, purple=agents,
// bad=refusals).

function svgOpen(w, h) {
  return `<svg viewBox="0 0 ${w} ${h}" xmlns="http://www.w3.org/2000/svg"
    style="width:100%;height:auto;display:block;font-family:inherit"
    role="img">`;
}

const DEFS = `<defs>
  <marker id="arr" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
    <path d="M0,0 L10,5 L0,10 z" fill="var(--text-dim)"/>
  </marker>
  <marker id="arrA" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
    <path d="M0,0 L10,5 L0,10 z" fill="var(--accent)"/>
  </marker>
  <marker id="arrB" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
    <path d="M0,0 L10,5 L0,10 z" fill="var(--bad)"/>
  </marker>
</defs>`;

function box(x, y, w, h, title, sub, stroke = "var(--border-hover)", fill = "var(--surface)") {
  const t = `<text x="${x + w / 2}" y="${y + (sub ? h / 2 - 6 : h / 2 + 1)}" text-anchor="middle" dominant-baseline="middle"
      fill="var(--text)" font-size="13" font-weight="600">${esc(title)}</text>`;
  const s = sub ? `<text x="${x + w / 2}" y="${y + h / 2 + 13}" text-anchor="middle" dominant-baseline="middle"
      fill="var(--text-dim)" font-size="10.5">${esc(sub)}</text>` : "";
  return `<rect x="${x}" y="${y}" width="${w}" height="${h}" rx="10"
      fill="${fill}" stroke="${stroke}" stroke-width="1.4"/>${t}${s}`;
}

function chip(x, y, w, text, color = "var(--accent)") {
  return `<rect x="${x}" y="${y}" width="${w}" height="20" rx="10" fill="none" stroke="${color}" stroke-width="1"/>
    <text x="${x + w / 2}" y="${y + 11}" text-anchor="middle" dominant-baseline="middle" fill="${color}" font-size="10" font-weight="600">${esc(text)}</text>`;
}

function arrow(x1, y1, x2, y2, label, color = "var(--text-dim)", marker = "arr") {
  const mx = (x1 + x2) / 2, my = (y1 + y2) / 2;
  const l = label ? `<text x="${mx}" y="${my - 7}" text-anchor="middle" fill="var(--text-dim)" font-size="10">${esc(label)}</text>` : "";
  return `<line x1="${x1}" y1="${y1}" x2="${x2}" y2="${y2}" stroke="${color}" stroke-width="1.4" marker-end="url(#${marker})"/>${l}`;
}

function elbow(x1, y1, x2, y2, label, color = "var(--text-dim)", marker = "arr") {
  const l = label ? `<text x="${x1 + 8}" y="${(y1 + y2) / 2}" fill="var(--text-dim)" font-size="10">${esc(label)}</text>` : "";
  return `<path d="M ${x1} ${y1} L ${x1} ${y2} L ${x2} ${y2}" fill="none" stroke="${color}" stroke-width="1.4" marker-end="url(#${marker})"/>${l}`;
}

function emoji(x, y, e, size = 20) {
  return `<text x="${x}" y="${y}" text-anchor="middle" dominant-baseline="middle" font-size="${size}">${e}</text>`;
}

// ─── Diagrams ───────────────────────────────────────────────────────────────

function diagramWorkLoop() {
  let s = svgOpen(960, 300) + DEFS;
  // The loop: request → SOP run → agent → gate → action → done, ledger under.
  s += emoji(70, 52, "🧑", 26) + `<text x="70" y="82" text-anchor="middle" fill="var(--text)" font-size="12" font-weight="600">A named person</text>
       <text x="70" y="96" text-anchor="middle" fill="var(--text-dim)" font-size="10">signed in, on the record</text>`;
  s += arrow(115, 60, 165, 60, "raises");
  s += box(168, 30, 150, 62, "📥 Service request", "SLA clocks start", "var(--accent)");
  s += arrow(318, 60, 368, 60, "compiles");
  s += box(371, 30, 150, 62, "📋 SOP → run", "the procedure, per request", "var(--accent)");
  s += arrow(521, 60, 571, 60);
  s += box(574, 30, 120, 62, "🤖 Agent", "drafts real work", "var(--purple)");
  s += arrow(694, 60, 744, 60);
  s += box(747, 30, 130, 62, "🚧 Human gate", "routed to a seat", "var(--gate)");
  // gate down to action
  s += elbow(812, 92, 700, 150, "approved");
  s += box(574, 122, 130, 56, "⚙️ Action", "via a capability", "var(--ok)");
  s += chip(586, 186, 106, "SIMULATED until live", "var(--gate)");
  s += arrow(574, 150, 524, 150);
  s += box(371, 122, 150, 56, "🏁 Done", "outcome + work product", "var(--ok)");
  // ledger
  s += `<rect x="60" y="228" width="820" height="52" rx="10" fill="var(--info-bg)" stroke="var(--border)" stroke-width="1"/>`;
  s += `<text x="470" y="249" text-anchor="middle" fill="var(--text)" font-size="12" font-weight="600">The request ledger — every step above lands here as a timeline fact</text>`;
  s += `<text x="470" y="266" text-anchor="middle" fill="var(--text-dim)" font-size="10.5">who asked · what the agent wrote · who approved (from their token, never a claim) · what was executed, where, under whose authority</text>`;
  for (const x of [243, 446, 634, 812]) s += `<line x1="${x}" y1="94" x2="${x}" y2="226" stroke="var(--border)" stroke-width="1" stroke-dasharray="3 4"/>`;
  return s + "</svg>";
}

function diagramAuthority() {
  let s = svgOpen(960, 330) + DEFS;
  // Left: the chain. Right: what it grants + the unbound state.
  s += box(60, 30, 220, 64, "🧑 Person", "dana@your-company — can sign in", "var(--accent)");
  s += arrow(170, 94, 170, 128, "holds");
  s += box(60, 131, 220, 84, "🪑 Seat: IT Manager", "autonomy: coordinate", "var(--purple)");
  s += chip(76, 186, 84, "decides ×3", "var(--purple)") + chip(168, 186, 96, "holds a gate", "var(--gate)");
  s += arrow(170, 215, 170, 249, "sits in");
  s += box(60, 252, 220, 56, "🏢 Department", "IT — org chart, services, SOPs", "var(--accent)");
  // derived authority
  s += arrow(280, 170, 384, 170, "derives", "var(--accent)", "arrA");
  s += box(388, 58, 250, 44, "Approve this department's gates", "they arrive in your inbox by name", "var(--ok)");
  s += box(388, 112, 250, 44, "See and steer its queue & KPIs", "", "var(--ok)");
  s += box(388, 166, 250, 44, "Change its org chart", "department.write from the root seat", "var(--ok)");
  s += box(388, 220, 250, 44, "Perform verbs up to your tier", "the capability funnel checks the seat", "var(--ok)");
  for (const y of [80, 134, 188, 242]) s += arrow(340, 170, 384, y + 2, "", "var(--border-hover)");
  // the honest unbound state
  s += `<rect x="680" y="58" width="220" height="206" rx="10" fill="var(--surface)" stroke="var(--border)" stroke-dasharray="4 4"/>`;
  s += `<text x="790" y="88" text-anchor="middle" fill="var(--text)" font-size="12" font-weight="600">◌ No seat?</text>`;
  s += `<text x="790" y="112" text-anchor="middle" fill="var(--text-dim)" font-size="10.5">You can sign in and raise</text>`;
  s += `<text x="790" y="126" text-anchor="middle" fill="var(--text-dim)" font-size="10.5">requests — and nothing else.</text>`;
  s += `<text x="790" y="150" text-anchor="middle" fill="var(--text-dim)" font-size="10.5">That is the honest state of a</text>`;
  s += `<text x="790" y="164" text-anchor="middle" fill="var(--text-dim)" font-size="10.5">new joiner, not a fault.</text>`;
  s += `<text x="790" y="192" text-anchor="middle" fill="var(--text-dim)" font-size="10.5">Authority arrives when an</text>`;
  s += `<text x="790" y="206" text-anchor="middle" fill="var(--text-dim)" font-size="10.5">admin assigns you a seat in</text>`;
  s += `<text x="790" y="220" text-anchor="middle" fill="var(--text-dim)" font-size="10.5">People — never from the</text>`;
  s += `<text x="790" y="234" text-anchor="middle" fill="var(--text-dim)" font-size="10.5">account itself.</text>`;
  s += `<text x="480" y="316" text-anchor="middle" fill="var(--text-muted)" font-size="10.5">Roles (platform admin, department head, supervisor, employee) grant a class of permission — the seats decide where it applies.</text>`;
  return s + "</svg>";
}

function diagramGateRouting() {
  let s = svgOpen(960, 290) + DEFS;
  s += box(60, 36, 240, 70, "📋 SOP step", 'approval · required_by: it-manager-01', "var(--gate)");
  s += arrow(300, 71, 356, 71, "resolves via");
  s += box(360, 36, 240, 70, "🪑 Org chart seat", "the position carrying that role", "var(--purple)");
  s += arrow(600, 71, 656, 71, "held by");
  s += box(660, 36, 240, 70, "🧑 A named person", "their personal approval inbox", "var(--accent)");
  // fallbacks
  s += elbow(180, 106, 356, 168, "step names nobody?");
  s += box(360, 142, 240, 52, "Department head's seat", "whoever runs the department", "var(--purple)");
  s += elbow(480, 194, 656, 246, "nobody holds the seat?");
  s += box(660, 220, 240, 52, "Role queue — honestly unowned", "never a fabricated approver", "var(--warn)");
  s += `<text x="60" y="240" fill="var(--text)" font-size="12" font-weight="600">What your approval records:</text>`;
  s += `<text x="60" y="258" fill="var(--text-dim)" font-size="10.5">your identity from your token — a request body claiming someone else is kept but ignored.</text>`;
  s += `<text x="60" y="272" fill="var(--text-dim)" font-size="10.5">A truncated agent draft arrives marked ⚠ INCOMPLETE, so you never sign an unfinished assessment unknowingly.</text>`;
  return s + "</svg>";
}

function diagramDeploy() {
  let s = svgOpen(960, 250) + DEFS;
  const stages = [
    ["📚 Template", "the blueprint: org chart, SOPs, services, KPIs"],
    ["✅ Validate", "structure + registry pre-flight"],
    ["🔌 Connect", "integrations recorded"],
    ["🧠 Memory", "charter + services embedded"],
    ["🤖 Agents", "registered with stable identity"],
    ["🟢 Operational", "the department is live"],
  ];
  let x = 40;
  stages.forEach(([t, sub], i) => {
    s += box(x, 40, 140, 66, t, "", i === stages.length - 1 ? "var(--ok)" : "var(--accent)");
    s += `<text x="${x + 70}" y="122" text-anchor="middle" fill="var(--text-dim)" font-size="9.5">${esc(sub)}</text>`;
    if (i < stages.length - 1) s += arrow(x + 140, 73, x + 152, 73);
    x += 152;
  });
  s += `<text x="480" y="170" text-anchor="middle" fill="var(--text)" font-size="12" font-weight="600">Deploying the same template twice produces the same agent identities</text>`;
  s += `<text x="480" y="188" text-anchor="middle" fill="var(--text-dim)" font-size="10.5">agent ids derive from department + definition, so a redeploy restores agents instead of minting orphans</text>`;
  s += `<text x="480" y="214" text-anchor="middle" fill="var(--text-dim)" font-size="10.5">and when a template's SOPs improve, a live department adopts them with one sync — no redeploy, no severed history</text>`;
  return s + "</svg>";
}

function diagramCapability() {
  let s = svgOpen(960, 430) + DEFS;
  // L1..L3 stack on the left
  s += box(40, 30, 260, 62, "L1 · Vocabulary", "business verbs: itsm.ticket.assign…", "var(--accent)");
  s += box(40, 104, 260, 62, "L2 · Providers", "simulated today; MCP / native / HTTP next", "var(--accent)");
  s += box(40, 178, 260, 62, "L3 · Bindings", "which provider performs each verb, per tenant", "var(--accent)");
  s += `<text x="170" y="266" text-anchor="middle" fill="var(--text-dim)" font-size="10.5">The SOP names the verb. The binding names</text>`;
  s += `<text x="170" y="280" text-anchor="middle" fill="var(--text-dim)" font-size="10.5">the system. Same SOP, any stack — swapping</text>`;
  s += `<text x="170" y="294" text-anchor="middle" fill="var(--text-dim)" font-size="10.5">simulated for live is a binding change.</text>`;
  // The funnel: L4
  s += `<text x="640" y="24" text-anchor="middle" fill="var(--text)" font-size="13" font-weight="700">L4 · The governed funnel — one door, no exceptions</text>`;
  const stages = [
    ["Resolve binding", "department override → tenant default", "blocked_no_binding"],
    ["Validate input", "against the verb's JSON-Schema contract", "invalid_input"],
    ["Policy check", "Module 10 · deny closed, default deny", "denied_policy"],
    ["Authority check", "the seat's tier ≥ the verb's minimum", "denied_authority"],
    ["Dispatch", "the bound provider performs it", "failed"],
  ];
  let y = 40;
  stages.forEach(([t, sub, refusal], i) => {
    s += box(400, y, 300, 52, t, sub, "var(--accent)");
    s += arrow(720, y + 26, 790, y + 26, "", "var(--bad)", "arrB");
    s += `<text x="796" y="${y + 30}" fill="var(--bad)" font-size="10.5" font-family="monospace">${esc(refusal)}</text>`;
    if (i < stages.length - 1) s += arrow(550, y + 52, 550, y + 64);
    y += 64;
  });
  s += arrow(550, y - 12, 550, y + 4, "", "var(--ok)");
  s += box(400, y + 6, 300, 56, "🧾 Immutable invocation record", "actor · seat · policy decision · external ref · SIMULATED flag", "var(--ok)");
  s += `<text x="550" y="${y + 84}" text-anchor="middle" fill="var(--text-dim)" font-size="10.5">Refusals land here too — a denied action is as much a fact as a completed one.</text>`;
  return s + "</svg>";
}

function diagramMeasurement() {
  let s = svgOpen(960, 230) + DEFS;
  s += box(60, 40, 220, 70, "🧾 Request ledger", "every request, timed and attributed", "var(--accent)");
  s += arrow(280, 75, 340, 75, "computed, never typed in");
  s += box(344, 40, 250, 70, "📊 Measured KPIs", "SLA %, cycle time, gate turnaround, spend", "var(--ok)");
  s += arrow(594, 75, 654, 75, "each morning");
  s += box(658, 40, 240, 70, "📰 Department briefing", "drafted by the head agent, on real figures", "var(--purple)");
  s += `<rect x="60" y="140" width="838" height="62" rx="10" fill="var(--warn-bg)" stroke="var(--border)"/>`;
  s += `<text x="479" y="164" text-anchor="middle" fill="var(--text)" font-size="12" font-weight="600">The honesty rule: a number is measured, or it says "not yet measured" — never invented</text>`;
  s += `<text x="479" y="184" text-anchor="middle" fill="var(--text-dim)" font-size="10.5">business KPIs with no data source show exactly that, and simulated actions are labelled SIMULATED wherever they appear</text>`;
  return s + "</svg>";
}

function diagramModules() {
  let s = svgOpen(960, 300) + DEFS;
  const groups = [
    ["Your window", "var(--accent)", [["21", "Portal — every screen in this app"]]],
    ["The work", "var(--purple)", [["05", "Departments & the work loop"], ["03", "Agent runs (SOP → DAG)"], ["09", "Human gates & escalations"], ["08", "Capabilities — the doing layer"]]],
    ["The guardrails", "var(--gate)", [["02", "Identity & access"], ["10", "Policy engine (default deny)"], ["17", "Cost & budgets"], ["11", "Observability"]]],
    ["The substrate", "var(--ok)", [["01", "Tenants"], ["04", "Agent registry (durable)"], ["07", "Memory & retrieval"], ["06", "Knowledge ingestion"], ["18", "Connectors"], ["12–16, 19", "Models, collab, sandbox, Arabic core"]]],
  ];
  let x = 40;
  for (const [title, color, mods] of groups) {
    s += `<text x="${x + 105}" y="30" text-anchor="middle" fill="${color}" font-size="12.5" font-weight="700">${esc(title)}</text>`;
    let y = 44;
    for (const [num, name] of mods) {
      s += `<rect x="${x}" y="${y}" width="210" height="34" rx="8" fill="var(--surface)" stroke="${color}" stroke-width="1"/>`;
      s += `<text x="${x + 12}" y="${y + 18}" fill="${color}" font-size="10.5" font-family="monospace" font-weight="700">M${esc(num)}</text>`;
      s += `<text x="${x + 52}" y="${y + 18}" fill="var(--text-dim)" font-size="9.8">${esc(name)}</text>`;
      y += 40;
    }
    x += 232;
  }
  return s + "</svg>";
}

// ─── Chapters ───────────────────────────────────────────────────────────────

const CHAPTERS = [
  {
    id: "loop", icon: "🔁", title: "How work gets done",
    body: () => `
      <p>Operan runs a department the way a well-run department already works — it just staffs the
      procedure with agents and keeps every step on the record. A named person raises a request.
      The service's SOP compiles into a run. Agents draft the real work. Where the SOP says a human
      decides, a gate routes to the person who holds that seat. Where the SOP says <i>do</i> something,
      a capability performs it under governance. The outcome, and every step behind it, lives on the
      request's timeline.</p>
      ${diagramWorkLoop()}
      <p class="hint">Nothing in this loop is decorative: if a step failed, the timeline says so; if a
      draft was cut short, it is marked incomplete; if an action was simulated, it says SIMULATED.</p>`,
  },
  {
    id: "authority", icon: "🪑", title: "Who may do what",
    body: () => `
      <p>Authority is not a settings page — it is the org chart. Your account lets you sign in; the
      <b>seat</b> you hold decides what you may approve, see and change, and in which department.
      That is one chain: <b>person → seat → department → authority</b>.</p>
      ${diagramAuthority()}
      <p class="hint">This is why an approval in Operan means something: the approver was not "whoever
      had the admin password" but the person who holds the seat the SOP names.</p>`,
  },
  {
    id: "request", icon: "📥", title: "Raising your first request",
    body: () => `
      <div class="grid g4" style="margin:14px 0">
        ${["Pick the service|Tasks & Projects → New request. Each service shows its SLA before you commit.",
           "Describe it|A title and what you need. Your identity attaches from your session — no forms about who you are.",
           "Watch it move|The timeline fills in live: the agent's draft, the gate, the action. Refresh-safe — the link is shareable.",
           "Read the outcome|Done means done: the work product is on the request, with SLA verdicts (met, late) stated plainly."]
          .map((t, i) => { const [a, b] = t.split("|"); return `
          <div class="card"><div style="font-size:20px;font-weight:800;color:var(--accent)">${i + 1}</div>
          <b>${esc(a)}</b><div class="hint">${esc(b)}</div></div>`; }).join("")}
      </div>
      <div class="card">
        <h3>Reading a timeline</h3>
        <div class="grid g4" style="gap:8px">
          ${[["📥 created", "your request, on the ledger"], ["🚀 dispatched", "the SOP started running"],
             ["🤖 agent_output", "an agent produced real work"], ["🚧 gate_raised", "a human decision is needed"],
             ["✅ gate_responded", "a named person decided"], ["⚙️ action_executed", "a capability performed a verb"],
             ["🏁 completed", "outcome recorded"], ["⚠ failed", "stopped — with the reason, never silently"]]
            .map(([k, v]) => `<div><b style="font-size:12px">${esc(k)}</b><div class="hint">${esc(v)}</div></div>`).join("")}
        </div>
      </div>`,
  },
  {
    id: "inbox", icon: "🚧", title: "Your approval inbox",
    body: () => `
      <p>When an SOP needs a human, it does not post to a shared pile and hope. The step names a
      role; the org chart resolves it to a seat; the seat resolves to <b>you</b>; the approval lands
      in your personal queue.</p>
      ${diagramGateRouting()}
      <p class="hint">Supervision → your queue shows only what needs <i>you</i>. Approve or reject with
      a comment; the run resumes or stops immediately, and the request's timeline names you either way.</p>`,
  },
  {
    id: "departments", icon: "🏢", title: "Deploying a department",
    body: () => `
      <p>A department is deployed from a <b>template</b>: a complete operating blueprint — org chart
      with autonomy tiers and decision rights, the service portfolio with SLAs, the SOPs that deliver
      each service, KPIs, risks and controls. Deploy walks six stages and ends with a live department.</p>
      ${diagramDeploy()}`,
  },
  {
    id: "people", icon: "🧑‍💼", title: "People & seats",
    body: () => `
      <p>The People screen is the bridge between accounts and authority. Add a person, set their
      password, and <b>assign them a seat</b> — that assignment is the moment they become a department
      head or an approver, because gates route to seats, not to accounts.</p>
      <div class="grid g3" style="margin:14px 0">
        <div class="card"><b>1 · Add the person</b><div class="hint">An account that can sign in and raise requests. Nothing more yet — honestly.</div></div>
        <div class="card"><b>2 · Set a credential</b><div class="hint">Setting the password activates the account. Deactivating it really does lock them out.</div></div>
        <div class="card"><b>3 · Assign a seat</b><div class="hint">Vacant and agent-held seats are both assignable — putting a human in an agent's seat is how a department gets a human head.</div></div>
      </div>
      <p class="hint">"Seats no person holds" is not an error list — it is where you decide which
      decisions deserve a human.</p>`,
  },
  {
    id: "capabilities", icon: "🗣", title: "Capabilities — how agents act",
    body: () => `
      <p>Agents drafting text is half a department. The other half is <b>doing</b> — assigning the
      ticket, granting the access, running the restore. Operan's rule: SOPs name <b>business verbs</b>,
      never vendors, and every execution passes one governed funnel. Anyone can call an API; what the
      funnel produces is the sentence <i>"an AI performed this action, within its authority, under
      policy — and here is the record."</i></p>
      ${diagramCapability()}
      <p class="hint">Today every verb is performed by the <b>simulated provider</b>: realistic
      responses, nothing touched, SIMULATED stamped on every record and screen. Pointing a verb at
      your real ITSM or directory is a binding change — the SOPs do not change at all.</p>`,
  },
  {
    id: "measurement", icon: "📊", title: "Measurement & briefings",
    body: () => `
      <p>Performance numbers are computed from the request ledger — SLA compliance, cycle time, gate
      turnaround, token spend — and each morning the department's head agent drafts a briefing from
      those real figures, leading with whatever needs a human decision.</p>
      ${diagramMeasurement()}`,
  },
  {
    id: "map", icon: "🗺", title: "What runs underneath",
    body: () => `
      <p>Twenty-one services, four jobs. You will spend your day in the top two rows; the rest exist
      so the top two can be trusted.</p>
      ${diagramModules()}`,
  },
];

// ─── View ───────────────────────────────────────────────────────────────────

export default async function viewGuide() {
  const who = session.displayName || "there";
  return `<div id="guideRoot">
    <div class="card" style="margin-bottom:16px">
      <h3>📖 The Operan guide</h3>
      <div class="hint">Hello ${esc(who)} — nine short chapters, each drawn before it is described.
      Everything here reflects how the platform actually behaves, including its refusals.</div>
      <div style="display:flex;gap:8px;margin-top:12px;flex-wrap:wrap">
        ${CHAPTERS.map((c, i) => `<button class="sm guide-tab ${i === 0 ? "primary" : "ghost"}"
          onclick="window.guideChapter('${c.id}', this)">${c.icon} ${esc(c.title)}</button>`).join("")}
      </div>
    </div>
    ${CHAPTERS.map((c, i) => `
      <div class="card guide-chapter" id="guide-${c.id}" style="${i === 0 ? "" : "display:none"}">
        <h3>${c.icon} ${esc(c.title)}</h3>
        ${c.body()}
      </div>`).join("")}
  </div>`;
}

window.guideChapter = function (id, btn) {
  document.querySelectorAll(".guide-chapter").forEach(el => { el.style.display = "none"; });
  const el = document.getElementById("guide-" + id);
  if (el) el.style.display = "";
  document.querySelectorAll(".guide-tab").forEach(b => { b.classList.remove("primary"); b.classList.add("ghost"); });
  if (btn) { btn.classList.add("primary"); btn.classList.remove("ghost"); }
};
