// Knowledge Ingestion Pipeline (Module 06).
import { SVC, get, post, patch, del, uuid4 } from "../api.js";
import { $, esc, badge, rel, toast, rowItem } from "../ui.js";

const DATA_SOURCES = [
  { id: "sharepoint", name: "SharePoint", icon: "📁", description: "Microsoft SharePoint sites and document libraries" },
  { id: "confluence", name: "Confluence", icon: "📝", description: "Atlassian Confluence spaces and pages" },
  { id: "jira", name: "Jira", icon: "🎫", description: "Jira issues, epics, and project documentation" },
  { id: "email", name: "Email", icon: "📧", description: "Outlook/Gmail mailboxes (ingest relevant threads)" },
  { id: "files", name: "File Upload", icon: "📄", description: "PDF, DOCX, TXT, CSV — direct upload" },
  { id: "database", name: "Database", icon: "🗄️", description: "PostgreSQL, MySQL — query-based ingestion" },
  { id: "api", name: "REST API", icon: "🔗", description: "Custom API endpoints returning JSON" },
  { id: "git", name: "Git Repository", icon: "🐙", description: "GitHub/GitLab repositories" },
];

export async function viewIngestion() {
  let sourcesR, jobsR, knowledgeR;
  try {
    [sourcesR, jobsR, knowledgeR] = await Promise.all([
      get(SVC.knowledge + "/sources?page_size=50"),
      get(SVC.knowledge + "/jobs?page_size=50"),
      get(SVC.memory + "/vectors?embedding_type=platform&page_size=50"),
    ]);
  } catch (e) { return viewError("Failed to load knowledge data", e.message); }

  const sources = (sourcesR.data && sourcesR.data.items) || [];
  const jobs = (jobsR.data && jobsR.data.items) || [];
  const vectors = (knowledgeR.data && knowledgeR.data.items) || [];

  return `
    <div class="grid g4" style="margin-bottom:18px">
      <div class="card metric"><b>${sources.length}</b><span>data sources</span></div>
      <div class="card metric"><b>${jobs.length}</b><span>ingestion jobs</span></div>
      <div class="card metric"><b>${vectors.length}</b><span>knowledge chunks</span></div>
      <div class="card metric"><b>${jobs.filter(j=>j.status==='completed').length}</b><span>completed jobs</span></div>
    </div>

    <div class="card" style="margin-bottom:18px">
      <h3>Data source configuration <span class="tag">Module 06</span></h3>
      <div class="hint">Connect institutional knowledge from SharePoint, Confluence, email, databases, and more.</div>
      <div class="frow" style="margin-bottom:14px">
        <select id="sourceType">
          ${DATA_SOURCES.map(s => `<option value="${esc(s.id)}">${esc(s.icon)} ${esc(s.name)}</option>`).join("")}
        </select>
        <input id="sourceUrl" placeholder="source URL / connection string">
        <button class="sm" onclick="window.addSource()">Connect</button>
      </div>
      ${sources.length === 0
        ? `<div class="empty">No data sources connected yet.</div>`
        : sources.map(s => rowItem({
            title: `${esc(s.icon || "📁")} ${esc(s.name || s.source_type)}`,
            meta: `${esc(s.endpoint || s.url || "—")} · ${esc(s.status || "inactive")} · ${rel(s.created_at)}`,
            badges: badge(s.status || "inactive"),
            actions: s.status === "active"
              ? `<button class="ghost sm" onclick="window.testSource('${esc(s.id)}')">Test</button>
                 <button class="sm bad" onclick="window.removeSource('${esc(s.id)}')">Remove</button>`
              : `<button class="ok sm" onclick="window.enableSource('${esc(s.id)}')">Enable</button>`,
          })).join("")}
    </div>

    <div class="grid g2">
      <div class="card">
        <h3>Ingestion jobs</h3>
        <div class="hint">Monitor ingestion progress: chunking, embedding, graph construction.</div>
        ${jobs.length === 0
          ? `<div class="empty">No jobs running.</div>`
          : jobs.map(j => rowItem({
              title: `🔄 ${esc(j.name || j.source_id?.slice(0,8) || "ingestion")}`,
              meta: `${esc(j.status)} · ${esc(j.chunks_processed||0)} chunks · ${esc(j.embedding_model||"—")}`,
              badges: badge(j.status),
            })).join("")}
      </div>
      <div class="card">
        <h3>Knowledge coverage</h3>
        <div class="hint">How much institutional knowledge has been ingested and indexed.</div>
        <div class="kv">
          <dt>Total chunks</dt><dd>${esc(String(vectors.length))}</dd>
          <dt>Departments covered</dt><dd>${esc(String(new Set(vectors.map(v=>v.metadata?.department_id)).size))}</dd>
          <dt>Embedding model</dt><dd>${esc(vectors[0]?.embedding_model || "—")}</dd>
          <dt>Duplicate rate</dt><dd>${esc("0.0")}%</dd>
        </div>
        <div style="margin-top:14px">
          <button class="sm" onclick="window.runDedup()">Run deduplication</button>
          <button class="ghost sm" style="margin-left:8px" onclick="window.refreshAllSources()">Refresh all sources</button>
        </div>
      </div>
    </div>`;
}

