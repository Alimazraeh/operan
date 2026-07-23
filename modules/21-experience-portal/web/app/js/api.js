// Operan Portal API Client
// Real login via M02 (IAM), session management, tenant CRUD, and all service proxies.

// ── Service endpoints ──────────────────────────────────────
export const SVC = {
  tenant:      "/svc/tenant",
  orchestration: "/svc/orchestration/api/v1/orchestration",
  registry:    "/svc/registry",
  templates:   "/svc/templates",
  knowledge:   "/svc/knowledge",
  memory:      "/svc/memory",
  tools:       "/svc/tools",
  supervision: "/svc/supervision",
  policies:    "/svc/policies",
  observability: "/svc/observability",
  cost:        "/svc/cost",
  connectors:  "/svc/connectors",
  iam:         "/svc/iam",
};

// ── Session store ──────────────────────────────────────────
export const session = {
  jwt: "",
  tenant: "",
  userId: "",
  email: "",
  role: "",
  get active() { return !!this.jwt; },
};

// ── UUID helper ────────────────────────────────────────────
export function uuid4() {
  const b = crypto.getRandomValues(new Uint8Array(16));
  b[6] = (b[6] & 0x0f) | 0x40; b[8] = (b[8] & 0x3f) | 0x80;
  const h = [...b].map(x => x.toString(16).padStart(2, "0")).join("");
  return `${h.slice(0,8)}-${h.slice(8,12)}-${h.slice(12,16)}-${h.slice(16,20)}-${h.slice(20)}`;
}

// ── Auth: M02 password-file login ──────────────────────────
// POST /svc/iam/admin/login → { token, user_id, email }
export async function login(password, tenantId) {
  const resp = await fetch(SVC.iam + "/admin/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ password, tenant: tenantId || "default-tenant" }),
  });
  if (!resp.ok) {
    const err = await resp.json().catch(() => ({}));
    throw new Error(err.error?.message || `Login failed (${resp.status})`);
  }
  const data = await resp.json();
  session.jwt = data.token;
  session.userId = data.user_id;
  session.email = data.email;
  session.role = "admin";
  return data;
}

// ── Auth: register a new tenant (M01) ─────────────────────
// POST /svc/tenant/tenants → { tenant object }
export async function registerTenant(name, slug, plan) {
  return post("/svc/tenant/tenants", {
    name, slug: slug || name.toLowerCase().replace(/\s+/g, "-"),
    plan: plan || "enterprise",
    status: "provisioning",
    isolation_config: {
      namespace: slug || name.toLowerCase().replace(/\s+/g, "-"),
      encryption_algorithm: "aes-256-gcm",
      network_policy: "default-deny",
    },
    quota: {
      max_agents: plan === "small" ? 5 : plan === "medium" ? 25 : 100,
      max_workflows: plan === "small" ? 10 : plan === "medium" ? 50 : 200,
      max_templates: plan === "small" ? 3 : plan === "medium" ? 10 : 50,
      cpu_limit_millicores: plan === "small" ? 2000 : plan === "medium" ? 8000 : 32000,
      memory_limit_mb: plan === "small" ? 4096 : plan === "medium" ? 16384 : 65536,
    },
  });
}

// ── Auth: password generation endpoint (M02) ──────────────
export async function generateAdminPassword() {
  const resp = await fetch(SVC.iam + "/admin/generate-password", { method: "POST" });
  if (!resp.ok) throw new Error("Password generation failed");
  return resp.json();
}

// ── HTTP helpers ───────────────────────────────────────────
async function api(method, url, body) {
  const headers = {
    "X-Tenant-ID": session.tenant,
    "Content-Type": "application/json",
  };
  if (session.jwt) {
    headers["Authorization"] = "Bearer " + session.jwt;
  }
  const res = await fetch(url, { method, headers, body: body ? JSON.stringify(body) : undefined });
  let data = null;
  try { data = await res.json(); } catch (_) {}
  if (!res.ok && data?.error?.message) {
    throw new Error(data.error.message);
  }
  return { ok: res.ok, status: res.status, data };
}

export function get(url) { return api("GET", url); }
export function post(url, body) { return api("POST", url, body); }
export function patch(url, body) { return api("PATCH", url, body); }
export function del(url) { return api("DELETE", url); }

// ── Tenant CRUD ────────────────────────────────────────────
export function listTenants(page, pageSize) {
  const params = new URLSearchParams();
  if (page) params.set("page", page);
  if (pageSize) params.set("page_size", pageSize);
  return get("/svc/tenant/tenants?" + params.toString());
}

