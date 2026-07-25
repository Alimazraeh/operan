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
  displayName: "",
  role: "",
  roles: [],
  get active() { return !!this.jwt; },
};

// ── UUID helper ────────────────────────────────────────────
export function uuid4() {
  const b = crypto.getRandomValues(new Uint8Array(16));
  b[6] = (b[6] & 0x0f) | 0x40; b[8] = (b[8] & 0x3f) | 0x80;
  const h = [...b].map(x => x.toString(16).padStart(2, "0")).join("");
  return `${h.slice(0,8)}-${h.slice(8,12)}-${h.slice(12,16)}-${h.slice(16,20)}-${h.slice(20)}`;
}

// ── Auth: real per-user login (M02) ────────────────────────
// POST /svc/iam/auth/login → { token, user_id, email, display_name, roles }
// The token carries that person's own id, so every request they raise and
// every gate they decide is attributed to them rather than to a shared
// synthetic admin.
export async function loginUser(email, password, tenantId) {
  const resp = await fetch(SVC.iam + "/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password, tenant: tenantId }),
  });
  if (!resp.ok) {
    const err = await resp.json().catch(() => ({}));
    throw new Error(err.error || `Sign-in failed (${resp.status})`);
  }
  const data = await resp.json();
  session.jwt = data.token;
  session.tenant = tenantId;
  session.userId = data.user_id;
  session.email = data.email;
  session.displayName = data.display_name || data.email;
  session.roles = data.roles || [];
  session.role = session.roles[0] || "user";
  return data;
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
  session.tenant = tenantId || "default-tenant"; // every /svc call sends this as X-Tenant-ID
  session.userId = data.user_id;
  session.email = data.email;
  session.displayName = "Platform admin";
  session.roles = ["platform_admin"];
  session.role = "platform_admin";
  return data;
}

