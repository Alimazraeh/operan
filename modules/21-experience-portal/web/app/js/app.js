// Operan Portal — Main App Router & Auth
import { SVC, session, uuid4, login, registerTenant, probeService, get } from "./api.js";
import { $, esc, toast } from "./ui.js";

// ── Import new views ───────────────────────────────────────
import viewDashboard from "./views/dashboard.js";
import viewTeams from "./views/teams.js";
import viewTasks from "./views/tasks.js";
import viewReports from "./views/reports.js";
import viewSettings from "./views/settings.js";

// ── Import existing views (named exports) ──────────────────
import { viewDepartments, viewDepartment } from "./views/departments.js";
import { viewWorkflows } from "./views/workflows.js";
import { viewSupervision } from "./views/supervision.js";
import { viewAgents, viewAgent } from "./views/agents.js";
import { viewIngestion } from "./views/ingestion.js";
import { viewConnectors } from "./views/connectors.js";
import { viewTenants } from "./views/tenants.js";
import { viewPolicies } from "./views/policies.js";
import { viewCost } from "./views/cost.js";
import { viewObservability } from "./views/observability.js";

// ── View registry ──────────────────────────────────────────
const VIEWS = {
  dashboard:    { title: "Dashboard",     render: viewDashboard },
  departments:  { title: "Departments",   render: viewDepartments },
  department:   { title: "Department",    render: viewDepartment, parent: "departments" },
  teams:        { title: "Teams",         render: viewTeams },
  tasks:        { title: "Tasks & Projects", render: viewTasks },
  workflows:    { title: "Workflows",     render: viewWorkflows },
  supervision:  { title: "Supervision",   render: viewSupervision },
  policies:     { title: "Policies",      render: viewPolicies },
  reports:      { title: "Reports",       render: viewReports },
  cost:         { title: "Costs",         render: viewCost },
  agents:       { title: "Agents",        render: viewAgents },
  agent:        { title: "Agent",         render: viewAgent, parent: "agents" },
  ingestion:    { title: "Knowledge",     render: viewIngestion },
  connectors:   { title: "Connectors",    render: viewConnectors },
  settings:     { title: "Settings",      render: viewSettings },
  tenants:      { title: "Tenants",       render: viewTenants },
  observability:{ title: "Observability", render: viewObservability },
};

// Register views (some export both function and register)
if (typeof viewDashboard === 'function' && viewDashboard.length === 0) {
  // Already a render function, good
}
if (VIEWS.dashboard && VIEWS.dashboard.render.register) {
  VIEWS.dashboard.render = VIEWS.dashboard.render.register;
}

// ── Auth state ─────────────────────────────────────────────
function isAuthenticated() {
  return session.active && localStorage.getItem("operan.jwt");
}

async function restoreSession() {
  const jwt = localStorage.getItem("operan.jwt");
  const tenant = localStorage.getItem("operan.tenant");
  const userId = localStorage.getItem("operan.userId");
  const email = localStorage.getItem("operan.email");
  if (jwt && tenant) {
    // Never resurrect an expired token — it would render a shell where
    // every call 401s. Fall through to the login page instead.
    try {
      const p = jwt.split(".")[1].replace(/-/g, "+").replace(/_/g, "/");
      const claims = JSON.parse(atob(p + "=".repeat((4 - p.length % 4) % 4)));
      if (claims.exp && claims.exp * 1000 < Date.now()) return false;
    } catch (_) { return false; }
    session.jwt = jwt;
    session.tenant = tenant;
    session.userId = userId || "";
    session.email = email || "";
    return true;
  }
  return false;
}

// ── Render pages ──────────────────────────────────────────
window.renderLoginPage = function() {
  document.getElementById("shell").style.display = "none";
  document.getElementById("landing").style.display = "none";
  document.getElementById("register").style.display = "none";
  document.getElementById("login").style.display = "flex";
};

window.renderLandingPage = function() {
  document.getElementById("login").style.display = "none";
  document.getElementById("register").style.display = "none";
  document.getElementById("shell").style.display = "none";
  document.getElementById("landing").style.display = "block";
};

window.renderRegisterPage = function() {
  document.getElementById("login").style.display = "none";
  document.getElementById("landing").style.display = "none";
  document.getElementById("shell").style.display = "none";
  document.getElementById("register").style.display = "flex";
};

