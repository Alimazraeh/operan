// People — the humans in the organisation, and which seats they hold.
//
// This is the bridge between Module 02 (who exists) and Module 05 (the org
// chart). Binding a person to a seat is what gives them scope and approval
// authority: user → position → department → authority. An unbound person can
// sign in and raise requests, and nothing else — which is the honest state,
// not a broken one.
import { esc, rel, toast, statCard, badge } from "../ui.js";
import {
  unwrapList, listIamUsers, createIamUser, setUserPassword,
  listDepartments, getDeptOrgChart, setPositionHolder, session,
} from "../api.js";
import { can, authz } from "../perm.js";

export default async function viewPeople() {
  const [usersR, deptR] = await Promise.allSettled([listIamUsers(), listDepartments(1, 50)]);
  const users = usersR.status === "fulfilled" ? unwrapList(usersR.value, "users") : [];
  const depts = (deptR.status === "fulfilled" ? unwrapList(deptR.value) : [])
    .filter(d => d.status === "operational" || d.status === "degraded");

  const charts = (await Promise.allSettled(depts.map(d => getDeptOrgChart(d.id))))
    .map((r, i) => ({
      dept: depts[i],
      positions: r.status === "fulfilled" && r.value.ok ? (r.value.data?.positions || []) : [],
    }));

  // Which seats each person holds, and which seats nobody holds.
  const seatsByUser = {};
  const humanSeats = [];
  const vacantSeats = [];
  for (const { dept, positions } of charts) {
    for (const p of positions) {
      if (p.holder_type === "human" && p.human_ref) {
        (seatsByUser[p.human_ref] = seatsByUser[p.human_ref] || []).push({ dept, pos: p });
        humanSeats.push({ dept, pos: p });
      } else {
        // Every seat no person holds — vacant or agent-held. Both are
        // bindable: assigning a human to an agent-held seat is how a
        // department gets a human head.
        vacantSeats.push({ dept, pos: p });
      }
    }
  }
  const unbound = users.filter(u => !(seatsByUser[u.id] || []).length);

  window._people = { depts, charts, users };
  const editable = can("org.write");

  return `<div id="peopleRoot">
    <div class="stats-grid">
      ${statCard("🧑‍💼", "People", users.length, users.length ? "Platform users" : "No users yet")}
      ${statCard("🪑", "Seats held by people", humanSeats.length, humanSeats.length ? "Bound to the org chart" : "Nobody holds a seat yet")}
      ${statCard("◌", "Unbound people", unbound.length, unbound.length ? "Can sign in, hold no authority" : "Everyone holds a seat")}
      ${statCard("🏢", "Live departments", depts.length, "Operational or degraded")}
    </div>

    <div class="card" style="margin-bottom:18px">
      <h3>People <span class="tag">Module 02</span></h3>
      <div class="hint">A person's scope and approval authority come from the seats they hold, not
      from their account. Somebody with no seat can sign in and raise requests — that is the honest
      state of a new joiner, not a fault.${editable ? "" : " You do not have permission to change bindings."}</div>
      ${editable ? `
        <div class="toolbar">
          <div class="search"><input id="peopleSearch" placeholder="Filter people…" oninput="window.filterPeople(this.value)"></div>
          <button class="primary" onclick="window.openAddPersonModal()">+ Add person</button>
        </div>` : ""}
      ${users.length === 0
        ? `<div class="empty">No platform users yet.${editable ? " Add one above." : ""}</div>`
        : users.map(u => personRow(u, seatsByUser[u.id] || [], editable)).join("")}
    </div>

    <div class="card">
      <h3>Seats no person holds <span class="tag">${vacantSeats.length}</span></h3>
      <div class="hint">Vacant and agent-held seats. Assigning a person here is what makes them a
      department head or an approver — and it is what routes that seat's gates to them.</div>
      ${vacantSeats.length === 0
        ? `<div class="empty">Every seat has a holder.</div>`
        : vacantSeats.slice(0, 20).map(({ dept, pos }) => `
          <div class="row-item" data-name="${esc((pos.title + " " + dept.name).toLowerCase())}">
            <div class="grow">
              <div class="t">🪑 ${esc(pos.title)} <span class="tag">${esc(dept.name)}</span>
                ${pos.reports_to ? "" : `<span class="wfstep agent" style="font-size:10px;padding:1px 7px">department root</span>`}
                ${(pos.approval_gate_refs || []).length ? `<span class="wfstep human_gate" style="font-size:10px;padding:1px 7px">🚧 holds gate</span>` : ""}</div>
              <div class="m">${esc(pos.role_type || "seat")}${pos.autonomy_tier ? " · " + esc(pos.autonomy_tier) : ""}
                ${(pos.decision_rights || []).length ? " · decides: " + esc((pos.decision_rights || []).map(d => d.decision).join(", ").slice(0, 80)) : ""}</div>
            </div>
            <div class="actions">${badge(pos.holder_type || "vacant")}${editable
              ? `<button class="sm" onclick="window.openBindModal('${esc(dept.id)}','${esc(pos.id)}')">Assign a person</button>`
              : ""}</div>
          </div>`).join("")}
    </div>
  </div>`;
}