window.addSource = async function () {
  const type = $("sourceType").value;
  const url = $("sourceUrl").value.trim();
  if (!url) { toast("URL/connection string required", "bad"); return; }
  const source = DATA_SOURCES.find(s => s.id === type);
  try {
    const r = await post(SVC.knowledge + "/sources", {
      id: uuid4(), source_type: type, name: source?.name || type,
      endpoint: url, status: "inactive",
    });
    if (r.ok) { toast("Source " + esc(source?.name || type) + " added", "ok"); window.go("ingestion"); }
    else toast("Failed: " + esc(JSON.stringify(r.data).slice(0, 120)), "bad");
  } catch (e) { toast("Error: " + esc(String(e)), "bad"); }
};

window.removeSource = async function (id) {
  if (!confirm("Remove this source? Ingestion jobs will stop.")) return;
  const r = await del(SVC.knowledge + "/sources/" + id);
  if (r.ok) { toast("Source removed", "ok"); window.go("ingestion"); }
  else toast("Failed", "bad");
};

window.enableSource = async function (id) {
  const r = await patch(SVC.knowledge + "/sources/" + id, { status: "active" });
  if (r.ok) { toast("Source enabled", "ok"); window.go("ingestion"); }
  else toast("Failed", "bad");
};

window.testSource = async function (id) {
  toast("Testing connection...", "ok");
  try {
    const r = await post(SVC.knowledge + "/sources/" + id + "/test");
    if (r.ok) toast("Connection successful!", "ok");
    else toast("Connection failed: " + esc(JSON.stringify(r.data).slice(0, 120)), "bad");
  } catch (e) { toast("Test error: " + esc(String(e)), "bad"); }
};

window.runDedup = async function () {
  toast("Running deduplication...", "ok");
  try {
    const r = await post(SVC.knowledge + "/dedup");
    if (r.ok) toast("Deduplication complete — " + esc(String(r.data?.removed || 0)) + " duplicates removed", "ok");
    else toast("Failed: " + esc(JSON.stringify(r.data).slice(0, 120)), "bad");
  } catch (e) { toast("Error: " + esc(String(e)), "bad"); }
};

window.refreshAllSources = async function () {
  toast("Refreshing all sources...", "ok");
  try {
    const r = await post(SVC.knowledge + "/sources/refresh");
    if (r.ok) toast("Refresh triggered for all sources", "ok");
    else toast("Failed: " + esc(JSON.stringify(r.data).slice(0, 120)), "bad");
  } catch (e) { toast("Error: " + esc(String(e)), "bad"); }
};

function viewError(title, msg) {
  return `<div class="error-box"><div class="err-title">${esc(title)}</div><div class="err-msg">${esc(msg)}</div><button onclick="window.go('ingestion')">Retry</button></div>`;
}