function renderDashboard() {
  document.getElementById("login").style.display = "none";
  document.getElementById("landing").style.display = "none";
  document.getElementById("register").style.display = "none";
  document.getElementById("shell").style.display = "flex";
}

// ── Router ─────────────────────────────────────────────────
let currentView = "dashboard";

window.go = async function (name, ...args) {
  if (!VIEWS[name]) return;
  currentView = name;
  const v = VIEWS[name];
  document.querySelectorAll(".navlink").forEach(el =>
    el.classList.toggle("active", el.dataset.view === (v.parent || name)));

  const crumb = $("crumb");
  if (crumb) crumb.textContent = v.title;

  const viewEl = $("view");
  if (viewEl) {
    viewEl.innerHTML = `<div class="card"><div class="skel-line w60"></div><div class="skel-line w80"></div><div class="skel-line"></div></div>`;
    try {
      viewEl.innerHTML = await v.render(...args);
    } catch (e) {
      console.error(e);
      viewEl.innerHTML = `<div class="error-box"><h3>Error</h3><p>${esc(String(e))}</p><button class="ghost" onclick="window.go('${name}')">Retry</button></div>`;
    }
  }
};

// ── Login handler ──────────────────────────────────────────
async function handleLogin() {
  const password = $("#loginSecret").value.trim();
  const tenantId = $("#loginTenant").value.trim();
  const msg = $("#loginMsg");

  if (!password) { msg.textContent = "Enter the admin password."; msg.className = "err"; return; }
  if (!tenantId) { msg.textContent = "Enter a tenant ID or click New."; msg.className = "err"; return; }

  msg.textContent = "Authenticating…";
  msg.className = "err loading";

  try {
    await login(password, tenantId);
    localStorage.setItem("operan.jwt", session.jwt);
    localStorage.setItem("operan.tenant", session.tenant);
    localStorage.setItem("operan.userId", session.userId);
    localStorage.setItem("operan.email", session.email);
    msg.textContent = "";
    ensureTenantRecord(); // async bookkeeping — M01 record for this workspace
    renderDashboard();
    setupShell();
    await window.go("dashboard");
  } catch (e) {
    msg.textContent = e.message || "Authentication failed";
    msg.className = "err";
  }
}

// ── Shell setup (after login) ──────────────────────────────
function setupShell() {
  renderDashboard();

  // Nav link clicks
  document.querySelectorAll(".navlink").forEach(el => {
    el.addEventListener("click", (e) => {
      e.preventDefault();
      window.go(el.dataset.view);
    });
  });

  // Logout
  const btnLogout = $("btnLogout");
  if (btnLogout) {
    btnLogout.addEventListener("click", () => {
      localStorage.removeItem("operan.jwt");
      localStorage.removeItem("operan.tenant");
      localStorage.removeItem("operan.userId");
      localStorage.removeItem("operan.email");
      session.jwt = "";
      session.tenant = "";
      session.userId = "";
      session.email = "";
      renderLandingPage();
    });
  }

  // Mobile sidebar
  const menuToggle = $("menuToggle");
  const sidebar = $("sidebar");
  if (menuToggle && sidebar) {
    menuToggle.addEventListener("click", () => sidebar.classList.toggle("open"));
    const overlay = sidebar.querySelector(".overlay");
    if (overlay) overlay.addEventListener("click", () => sidebar.classList.remove("open"));
  }

  // Health dots
  updateHealthDots();
}

async function updateHealthDots() {
  const probes = [
    ["tenant", "/svc/tenant/health"], ["iam", "/svc/iam/health"],
    ["templates", "/svc/templates/health"], ["registry", "/svc/registry/health"],
    ["orchestr", "/svc/orchestration/health"], ["supervision", "/svc/supervision/health"],
    ["observab", "/svc/observability/healthz"], ["cost", "/svc/cost/health"],
  ];
  const container = $("healthdots");
  if (!container) return;
  container.innerHTML = "";
  await Promise.all(probes.map(([name, path]) => {
    const dot = document.createElement("span");
    dot.className = "dot"; dot.id = "dot-" + name;
    dot.innerHTML = `<i></i>${name}`;
    container.appendChild(dot);
    return probeService(path).then(ok => {
      dot.className = "dot " + (ok ? "ok" : "bad");
    });
  }));
}

