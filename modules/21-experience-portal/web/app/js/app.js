// Operan Portal — Main App Router & Auth
import { SVC, session, uuid4, login, loginUser, registerTenant, probeService, get } from "./api.js";
import { $, esc, toast } from "./ui.js";

// ── Import new views ───────────────────────────────────────
import viewDashboard from "./views/dashboard.js";
import viewTeams from "./views/teams.js";
import viewTasks from "./views/tasks.js";
import viewReports from "./views/reports.js";
import viewSettings from "./views/settings.js";
import viewPeople from "./views/people.js";
import viewCapabilities from "./views/capabilities.js";

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
import { viewIAM } from "./views/iam.js";
import { loadAuthz, can, authorityLabel, isUnbound, authz } from "./perm.js";

// ── View registry ──────────────────────────────────────────
// Each view declares its hash route and the permission it needs. `requires` is
// a UI affordance, not a security boundary — every service enforces its own
// authorization. It exists so nobody acts under authority they do not have by
// stumbling into a screen.
const VIEWS = {
  dashboard:    { title: "Dashboard", route: "/", render: viewDashboard },
  departments:  { title: "Departments", route: "/departments", render: viewDepartments, requires: "department.read" },
  department:   { title: "Department", route: "/departments/:id", render: viewDepartment, parent: "departments", requires: "department.read" },
  teams:        { title: "Teams", route: "/teams", render: viewTeams, requires: "department.read" },
  tasks:        { title: "Tasks & Projects", route: "/tasks", render: viewTasks },
  workflows:    { title: "Workflows", route: "/workflows", render: viewWorkflows, requires: "department.read" },
  supervision:  { title: "Supervision", route: "/supervision", render: viewSupervision, requires: "approval.read" },
  policies:     { title: "Policies", route: "/policies", render: viewPolicies, requires: "department.read" },
  reports:      { title: "Reports", route: "/reports", render: viewReports, requires: "kpi.read" },
  cost:         { title: "Costs", route: "/costs", render: viewCost, requires: "kpi.read" },
  people:       { title: "People", route: "/people", render: viewPeople, requires: "people.read" },
  agents:       { title: "Agents", route: "/agents", render: viewAgents, requires: "department.read" },
  agent:        { title: "Agent", route: "/agents/:id", render: viewAgent, parent: "agents", requires: "department.read" },
  ingestion:    { title: "Knowledge", route: "/knowledge", render: viewIngestion, requires: "department.read" },
  capabilities: { title: "Capabilities", route: "/capabilities", render: viewCapabilities, requires: "platform.admin" },
  connectors:   { title: "Connectors", route: "/connectors", render: viewConnectors, requires: "platform.admin" },
  tenants:      { title: "Tenants", route: "/tenants", render: viewTenants, requires: "platform.admin" },
  iam:          { title: "Identity & Access", route: "/iam", render: viewIAM, requires: "platform.admin" },
  settings:     { title: "Settings", route: "/settings", render: viewSettings },
  observability:{ title: "Observability", route: "/observability", render: viewObservability, requires: "platform.admin" },
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
    try {
      session.roles = JSON.parse(localStorage.getItem("operan.roles") || "[]");
    } catch (_) { session.roles = []; }
    session.role = session.roles[0] || "";
    session.displayName = localStorage.getItem("operan.displayName") || session.email;
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
// Views are addressable. With an approval inbox and several personas, the
// primary interaction is "here is the thing that needs you" — in an email or a
// message — so a request or a department must have a link. Hash routing needs
// no server change: the portal is a Go binary serving an embedded filesystem.
let currentView = "dashboard";
let suppressHashWrite = false;

// hashFor renders a view + args back into a hash, filling :params in order.
function hashFor(name, args) {
  const route = VIEWS[name] && VIEWS[name].route;
  if (!route) return "";
  let i = 0;
  const path = route.replace(/:[^/]+/g, () => encodeURIComponent(args[i++] ?? ""));
  const trimmed = path.replace(/\/+$/, "");
  return trimmed === "" ? "#/" : "#" + trimmed;
}

// matchHash resolves a hash back to a view and its positional args.
function matchHash(hash) {
  const path = (hash || "").replace(/^#/, "") || "/";
  const parts = path.split("/").filter(Boolean);
  for (const [name, v] of Object.entries(VIEWS)) {
    if (!v.route) continue;
    const rparts = v.route.split("/").filter(Boolean);
    if (rparts.length !== parts.length) continue;
    const args = [];
    let ok = true;
    for (let i = 0; i < rparts.length; i++) {
      if (rparts[i].startsWith(":")) args.push(decodeURIComponent(parts[i]));
      else if (rparts[i] !== parts[i]) { ok = false; break; }
    }
    if (ok) return { name, args };
  }
  return null;
}

// renderDenied is shown instead of a view the signed-in person may not open.
// Being told plainly is better than an empty screen or a wall of failed calls.
function renderDenied(v) {
  return `<div class="error-box">
    <h3>Not available to you</h3>
    <p>${esc(v.title)} needs the <code>${esc(v.requires || "")}</code> permission, which your
    current authority does not include${isUnbound() ? " — you do not hold a seat in any department yet" : ""}.</p>
    <button class="ghost" onclick="window.go('dashboard')">Go to your dashboard</button>
  </div>`;
}

window.go = async function (name, ...args) {
  if (!VIEWS[name]) return;
  const v = VIEWS[name];
  currentView = name;
  document.querySelectorAll(".navlink").forEach(el =>
    el.classList.toggle("active", el.dataset.view === (v.parent || name)));

  const crumb = $("crumb");
  if (crumb) crumb.textContent = v.title;

  // Keep the address bar in step so the current screen can be shared.
  const h = hashFor(name, args);
  if (h && location.hash !== h) {
    suppressHashWrite = true;
    location.hash = h;
    setTimeout(() => { suppressHashWrite = false; }, 0);
  }

  const viewEl = $("view");
  if (!viewEl) return;

  if (v.requires && !can(v.requires)) {
    viewEl.innerHTML = renderDenied(v);
    return;
  }

  viewEl.innerHTML = `<div class="card"><div class="skel-line w60"></div><div class="skel-line w80"></div><div class="skel-line"></div></div>`;
  try {
    viewEl.innerHTML = await v.render(...args);
  } catch (e) {
    console.error(e);
    viewEl.innerHTML = `<div class="error-box"><h3>Error</h3><p>${esc(String(e))}</p><button class="ghost" onclick="window.go('${name}')">Retry</button></div>`;
  }
};

// goFromHash routes the current address, falling back to the dashboard.
async function goFromHash() {
  if (suppressHashWrite) return;
  const m = matchHash(location.hash);
  if (m) await window.go(m.name, ...m.args);
  else await window.go("dashboard");
}
window.addEventListener("hashchange", () => { if (!suppressHashWrite) goFromHash(); });

// ── Login handler ──────────────────────────────────────────
async function handleLogin() {
  const email = ($("#loginEmail")?.value || "").trim();
  const password = $("#loginSecret").value.trim();
  const tenantId = $("#loginTenant").value.trim();
  const msg = $("#loginMsg");

  if (!password) { msg.textContent = "Enter your password."; msg.className = "err"; return; }
  if (!tenantId) { msg.textContent = "Enter a tenant ID or click New."; msg.className = "err"; return; }

  msg.textContent = "Authenticating…";
  msg.className = "err loading";

  try {
    // With an email this is a real person signing in, and everything they do
    // is attributed to them. Without one it is the shared admin bootstrap.
    if (email) {
      await loginUser(email, password, tenantId);
    } else {
      await login(password, tenantId);
    }
    localStorage.setItem("operan.jwt", session.jwt);
    localStorage.setItem("operan.tenant", session.tenant);
    localStorage.setItem("operan.userId", session.userId);
    localStorage.setItem("operan.email", session.email);
    localStorage.setItem("operan.roles", JSON.stringify(session.roles || []));
    localStorage.setItem("operan.displayName", session.displayName || "");
    msg.textContent = "";
    ensureTenantRecord(); // async bookkeeping — M01 record for this workspace
    // Authority must be known before the first screen renders, or the nav
    // flashes options this person cannot use.
    await loadAuthz();
    renderDashboard();
    setupShell();
    await goFromHash();
  } catch (e) {
    msg.textContent = e.message || "Authentication failed";
    msg.className = "err";
  }
}

// applyNavPermissions hides links the signed-in person cannot open, and any
// section heading left with nothing under it.
function applyNavPermissions() {
  document.querySelectorAll(".navlink").forEach(el => {
    const v = VIEWS[el.dataset.view];
    const allowed = !v || !v.requires || can(v.requires);
    el.style.display = allowed ? "" : "none";
  });
  // A section heading with no visible links beneath it is noise.
  const sidebar = document.getElementById("sidebar");
  if (!sidebar) return;
  const kids = [...sidebar.children];
  kids.forEach((el, i) => {
    if (!el.classList.contains("nav-section")) return;
    let anyVisible = false;
    for (let j = i + 1; j < kids.length; j++) {
      if (kids[j].classList.contains("nav-section")) break;
      if (kids[j].classList.contains("navlink") && kids[j].style.display !== "none") {
        anyVisible = true;
        break;
      }
    }
    el.style.display = anyVisible ? "" : "none";
  });
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
      localStorage.removeItem("operan.roles");
      localStorage.removeItem("operan.displayName");
      session.roles = [];
      session.role = "";
      session.displayName = "";
      authz.permissions = new Set();
      authz.assignments = [];
      authz.loaded = false;
      location.hash = "";
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

  // Show only what this person may open, and say under whose authority they
  // are acting. Acting under the wrong authority must not be possible to do
  // unknowingly.
  applyNavPermissions();
  const chip = $("tenantChip");
  if (chip) {
    const who = session.displayName || session.email || "signed in";
    chip.innerHTML = `<b>${esc(who)}</b>` +
      `<br><span class="hint" style="margin:0">${esc(authorityLabel())}</span>` +
      `<br><span class="hint" style="margin:0">${esc(session.tenant || "")}</span>`;
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
  const loginEmail = $("#loginEmail");
  if (loginEmail) {
    loginEmail.value = localStorage.getItem("operan.email") === "admin@operan"
      ? "" : (localStorage.getItem("operan.email") || "");
    loginEmail.addEventListener("keydown", e => { if (e.key === "Enter") handleLogin(); });
  }

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
    // Resolve authority first so a deep link is judged against real
    // permissions rather than an empty set.
    loadAuthz().then(() => {
      renderDashboard();
      setupShell();
      goFromHash();
    });
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