function personRow(u, seats, editable) {
  const isMe = u.id === session.userId;
  const seatText = seats.length
    ? seats.map(s => `${esc(s.pos.title)} · ${esc(s.dept.name)}`).join(" · ")
    : "no seat — can sign in and raise requests, no authority";
  return `<div class="row-item" data-name="${esc(((u.display_name || "") + " " + (u.email || "")).toLowerCase())}">
    <div class="grow">
      <div class="t">🧑 ${esc(u.display_name || u.email)}${isMe ? ` <span class="tag">you</span>` : ""}
        ${(u.role_ids || []).map(r => `<span class="tag">${esc(String(r).replace(/_/g, " "))}</span>`).join("")}</div>
      <div class="m">${esc(u.email || "")} · ${seats.length ? "" : "◌ "}${seatText}${u.created_at ? " · joined " + rel(u.created_at) : ""}</div>
    </div>
    <div class="actions">
      <span class="badge ${esc(u.status || "active")}">${esc(u.status || "active")}</span>
      ${editable ? `<button class="ghost sm" onclick="window.openSetPasswordModal('${esc(u.id)}','${esc(u.display_name || u.email)}')">Set password</button>` : ""}
    </div>
  </div>`;
}

window.filterPeople = function (q) {
  const needle = (q || "").toLowerCase();
  document.querySelectorAll("#peopleRoot [data-name]").forEach(row => {
    row.style.display = (row.dataset.name || "").includes(needle) ? "" : "none";
  });
};

// ── Bind a person into a seat ───────────────────────────────
window.openBindModal = function (deptId, positionId) {
  const st = window._people || {};
  const chart = (st.charts || []).find(c => c.dept.id === deptId);
  const pos = (chart?.positions || []).find(p => p.id === positionId);
  const users = st.users || [];
  const modal = document.createElement("div");
  modal.className = "modal-overlay show";
  modal.innerHTML = `
    <div class="modal">
      <div class="modal-header"><h3>Assign a person to a seat</h3>
        <button class="ghost sm" onclick="this.closest('.modal-overlay').remove()">✕</button></div>
      <div class="modal-body">
        <div class="kv">
          <dt>Seat</dt><dd>${esc(pos?.title || positionId)}</dd>
          <dt>Department</dt><dd>${esc(chart?.dept.name || deptId)}</dd>
          <dt>Autonomy</dt><dd>${esc(pos?.autonomy_tier || "—")}</dd>
          <dt>Decides</dt><dd>${esc((pos?.decision_rights || []).map(d => d.decision).join("; ") || "—")}</dd>
          <dt>Holds gates</dt><dd>${esc((pos?.approval_gate_refs || []).join(", ") || "—")}</dd>
        </div>
        <div class="form-group" style="margin-top:12px"><label>Person</label>
          <select id="bindUser">
            <option value="">— choose —</option>
            ${users.map(u => `<option value="${esc(u.id)}">${esc(u.display_name || u.email)} — ${esc(u.email || "")}</option>`).join("")}
          </select>
          <div class="hint">Whoever you bind inherits this seat's authority: its decision rights and
          any gate it holds. The binding is verified against the identity service.</div>
        </div>
      </div>
      <div class="modal-footer">
        <button class="ghost" onclick="this.closest('.modal-overlay').remove()">Cancel</button>
        <button class="primary" onclick="window.doBindSeat('${esc(deptId)}','${esc(positionId)}')">Assign</button>
      </div>
    </div>`;
  document.body.appendChild(modal);
};