// ── Init ───────────────────────────────────────────────────
document.addEventListener("DOMContentLoaded", async () => {
  // Rehydrate the in-memory session from localStorage before any auth
  // check — otherwise a stored login never survives a page reload.
  await restoreSession();

  // Login page
  const btnConnect = $("#btnConnect");
  if (btnConnect) btnConnect.addEventListener("click", handleLogin);
  const loginSecret = $("#loginSecret");
  if (loginSecret) loginSecret.addEventListener("keydown", e => { if (e.key === "Enter") handleLogin(); });

  const btnNewTenant = $("#btnNewTenant");
  if (btnNewTenant) btnNewTenant.addEventListener("click", () => {
    $("#loginTenant").value = uuid4();
  });
  const loginTenant = $("#loginTenant");
  if (loginTenant) loginTenant.value = localStorage.getItem("operan.tenant") || "";

  // Landing page
  const btnGoLogin = $("#btnGoLogin");
  if (btnGoLogin) btnGoLogin.addEventListener("click", () => renderLoginPage());
  const btnLaunch = $("#btnLaunch");
  if (btnLaunch) btnLaunch.addEventListener("click", () => {
    if (isAuthenticated()) { renderDashboard(); setupShell(); window.go("dashboard"); }
    else renderLoginPage();
  });
  const btnHeroLogin = $("#btnHeroLogin");
  if (btnHeroLogin) btnHeroLogin.addEventListener("click", () => {
    if (isAuthenticated()) { renderDashboard(); setupShell(); window.go("dashboard"); }
    else renderLoginPage();
  });
  const btnHeroDemo = $("#btnHeroDemo");
  if (btnHeroDemo) btnHeroDemo.addEventListener("click", () => {
    toast("Demo video coming soon", "info");
  });

  // Register page
  const btnRegister = $("#btnRegister");
  if (btnRegister) btnRegister.addEventListener("click", handleRegister);

  // Check for existing session
  if (isAuthenticated()) {
    renderDashboard();
    setupShell();
    window.go("dashboard");
  } else {
    renderLandingPage();
  }
});

// ── Register handler ───────────────────────────────────────
async function handleRegister() {
  const name = $("#regName").value.trim();
  const slug = $("#regSlug").value.trim();
  const plan = $("#regPlan").value;
  const msg = $("#regMsg");

  if (!name) { msg.textContent = "Enter your company name."; return; }

  msg.textContent = "Provisioning…";
  msg.className = "err loading";

  // Tenant creation in Module 01 requires an authenticated token, so the
  // record is provisioned right after the first login (ensureTenantRecord).
  // Registration reserves the workspace name and hands off to login.
  const tenantSlug = slug || name.toLowerCase().replace(/\s+/g, "-");
  localStorage.setItem("operan.pendingTenant", JSON.stringify({ id: tenantSlug, name, plan }));
  renderLoginPage();
  const lt = $("#loginTenant");
  if (lt) lt.value = tenantSlug;
  localStorage.setItem("operan.tenant", tenantSlug);
  const lm = $("#loginMsg");
  if (lm) {
    lm.textContent = `Workspace “${name}” reserved — sign in with the platform admin password to provision it.`;
    lm.className = "err ok";
  }
  const ls = $("#loginSecret");
  if (ls) ls.focus();
}

// After the first authenticated login, make sure Module 01 has a tenant
// record for this workspace (idempotent: skips if it already exists).
async function ensureTenantRecord() {
  try {
    // M01 assigns UUID ids; workspace tenants are slugs — match either.
    const listR = await get(SVC.tenant + "/tenants?page_size=100");
    const items = (listR.data && listR.data.items) || [];
    if (items.some(t => t.id === session.tenant || t.slug === session.tenant || t.name === session.tenant)) return;
    const pending = JSON.parse(localStorage.getItem("operan.pendingTenant") || "null");
    const name = pending && pending.id === session.tenant ? pending.name : session.tenant;
    const plan = (pending && pending.plan) || "medium";
    const r = await registerTenant(name, session.tenant, plan);
    if (r.ok) localStorage.removeItem("operan.pendingTenant");
  } catch (_) { /* bookkeeping only — the platform works tenant-scoped regardless */ }
}