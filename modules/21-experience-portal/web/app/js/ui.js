// UI helpers for the real portal
// Accepts both a bare element id ("view") and a "#id" selector ("#btnLaunch") —
// app.js uses the selector form, the views use bare ids.
export const $ = id => document.getElementById(id && id[0] === "#" ? id.slice(1) : id);

// Inline onclick handlers in view-generated HTML run in global scope; give
// them the two helpers they reach for.
window.$ = $;

export function esc(s) {
  return String(s ?? "").replace(/[&<>"']/g, c =>
    ({"&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;"}[c]));
}

// ── Badge ──────────────────────────────────────────────────
export function badge(text, color = "") {
  const c = color ? ` style="background:${color}20;color:${color}"` : "";
  return `<span class="badge"${c}>${esc(text)}</span>`;
}

// ── Relative time ──────────────────────────────────────────
export function rel(ts) {
  if (!ts) return "—";
  const s = Math.max(0, (Date.now() - new Date(ts).getTime()) / 1000);
  if (s < 60) return Math.floor(s) + "s ago";
  if (s < 3600) return Math.floor(s / 60) + "m ago";
  if (s < 86400) return Math.floor(s / 3600) + "h ago";
  if (s < 604800) return Math.floor(s / 86400) + "d ago";
  return new Date(ts).toLocaleDateString();
}

// ── Format numbers ─────────────────────────────────────────
export function fmt(n) {
  if (n == null) return "—";
  if (typeof n === "string") return n;
  if (n >= 1e6) return (n / 1e6).toFixed(1) + "M";
  if (n >= 1e3) return (n / 1e3).toFixed(1) + "K";
  return n.toLocaleString();
}

// ── Toast notification ─────────────────────────────────────
export function toast(msg, kind = "info") {
  const el = document.createElement("div");
  el.className = "toast " + kind;
  el.innerHTML = msg;
  const container = $("toasts");
  if (!container) return;
  container.appendChild(el);
  setTimeout(() => { el.style.opacity = "0"; setTimeout(() => el.remove(), 300); }, 4000);
}

// ── Card with header ───────────────────────────────────────
export function card(title, subtitle, body) {
  return `<div class="card">
    ${title ? `<div class="card-header"><h2>${esc(title)}</h2>${subtitle ? `<small>${esc(subtitle)}</small>` : ""}</div>` : ""}
    <div class="card-body">${body}</div>
  </div>`;
}

// ── Table helper ───────────────────────────────────────────
export function table(headers, rows) {
  const th = headers.map(h => `<th>${esc(h)}</th>`).join("");
  const tr = rows.map(r => {
    const td = r.map(c => {
      if (typeof c === "object") return `<td>${esc(c.label || "")}</td>`;
      return `<td>${esc(String(c))}</td>`;
    }).join("");
    return `<tr>${td}</tr>`;
  }).join("");
  return `<div class="table-wrap"><table><thead><tr>${th}</tr></thead><tbody>${tr}</tbody></table></div>`;
}

// ── Stat card ──────────────────────────────────────────────
export function statCard(icon, label, value, sub, color = "") {
  const c = color || "var(--cyan)";
  return `<div class="stat-card">
    <div class="stat-icon" style="color:${c}">${esc(icon)}</div>
    <div class="stat-content">
      <div class="stat-value">${esc(String(value))}</div>
      <div class="stat-label">${esc(label)}</div>
      ${sub ? `<div class="stat-sub">${esc(sub)}</div>` : ""}
    </div>
  </div>`;
}

// ── Empty state ────────────────────────────────────────────
export function emptyState(icon, title, message, action) {
  return `<div class="empty-state">
    <div class="empty-icon">${esc(icon)}</div>
    <h3>${esc(title)}</h3>
    <p>${esc(message)}</p>
    ${action ? `<div class="empty-actions">${action}</div>` : ""}
  </div>`;
}

// ── Button helper ──────────────────────────────────────────
export function btn(text, cls = "", onclick = "") {
  const o = onclick ? ` onclick="${onclick}"` : "";
  return `<button class="${esc(cls)}"${o}>${esc(text)}</button>`;
}

// ── Pagination ─────────────────────────────────────────────
export function pagination(page, total, pageSize) {
  if (pageSize <= 1) return "";
  const totalPages = Math.ceil(total / pageSize) || 1;
  if (totalPages <= 1) return "";
  let html = `<div class="pagination"><button class="ghost sm" ${page <= 1 ? 'disabled' : ''} onclick="window.go(currentView, ${page - 1})">← Prev</button>`;
  for (let i = 1; i <= Math.min(totalPages, 5); i++) {
    html += `<span class="page-btn ${i === page ? 'active' : ''}" onclick="window.go(currentView, ${i})">${i}</span>`;
  }
  html += `<button class="ghost sm" ${page >= totalPages ? 'disabled' : ''} onclick="window.go(currentView, ${page + 1})">Next →</button></div>`;
  return html;
}

// ── Spinner ────────────────────────────────────────────────
export function spinner(size = "24px") {
  return `<div class="spinner" style="width:${size};height:${size};border-width:2px"></div>`;
}

// ── Row item card (used by old views) ──────────────────────
export function rowItem({ title, meta, badges, icon, onClick }) {
  const badgeHtml = badges ? (Array.isArray(badges) ? badges.map(b => {
    if (typeof b === 'string') return `<span class="badge ${b}">${esc(b)}</span>`;
    const c = b.color || 'info';
    const l = b.label || b;
    return `<span class="badge ${c}">${esc(l)}</span>`;
  }).join("") : badge(badges)) : "";
  return `<div class="row-item"${onClick ? ` onclick="${onClick}"` : ""}>
    ${icon ? `<div class="stat-icon">${esc(icon)}</div>` : ""}
    <div class="grow">
      <div class="t">${esc(title)}</div>
      ${meta ? `<div class="m">${esc(meta)}</div>` : ""}
      ${badgeHtml ? `<div style="margin-top:6px">${badgeHtml}</div>` : ""}
    </div>
  </div>`;
}

// ── Event row ──────────────────────────────────────────────
export function eventRow({ span, service, traceId, latency, status, ts }) {
  return `<div class="row-item">
    <div class="grow">
      <div class="t">${esc(span || "—")}</div>
      <div class="m">${esc(service || "—")} · ${esc(latency || "—")}${traceId ? ` · trace ${esc(traceId.slice(0,8))}` : ""} · ${esc(ts || "—")}</div>
    </div>
    <span class="badge ${status || ''}">${esc(status || "—")}</span>
  </div>`;
}

// ── KV table helper ────────────────────────────────────────
export function kvHtml(entries) {
  return entries.map(([k, v]) => {
    const vHtml = typeof v === 'function' ? v() : esc(String(v));
    return `<dt>${esc(k)}</dt><dd>${vHtml}</dd>`;
  }).join("");
}
window.toast = toast;