window.doBindSeat = async function (deptId, positionId) {
  const userId = document.getElementById("bindUser").value;
  if (!userId) { toast("Choose a person", "warn"); return; }
  const r = await setPositionHolder(deptId, positionId, { holder_type: "human", human_ref: userId });
  if (r.ok) {
    toast("Seat assigned — their authority now comes from it", "ok");
    document.querySelector(".modal-overlay")?.remove();
    window.go("people");
  } else {
    toast("Assignment failed: " + esc(r.data?.detail || r.data?.error?.message || r.status), "bad");
  }
};

// ── Add a person ────────────────────────────────────────────
window.openAddPersonModal = function () {
  const modal = document.createElement("div");
  modal.className = "modal-overlay show";
  modal.innerHTML = `
    <div class="modal">
      <div class="modal-header"><h3>Add a person</h3>
        <button class="ghost sm" onclick="this.closest('.modal-overlay').remove()">✕</button></div>
      <div class="modal-body">
        <div class="form-group"><label>Email</label><input id="npEmail" type="email" placeholder="name@company.com"></div>
        <div class="form-group"><label>Full name</label><input id="npName" placeholder="Dana Quinn"></div>
        <div class="form-group"><label>Role</label>
          <select id="npRole">
            <option value="employee">Employee — raise requests</option>
            <option value="supervisor">Supervisor — decide gates</option>
            <option value="department_head">Department head — run a department</option>
            <option value="executive">Executive — read across departments</option>
            <option value="platform_admin">Platform admin — everything</option>
          </select>
          <div class="hint">Scope still comes from the seats they hold. A role without a seat grants
          read of nothing in particular.</div>
        </div>
        <div class="form-group"><label>Initial password</label>
          <input id="npPassword" type="password" placeholder="at least 12 characters, mixing letters with digits or symbols">
          <div class="hint">Set by you and shared with them out of band — there is no self-service
          reset yet.</div>
        </div>
      </div>
      <div class="modal-footer">
        <button class="ghost" onclick="this.closest('.modal-overlay').remove()">Cancel</button>
        <button class="primary" onclick="window.doAddPerson()">Add</button>
      </div>
    </div>`;
  document.body.appendChild(modal);
};

window.doAddPerson = async function () {
  const email = document.getElementById("npEmail").value.trim();
  const name = document.getElementById("npName").value.trim();
  const role = document.getElementById("npRole").value;
  const password = document.getElementById("npPassword").value;
  if (!email || !name) { toast("Email and name are required", "warn"); return; }

  const created = await createIamUser(email, name, [role]);
  if (!created.ok) {
    toast("Could not add them: " + esc(created.data?.error || created.status), "bad");
    return;
  }
  const userId = created.data?.id;
  if (password) {
    const pw = await setUserPassword(userId, password);
    if (!pw.ok) {
      // The account exists; say so plainly rather than implying it failed.
      toast("Added, but the password was refused: " + esc(pw.data?.error || pw.status), "warn");
      document.querySelector(".modal-overlay")?.remove();
      window.go("people");
      return;
    }
  }
  toast(esc(name) + (password ? " can now sign in" : " added — set a password to let them sign in"), "ok");
  document.querySelector(".modal-overlay")?.remove();
  window.go("people");
};

// ── Set a password ──────────────────────────────────────────
window.openSetPasswordModal = function (userId, who) {
  const modal = document.createElement("div");
  modal.className = "modal-overlay show";
  modal.innerHTML = `
    <div class="modal">
      <div class="modal-header"><h3>Set a password for ${esc(who)}</h3>
        <button class="ghost sm" onclick="this.closest('.modal-overlay').remove()">✕</button></div>
      <div class="modal-body">
        <div class="form-group"><label>New password</label>
          <input id="spPassword" type="password" placeholder="at least 12 characters">
          <div class="hint">Share it out of band. Their existing sessions keep working until the
          token expires.</div>
        </div>
      </div>
      <div class="modal-footer">
        <button class="ghost" onclick="this.closest('.modal-overlay').remove()">Cancel</button>
        <button class="primary" onclick="window.doSetPassword('${esc(userId)}')">Set password</button>
      </div>
    </div>`;
  document.body.appendChild(modal);
};

window.doSetPassword = async function (userId) {
  const password = document.getElementById("spPassword").value;
  if (!password) { toast("Enter a password", "warn"); return; }
  const r = await setUserPassword(userId, password);
  if (r.ok) {
    toast("Password set", "ok");
    document.querySelector(".modal-overlay")?.remove();
  } else {
    toast("Refused: " + esc(r.data?.error || r.status), "bad");
  }
};