export function getTenant(id) { return get("/svc/tenant/tenants/" + id); }
export function updateTenant(id, data) { return patch("/svc/tenant/tenants/" + id, data); }

// ── Department Template CRUD ───────────────────────────────
export function listTemplates(page, pageSize) {
  const params = new URLSearchParams();
  if (page) params.set("page", page);
  if (pageSize) params.set("page_size", pageSize);
  return get("/svc/templates/templates?" + params.toString());
}

export function deployTemplate(templateId, config) {
  return post("/svc/templates/deployments", {
    template_id: templateId,
    name: config.name || "Deployed Department",
    department_id: config.department_id || uuid4(),
    agent_count: config.agent_count || 3,
    agent_sizes: config.agent_sizes || "small",
    status: "select",
  });
}

// ── Agent Registry ─────────────────────────────────────────
export function listAgents(page, pageSize) {
  const params = new URLSearchParams();
  if (page) params.set("page", page);
  if (pageSize) params.set("page_size", pageSize);
  return get("/svc/registry/agents?" + params.toString());
}

export function createAgent(name, role, capabilities, departmentId) {
  return post("/svc/registry/agents", {
    name, role, capabilities, department_id: departmentId,
    status: "active", version: "1.0.0",
  });
}

// ── Workflow / Orchestration ───────────────────────────────
export function listWorkflows(page, pageSize) {
  const params = new URLSearchParams();
  if (page) params.set("page", page);
  if (pageSize) params.set("page_size", pageSize);
  return get("/svc/orchestration/api/v1/orchestration/workflows?" + params.toString());
}

export function createWorkflow(name, nodes, edges) {
  return post("/svc/orchestration/api/v1/orchestration/workflows", {
    name, status: "draft", nodes, edges,
    variables: {}, checkpoint_interval: "1m",
  });
}

export function executeWorkflow(id) {
  return post(`/svc/orchestration/api/v1/orchestration/workflows/${id}/execute`, {});
}

// ── Human Supervision ──────────────────────────────────────
export function listSupervisionQueue(page, pageSize) {
  const params = new URLSearchParams();
  if (page) params.set("page", page);
  if (pageSize) params.set("page_size", pageSize);
  return get("/svc/supervision/queue?" + params.toString());
}

export function respondGate(gateId, response, comment) {
  return post(`/svc/supervision/queue/${gateId}/respond`, { response, comment });
}

// ── Observability ──────────────────────────────────────────
export function listSpans(service, fromDate, toDate, page, pageSize) {
  const params = new URLSearchParams();
  if (service) params.set("service", service);
  if (fromDate) params.set("from", fromDate);
  if (toDate) params.set("to", toDate);
  if (page) params.set("page", page);
  if (pageSize) params.set("page_size", pageSize);
  return get("/svc/observability/spans?" + params.toString());
}

// ── Cost Governance ────────────────────────────────────────
export function listCostEvents(page, pageSize) {
  const params = new URLSearchParams();
  if (page) params.set("page", page);
  if (pageSize) params.set("page_size", pageSize);
  return get("/svc/cost/events?" + params.toString());
}

export function listBudgets(page, pageSize) {
  const params = new URLSearchParams();
  if (page) params.set("page", page);
  if (pageSize) params.set("page_size", pageSize);
  return get("/svc/cost/budgets?" + params.toString());
}

// ── Policies ───────────────────────────────────────────────
export function listPolicies(page, pageSize) {
  const params = new URLSearchParams();
  if (page) params.set("page", page);
  if (pageSize) params.set("page_size", pageSize);
  return get("/svc/policies/policies?" + params.toString());
}

export function evaluatePolicy(context) {
  return post("/svc/policies/evaluate", {
    context,
    policy_ids: [],
  });
}

// ── Connectors ─────────────────────────────────────────────
export function listConnectors(page, pageSize) {
  const params = new URLSearchParams();
  if (page) params.set("page", page);
  if (pageSize) params.set("page_size", pageSize);
  return get("/svc/connectors/connectors?" + params.toString());
}

export function createConnector(name, type, config) {
  return post("/svc/connectors/connectors", { name, type, config, status: "disconnected" });
}

// ── Health probe ───────────────────────────────────────────
export async function probeService(path) {
  try { const r = await fetch(path); return r.ok; } catch { return false; }
}