// Knowledge Ingestion Pipeline (Module 06).
import { SVC, get, post, del, unwrapList } from "../api.js";
import { $, esc, badge, rel, toast, rowItem } from "../ui.js";

// The source types Module 06 actually accepts (DB-enforced enum).
const DATA_SOURCES = [
  { id: "url", name: "URL", icon: "🔗", description: "Fetch and ingest a single web page or document URL" },
  { id: "web_crawl", name: "Web crawl", icon: "🕸️", description: "Crawl a site from a starting URL" },
  { id: "file", name: "File", icon: "📄", description: "A file reachable by path or URL (PDF, DOCX, TXT, CSV)" },
  { id: "sharepoint", name: "SharePoint", icon: "📁", description: "Microsoft SharePoint site or library URL" },
  { id: "email", name: "Email", icon: "📧", description: "Mailbox ingestion endpoint" },
  { id: "s3", name: "S3 bucket", icon: "🪣", description: "S3/MinIO bucket or object URL" },
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

  const sources = unwrapList(sourcesR, "sources");
  const jobs = unwrapList(jobsR, "jobs");
  const vectors = unwrapList(knowledgeR, "items");

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
            meta: `${esc(s.source_url || s.endpoint || "—")} · ${rel(s.created_at)}`,
            badges: badge(s.status || "inactive"),
            actions: `<button class="sm" onclick="window.ingestSource('${esc(s.id)}')">Ingest now</button>
                 <button class="sm bad" onclick="window.removeSource('${esc(s.id)}')">Remove</button>`,
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
        </div>
        <div class="hint" style="margin-top:10px">Chunk-level dedup is automatic: previously seen content
        hashes are skipped across jobs, so re-ingesting unchanged sources adds nothing.</div>
      </div>
    </div>`;
}

window.addSource = async function () {
  const type = $("sourceType").value;
  const url = $("sourceUrl").value.trim();
  if (!url) { toast("URL/connection string required", "bad"); return; }
  const source = DATA_SOURCES.find(s => s.id === type);
  try {
    let ext = ((url.match(/\.(pdf|txt|html?|docx|md|csv)(\?|$)/i) || [])[1] || "html").toLowerCase();
    const fileType = ext === "htm" ? "html" : ext;
    const r = await post(SVC.knowledge + "/sources", {
      name: source?.name || type, source_type: type, source_url: url, file_type: fileType,
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

// The real pipeline trigger: source → job → chunks → platform vectors (M07).
window.ingestSource = async function (id) {
  const r = await post(SVC.knowledge + "/ingest", { source_id: id });
  if (r.ok) { toast("Ingestion job started — chunks land in the knowledge base", "ok"); setTimeout(() => window.go("ingestion"), 1200); }
  else toast("Ingest failed: " + esc(r.data?.error?.message || r.data?.message || JSON.stringify(r.data || {}).slice(0, 120)), "bad");
};

function viewError(title, msg) {
  return `<div class="error-box"><div class="err-title">${esc(title)}</div><div class="err-msg">${esc(msg)}</div><button onclick="window.go('ingestion')">Retry</button></div>`;
}