// ── Auth: register a new tenant (M01) ─────────────────────
// POST /svc/tenant/tenants → { tenant object }
// M01's real contract: {name, plan saas|enterprise|sovereign, region,
// isolation_level} — the record's name carries the workspace slug, the
// human title goes in display_name, and M01 assigns the UUID id.
export async function registerTenant(name, slug, plan) {
  const planMap = { small: "saas", medium: "saas", starter: "saas", saas: "saas", enterprise: "enterprise", sovereign: "sovereign" };
  const p = planMap[plan] || "saas";
  return post("/svc/tenant/tenants", {
    name: slug || name.toLowerCase().replace(/\s+/g, "-"),
    display_name: name,
    plan: p,
    region: "me-east-1",
    isolation_level: "namespace",
    quota: {
      max_agents: p === "saas" ? 25 : 100,
      max_workflows_per_day: p === "saas" ? 200 : 1000,
      max_storage_gb: p === "saas" ? 20 : 100,
      max_monthly_tokens: p === "saas" ? 5000000 : 50000000,
      max_concurrent_workflows: p === "saas" ? 10 : 50,
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

// unwrapList extracts the list payload from any of the platform's response
// envelopes: bare arrays, {items:[]}, {data:[]}, or a module-specific named
// key ({workflows:[]}, {budgets:[]}, …). Errors and non-list payloads yield []
// so views never call .filter/.reduce on an error object.
export function unwrapList(resp, key) {
  if (!resp || !resp.ok) return [];
  const d = resp.data;
  if (Array.isArray(d)) return d;
  if (!d || typeof d !== "object") return [];
  if (Array.isArray(d.items)) return d.items;
  if (Array.isArray(d.data)) return d.data;
  if (key && Array.isArray(d[key])) return d[key];
  return [];
}

export function get(url) { return api("GET", url); }
export function post(url, body) { return api("POST", url, body); }
export function patch(url, body) { return api("PATCH", url, body); }
export function put(url, body) { return api("PUT", url, body); }
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

// ── Department Template CRUD (Module 05) ───────────────────
export function listTemplates(page, pageSize) {
  const params = new URLSearchParams();
  if (page) params.set("page", page);
  if (pageSize) params.set("page_size", pageSize);
  return get("/svc/templates/templates?" + params.toString());
}

// Server-orchestrated deploy: Module 05 creates the Department instance and
// walks the pipeline itself; poll getDeployment until operational/failed.
export function deployTemplateReal(templateId, body) {
  return post(`/svc/templates/templates/${templateId}/deploy`,
    body || { environment: "production", configuration: { region: "me-central" } });
}

export function getDeployment(templateId, deploymentId) {
  return get(`/svc/templates/templates/${templateId}/deployments/${deploymentId}`);
}

export function seedTemplates() {
  return post("/svc/templates/templates/seed", {});
}

export function getTemplate(id) { return get("/svc/templates/templates/" + id); }

// ── Departments (Module 05 instances — the operating model) ─
export function listDepartments(page, pageSize) {
  const params = new URLSearchParams();
  if (page) params.set("page", page);
  if (pageSize) params.set("page_size", pageSize);
  return get("/svc/templates/departments?" + params.toString());
}

export function getDepartment(id) { return get("/svc/templates/departments/" + id); }

// The department's front door: a service request enters the work loop —
// M05 compiles the service's SOP into a per-request M03 workflow and runs it.
export function createServiceRequest(departmentId, serviceId, title, body, priority) {
  return post(`/svc/templates/departments/${departmentId}/requests`,
    { service_id: serviceId, title, body, priority });
}

export function listDeptRequests(departmentId) {
  return get(`/svc/templates/departments/${departmentId}/requests`);
}
export function getDeptOrgChart(id) { return get(`/svc/templates/departments/${id}/org-chart`); }
export function getDeptServices(id) { return get(`/svc/templates/departments/${id}/services`); }
export function getDeptValueChain(id) { return get(`/svc/templates/departments/${id}/value-chain`); }
export function getDeptRisks(id) { return get(`/svc/templates/departments/${id}/risks`); }
export function getDeptQuality(id) { return get(`/svc/templates/departments/${id}/quality`); }
export function getDeptCompliance(id) { return get(`/svc/templates/departments/${id}/compliance`); }
export function getDeptMeasurements(id) { return get(`/svc/templates/departments/${id}/kpi-measurements`); }

// Who sits in a seat. A human binding is verified against M02 server-side —
// a seat pointing at an id nobody can produce looks staffed and grants nothing.
export function setPositionHolder(departmentId, positionId, body) {
  return put(`/svc/templates/departments/${departmentId}/org-chart/${positionId}/holder`, body);
}

export function listIamUsers() { return get("/svc/iam/users?page_size=100"); }
export function setUserPassword(userId, password) {
  return post(`/svc/iam/users/${userId}/password`, { password });
}
export function createIamUser(email, displayName, roleIds) {
  return post("/svc/iam/users", { email, display_name: displayName, role_ids: roleIds });
}

// ── Agent Registry (Module 04) ─────────────────────────────
export function listAgents(page, pageSize) {
  const params = new URLSearchParams();
  if (page) params.set("page", page);
  if (pageSize) params.set("page_size", pageSize);
  return get("/svc/registry/registry/agents?" + params.toString());
}

// M04 contract: capabilities is a []string; the server assigns the id and
// sets status=active. tenant_id must match the X-Tenant-ID header.
export function createAgent(name, role, capabilities, departmentId) {
  return post("/svc/registry/registry/agents", {
    tenant_id: session.tenant,
    name, role,
    capabilities: (capabilities || []).map(c => typeof c === "string" ? c : c.capability || c.name || String(c)),
    department_id: departmentId || "",
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

export function listHumanTasks() {
  return get("/svc/orchestration/api/v1/orchestration/human-tasks");
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
  return get("/svc/cost/v1/cost-events?" + params.toString());
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

// ── Connectors (Module 18: base path /v1, connector_type key) ─
export function listConnectors(page, pageSize) {
  const params = new URLSearchParams();
  if (page) params.set("page", page);
  if (pageSize) params.set("page_size", pageSize);
  return get("/svc/connectors/v1/connectors?" + params.toString());
}

export function createConnector(name, connectorType, config) {
  return post("/svc/connectors/v1/connectors", {
    name, connector_type: connectorType,
    auth_method: "api_key", config: config || {}, credentials: {},
    sync_frequency: "manual",
  });
}

// ── Health probe ───────────────────────────────────────────
export async function probeService(path) {
  try { const r = await fetch(path); return r.ok; } catch { return false; }
}