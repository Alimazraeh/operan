// Operan portal: login, router, shell.
import {SVC, session, mintJWT, get, uuid4} from "./api.js";
import {$, esc, toast} from "./ui.js";
import {viewOverview} from "./views/overview.js";
import {viewDepartments, viewDepartment} from "./views/departments.js";
import {viewAgents, viewAgent} from "./views/agents.js";
import {viewWorkflows} from "./views/workflows.js";
import {viewSupervision} from "./views/supervision.js";
import {viewTools} from "./views/tools.js";
import {viewObservability} from "./views/observability.js";
import {viewScenario} from "./views/scenario.js";

const VIEWS = {
  overview:      {title: "Overview",      render: viewOverview},
  departments:   {title: "Departments",   render: viewDepartments},
  department:    {title: "Department",    render: viewDepartment, parent: "departments"},
  agents:        {title: "Agents",        render: viewAgents},
  agent:         {title: "Agent",         render: viewAgent, parent: "agents"},
  workflows:     {title: "Workflows",     render: viewWorkflows},
  supervision:   {title: "Supervision",   render: viewSupervision},
  tools:         {title: "Tools",         render: viewTools},
  observability: {title: "Observability", render: viewObservability},
  scenario:      {title: "The Story",     render: viewScenario},
};

let current = "overview";

window.go = async function (name, ...args) {
  const v = VIEWS[name];
  if (!v) return;
  current = name;
  document.querySelectorAll(".navlink").forEach(el =>
    el.classList.toggle("active", el.dataset.view === (v.parent || name)));
  $("crumb").textContent = v.title;
  $("view").innerHTML = `<div class="empty">Loading…</div>`;
  try {
    $("view").innerHTML = await v.render(...args);
  } catch (e) {
    console.error(e);
    $("view").innerHTML = `<div class="card"><div class="empty">This view hit an error — see the console.</div></div>`;
  }
};

// ─── Login ───────────────────────────────────────────────────────────────────
const PROBES = [
  ["tenant", "/svc/tenant/health"], ["orchestr", "/svc/orchestration/health"],
  ["registry", "/svc/registry/health"], ["templates", "/svc/templates/health"],
  ["memory", "/svc/memory/health"], ["tools", "/svc/tools/health"],
  ["supervision", "/svc/supervision/health"], ["observab", "/svc/observability/healthz"],
];

async function connect() {
  const secret = $("loginSecret").value.trim();
  const msg = $("loginMsg");
  if (!secret) { msg.textContent = "Enter the signing secret first."; return; }
  if (/^[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}$/.test(secret)) {
    msg.innerHTML = 'That looks like a JWT <i>token</i> — paste the <b>signing secret</b> instead.';
    return;
  }
  if (!$("loginTenant").value.trim()) $("loginTenant").value = uuid4();
  session.tenant = $("loginTenant").value.trim();
  session.jwt = await mintJWT(secret);
  localStorage.setItem("operan.tenant", session.tenant);

  const probe = await get(SVC.supervision + "/queue?page_size=1");
  if (probe.status === 401) {
    session.jwt = "";
    msg.innerHTML = '<b>Secret rejected (401).</b> Paste the output of <code>kubectl -n operan get secret operan-jwt -o jsonpath="{.data.secret}" | base64 -d</code>';
    return;
  }

  $("healthdots").innerHTML = PROBES.map(([n]) =>
    `<span class="dot" id="dot-${n}"><i></i>${n}</span>`).join("");
  PROBES.forEach(async ([n, path]) => {
    try {
      const r = await fetch(path);
      $("dot-" + n).className = "dot " + (r.ok ? "ok" : "bad");
    } catch (_) { $("dot-" + n).className = "dot bad"; }
  });

  $("tenantChip").textContent = "tenant " + session.tenant;
  $("login").style.display = "none";
  $("shell").style.display = "flex";
  window.go("overview");

  // Gentle live refresh for passive views.
  setInterval(() => {
    if (current === "overview" || current === "observability" || current === "supervision") {
      window.go(current);
    }
  }, 12000);
}

document.querySelectorAll(".navlink").forEach(el =>
  el.addEventListener("click", () => window.go(el.dataset.view)));
$("btnConnect").addEventListener("click", connect);
$("btnNewTenant").addEventListener("click", () => { $("loginTenant").value = uuid4(); });
$("loginSecret").addEventListener("keydown", e => { if (e.key === "Enter") connect(); });
$("btnLogout").addEventListener("click", () => location.reload());
$("loginTenant").value = localStorage.getItem("operan.tenant") || "";
