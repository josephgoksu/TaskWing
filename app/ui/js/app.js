// TaskWing desktop app: state, reducer and views.
// The app keeps no run logic of its own: it draws what the engine says and
// sends the user's choices back as commands.

import { connect, demoEngine, trayStatus, onTrayNew, openUrl } from "./engine.js";
import { icon, logo } from "./icons.js";

/* ---------------- state ---------------- */

const state = {
  engine: "connecting", // connecting | live | demo | stopped
  can: null, // optional commands the live engine handles (engine.hello); null = all
  runs: new Map(),
  order: [], // newest first
  view: { name: "home", runId: null },
  draft: loadDraft(),
};
// Until connect() settles, commands wait here instead of being dropped.
let engine = { queued: [], send(cmd) { this.queued.push(cmd); } };

function loadDraft() {
  const fallback = { repo: "your-org/shop", task: "", sandbox: "boat", models: "Claude" };
  try {
    const saved = JSON.parse(localStorage.getItem("tw.draft") || "null");
    return saved && typeof saved === "object" ? { ...fallback, ...pick(saved, Object.keys(fallback)) } : fallback;
  } catch {
    return fallback;
  }
}
function saveDraft() {
  try { localStorage.setItem("tw.draft", JSON.stringify(state.draft)); } catch { /* private window */ }
}
function pick(o, keys) {
  const out = {};
  for (const k of keys) if (typeof o[k] === "string") out[k] = o[k].slice(0, 2000);
  return out;
}

/* ---------------- reducer: engine events ---------------- */

const STAGES = ["planning", "approve", "build", "exam", "pr", "done"];
const CARD_STATES = new Set(["waiting", "working", "passed", "done", "needs", "stuck"]);
const str = (v, max = 500) => (typeof v === "string" ? v.slice(0, max) : "");
// The objects in an engine array, at most `max` of them.
const objs = (v, max) => (Array.isArray(v) ? v : []).filter((x) => x && typeof x === "object").slice(0, max);

function apply(e) {
  if (!e || typeof e.type !== "string") return;
  if (e.type === "engine.exit") {
    // An engine that exits before its first event doesn't speak this
    // protocol (an older taskwing with no `engine` command): use the demo.
    // The shell says whether the engine ever spoke; that survives a reload of
    // this page, which resets `heard`.
    const silent = typeof e.spoke === "boolean" ? !e.spoke : !heard;
    if (silent && state.engine !== "demo") useDemo();
    else state.engine = "stopped";
    return paint();
  }
  if (e.type === "engine.hello") {
    state.can = Array.isArray(e.can) ? e.can.filter((x) => typeof x === "string").slice(0, 50) : null;
    return paint();
  }
  if (e.type === "run.created") {
    const r = e.run || {};
    const id = str(r.id, 80);
    if (!id || state.runs.has(id)) return;
    state.runs.set(id, {
      id, repo: str(r.repo, 200), task: str(r.task, 2000), n: state.runs.size + 1,
      stage: "planning", paused: false, createdAt: Date.parse(r.created_at) || Date.now(),
      cards: [], checks: [], names: [], questions: [], notes: [], pr: null,
      endedAt: null, ok: null, secrets: null, buildAt: null, error: "",
    });
    state.order.unshift(id);
    if (pendingOpen) { pendingOpen = false; state.view = { name: "run", runId: id }; }
    return paint();
  }
  const run = state.runs.get(e.run_id);
  if (!run) return;
  switch (e.type) {
    case "plan.ready":
      run.cards = objs(e.cards, 100).map((c) => ({
        id: str(c.id, 40), title: str(c.title, 200),
        owns: (Array.isArray(c.owns) ? c.owns : []).map((o) => str(o, 200)).slice(0, 20),
        kind: str(c.proof?.kind, 20), state: "waiting", detail: "", proof: null,
      }));
      run.checks = objs(e.checks, 50).map((k) => ({ id: str(k.id, 40), text: str(k.text, 300), by: k.added_by === "user" ? "user" : "exam", result: null }));
      run.names = (Array.isArray(e.names) ? e.names : []).map((n) => str(n, 80)).slice(0, 20);
      break;
    case "run.stage":
      if (STAGES.includes(e.stage)) {
        run.stage = e.stage;
        if (e.stage === "build" && !run.buildAt) run.buildAt = Date.now();
      }
      if (typeof e.paused === "boolean") run.paused = e.paused;
      break;
    case "card.update": {
      const c = run.cards.find((x) => x.id === e.card?.id);
      if (!c) return;
      if (CARD_STATES.has(e.card.state)) c.state = e.card.state;
      if (typeof e.card.detail === "string") c.detail = str(e.card.detail, 200);
      if (e.card.proof && typeof e.card.proof === "object") {
        c.proof = { kind: str(e.card.proof.kind, 20), text: str(e.card.proof.text, 60), status: Number.isInteger(e.card.proof.status) ? e.card.proof.status : null };
      }
      break;
    }
    case "question": {
      const q = e.question || {};
      if (typeof q.id !== "string" || !q.id || run.questions.some((x) => x.id === str(q.id, 40))) return;
      run.questions.push({ id: str(q.id, 40), cardId: str(q.card_id, 40), text: str(q.text, 400), options: (Array.isArray(q.options) ? q.options : []).map((o) => str(o, 60)).slice(0, 4), open: true });
      break;
    }
    case "question.closed": {
      const q = run.questions.find((x) => x.id === e.question_id);
      if (q) { q.open = false; q.choice = str(e.choice, 60); q.sending = false; }
      break;
    }
    case "note":
      run.notes.unshift({ from: str(e.note?.from, 40), to: str(e.note?.to, 40), text: str(e.note?.text, 300), at: Date.parse(e.note?.at) || Date.now() });
      run.notes.length = Math.min(run.notes.length, 50);
      break;
    case "exam.result":
      for (const r of objs(e.checks, 200)) {
        const k = run.checks.find((x) => x.id === r.id);
        if (k && (r.result === "pass" || r.result === "fail")) k.result = r.result;
      }
      break;
    case "pr.opened": {
      const url = str(e.url, 400);
      run.pr = { number: Number.isInteger(e.number) ? e.number : null, url: /^https:\/\//.test(url) ? url : null, draft: e.draft !== false };
      break;
    }
    case "run.error":
      // The run stopped on an error (no repository, a sandbox that would not
      // start...), not on the exam. run.finished follows.
      run.error = str(e.message, 400) || "The engine stopped this run.";
      run.failed = true;
      break;
    case "run.finished":
      run.endedAt = Date.parse(e.ended_at) || Date.now();
      run.ok = e.ok === true;
      run.secrets = Number.isInteger(e.secrets_found) ? e.secrets_found : null;
      run.stage = run.ok ? "done" : run.stage;
      if (!run.ok) run.failed = true;
      break;
    default:
      return;
  }
  paint(run.id);
}

/* ---------------- helpers ---------------- */

const esc = (s) => String(s ?? "").replace(/[&<>"']/g, (ch) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[ch]);
const plural = (n, one, many) => `${n} ${n === 1 ? one : many}`;

function duration(ms) {
  const s = Math.max(0, Math.round(ms / 1000));
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ${String(s % 60).padStart(2, "0")}s`;
  return `${Math.floor(m / 60)}h ${String(m % 60).padStart(2, "0")}m`;
}
const ago = (t) => {
  const s = Math.round((Date.now() - t) / 1000);
  if (s < 45) return "just now";
  if (s < 3600) return `${Math.round(s / 60)} min ago`;
  return `${Math.round(s / 3600)} h ago`;
};
const isActive = (run) => !run.failed && run.stage !== "done";
// A finished run can't be answered any more, so its questions don't count.
const openQuestions = (run) => (isActive(run) ? run.questions.filter((q) => q.open) : []);
const cardName = (run, id) => run.cards.find((c) => c.id === id)?.title || "An agent";

function minis(run) {
  if (!run.cards.length) return `<span class="minis" aria-hidden="true"><i class="sq waiting"></i><i class="sq waiting"></i><i class="sq waiting"></i></span>`;
  return `<span class="minis" aria-hidden="true">${run.cards.map((c) => `<i class="sq ${c.state}"></i>`).join("")}</span>`;
}

function runMeta(run) {
  const q = openQuestions(run).length;
  if (q) return `<span class="m needs">${plural(q, "question", "questions")} for you</span>`;
  if (run.error) return `<span class="m">Stopped · ${esc(run.repo)}</span>`;
  if (run.failed) return `<span class="m">Exam failed · ${esc(run.repo)}</span>`;
  if (run.stage === "done") return `<span class="m">${run.pr?.number ? `Draft PR #${run.pr.number}` : "Finished"} · ${esc(run.repo)}</span>`;
  const label = { planning: "Planning", approve: "Waiting for your OK", build: run.paused ? "Paused" : "Building", exam: "Exam running", pr: "Opening the PR" }[run.stage];
  return `<span class="m">${label} · ${esc(run.repo)}</span>`;
}

/* ---------------- views ---------------- */

function sidebar() {
  const runs = state.order.map((id) => state.runs.get(id));
  const active = runs.filter(isActive);
  const finished = runs.filter((r) => !isActive(r));
  const item = (r) => `
    <button class="runlink${state.view.runId === r.id ? " on" : ""}" data-open="${esc(r.id)}" aria-current="${state.view.runId === r.id ? "page" : "false"}">
      ${minis(r)}<span class="t">${esc(r.task)}</span>${runMeta(r)}
    </button>`;
  const status = {
    connecting: `<span class="st st-line">Starting engine</span>`,
    live: `<span class="st st-pass"><span class="dot"></span>Engine on</span>`,
    demo: `<span class="st st-demo" title="No TaskWing engine was found, or it is too old, so runs are simulated."><span class="dot"></span>Demo engine</span>`,
    stopped: `<span class="st st-stuck"><span class="dot"></span>Engine stopped</span>`,
  }[state.engine];
  return `
    <div class="drag" data-tauri-drag-region></div>
    <div class="brand">${logo("nav")}<b>TaskWing</b></div>
    <button class="btn btn-ink btn-wide" data-act="new">${icon("plus")}New task</button>
    <nav class="side-list" aria-label="Runs">
      <p class="lab">Running</p>
      ${active.length ? active.map(item).join("") : `<p class="side-empty">Nothing running.</p>`}
      ${finished.length ? `<p class="lab">Finished</p>${finished.map(item).join("")}` : ""}
    </nav>
    <div class="side-foot">${status}<span class="mono muted">v0.1</span></div>`;
}

const EXAMPLES = [
  ["your-org/shop", "Add discount codes at checkout, with tests"],
  ["your-org/blog", "Add a health endpoint with a test"],
  ["your-org/todo-app", "Add dark mode to settings"],
];

function home() {
  const d = state.draft;
  return `
  <section class="home">
    <span class="home-logo">${logo("home")}</span>
    <span class="pill">Open source · AGPL-3.0</span>
    <h1 class="display">Give TaskWing <span class="accent">one task.</span></h1>
    <p class="lede">A team of agents works on it in parallel, each in its own sandbox. You approve the exam first, and you get back one tested pull request.</p>
    <form class="beam" id="composer" autocomplete="off">
      <div class="composer">
        <div class="bar">
          <label class="chipin">@<span class="sr-only">Repository</span><input name="repo" value="${esc(d.repo)}" spellcheck="false" required maxlength="200"></label>
        </div>
        <label class="sr-only" for="task">Task</label>
        <textarea id="task" name="task" rows="2" maxlength="2000" placeholder="Describe one task, for example: add discount codes at checkout, with tests">${esc(d.task)}</textarea>
        <div class="bar">
          <label class="chipin">Sandbox<select name="sandbox">${[["boat", "boat"], ["Local folders", "Local folders (no sandbox)"]].map(([v, label]) => `<option value="${v}"${v === d.sandbox ? " selected" : ""}>${label}</option>`).join("")}</select></label>
          <label class="chipin">Models<select name="models">${["Claude", "Mixed", "Open models"].map((o) => `<option${o === d.models ? " selected" : ""}>${o}</option>`).join("")}</select></label>
          <button class="btn btn-ink send" type="submit" aria-label="Plan this task">${icon("up")}</button>
        </div>
      </div>
    </form>
    <div class="examples" aria-label="Examples">
      ${EXAMPLES.map(([repo, task], i) => `<button class="ex" data-example="${i}">${esc(task)}</button>`).join("")}
    </div>
    <p class="hint"><kbd>⌘ Enter</kbd> plans the task · nothing runs until you approve the exam</p>
  </section>`;
}

function stages(run) {
  const order = ["planning", "build", "exam", "pr"];
  const names = { planning: "Plan", build: "Build", exam: "Exam", pr: "Pull request" };
  const cur = run.stage === "approve" ? "planning" : run.stage === "done" ? "done" : run.stage;
  const at = cur === "done" ? order.length : order.indexOf(cur);
  return `<div class="stages" aria-label="Stages">${order.map((s, i) => {
    let cls = "", lead = "";
    if (run.error && i === Math.min(at, order.length - 1)) { cls = "bad"; lead = icon("x"); }
    else if (run.failed && !run.error && s === "exam") { cls = "bad"; lead = icon("x"); }
    else if (i < at) { cls = "ok"; lead = icon("check"); }
    else if (i === at) {
      cls = "now"; lead = `<span class="dot"></span>`;
    } else if (s === "exam") lead = icon("lock");
    let label = names[s];
    if (s === "build" && i === at) {
      const done = run.cards.filter((c) => c.state === "passed").length;
      label = `Build · ${done} of ${run.cards.length} done${run.paused ? " · paused" : ""}`;
    }
    if (s === "planning" && run.stage === "approve") label = "Plan · your OK";
    return `${i ? `<span class="link${i <= at ? " ok" : ""}"></span>` : ""}<span class="stage ${cls}">${lead}${label}</span>`;
  }).join("")}</div>`;
}

function runHead(run, actions = "") {
  return `
    <div class="runhead">
      <div class="t">
        <span class="st st-line">@ ${esc(run.repo)} · run ${run.n}</span>
        <h1 class="h1 selectable">${esc(run.task)}</h1>
      </div>
      <div class="actions">${actions}</div>
    </div>
    ${stages(run)}`;
}

function planning(run) {
  return `
    ${runHead(run)}
    <div class="card planning">
      <span class="grid" aria-hidden="true"><i></i><i></i><i></i><i></i><i class="l"></i><i></i><i></i><i></i><i></i></span>
      <div class="stack" style="gap:4px">
        <b style="font-weight:600;font-size:16px">Your lead agent is reading the repository</b>
        <span class="muted">It splits the task into pieces that touch different files. A separate session writes the exam.</span>
      </div>
    </div>`;
}

function approve(run) {
  return `
    ${runHead(run)}
    <div class="grid2">
      <section class="card stack" aria-label="The plan">
        <div class="head"><p class="lab">The lead's plan · ${plural(run.cards.length, "piece", "pieces")}</p><span class="st st-pass">No shared files</span></div>
        ${run.cards.map((c) => `<div class="piece"><b>${esc(c.title)}</b>${c.owns.map((o) => `<span class="own">owns ${esc(o)}</span>`).join("")}</div>`).join("")}
        ${run.names.length ? `<p class="muted" style="font-size:12.5px">Pinned names: ${run.names.map((n) => `<span class="own">${esc(n)}</span>`).join(", ")}</p>` : ""}
      </section>
      <section class="card-soft stack" aria-label="The exam">
        <div class="head"><p class="lab">The exam · plain sentences</p><span class="st" style="background:#fff;color:var(--blue-600)">${icon("lock")}Tests stay hidden</span></div>
        ${run.checks.map((k) => `
          <div class="chk"><span class="box">${icon("check")}</span><span class="selectable">${esc(k.text)}</span>
            ${k.by === "user" ? `<button class="x" data-rmcheck="${esc(k.id)}" aria-label="Remove this check">${icon("x")}</button>` : ""}
          </div>`).join("")}
        <form class="addchk" id="addcheck" autocomplete="off">
          <label class="sr-only" for="newcheck">Add a check</label>
          <input id="newcheck" maxlength="300" placeholder="Add a check in your own words">
          <button class="btn btn-ghost" type="submit">Add</button>
        </form>
        <div class="actions" style="margin-top:4px">
          <button class="btn btn-ink btn-lg" style="flex:1" data-act="approve">Approve and start agents</button>
          <button class="btn btn-ghost btn-lg" data-act="redo">Plan again</button>
        </div>
      </section>
    </div>`;
}

function proof(c) {
  const p = c.proof;
  if (!p) return `<span class="proof empty">no proof yet</span>`;
  if (p.kind === "request") return `<span class="proof">${esc(p.text)} <b style="${p.status >= 400 ? "color:var(--stuck-ink)" : ""}">${p.status ?? ""}</b></span>`;
  if (p.kind === "timeline") return `<span class="proof"><span class="bars" aria-hidden="true"><i style="height:8px"></i><i style="height:14px"></i><i style="height:20px;background:var(--blue-500)"></i></span>${esc(p.text)}</span>`;
  if (p.kind === "page") return `<span class="proof"><span class="page" aria-hidden="true"><i style="width:70%"></i><i></i></span></span>`;
  if (p.kind === "test" && /pass$/.test(p.text)) return `<span class="proof">${icon("check", "color:var(--pass-ink)")}${esc(p.text)}</span>`;
  return `<span class="proof">${esc(p.text)}</span>`;
}

const STATE_PILL = {
  waiting: `<span class="st st-line">Waiting</span>`,
  working: `<span class="st st-work">Working</span>`,
  passed: `<span class="st st-pass">Done</span>`,
  done: `<span class="st">Done</span>`,
  needs: `<span class="st st-needs">Needs you</span>`,
  stuck: `<span class="st st-stuck">Stuck</span>`,
};

function building(run) {
  const count = (s) => run.cards.filter((c) => c.state === s).length;
  const q = openQuestions(run)[0];
  const examLocked = run.stage === "build";
  const actions = run.stage === "build"
    ? `${can("run.pause") ? `<button class="btn btn-ghost" data-act="${run.paused ? "resume" : "pause"}">${run.paused ? "Resume" : "Pause"}</button>` : ""}
       ${can("exam.now") ? `<button class="btn btn-ink" data-act="examnow">Run the exam now</button>` : ""}`
    : "";
  const note = run.notes[0];
  return `
    ${runHead(run, actions)}
    <div class="grid3">
      <section class="card stack span2 rspan2" aria-label="Agents">
        <div class="head"><p class="lab">Agents · each in its own sandbox</p>
          <span class="legend">
            ${count("working") ? `<span><i class="sq working"></i>${count("working")} working</span>` : ""}
            ${count("passed") ? `<span><i class="sq passed"></i>${count("passed")} done</span>` : ""}
            ${count("needs") ? `<span><i class="sq needs"></i>${count("needs")} needs you</span>` : ""}
            ${count("waiting") ? `<span><i class="sq waiting"></i>${count("waiting")} waiting</span>` : ""}
          </span>
        </div>
        ${run.cards.map((c) => `
          <div class="arow ${c.state === "needs" ? "needs" : c.state === "stuck" ? "stuck" : ""}">
            <i class="sq ${c.state}" aria-hidden="true"></i>
            <div class="who"><b>${esc(c.title)}</b><span>${esc(c.detail || "Waiting for a sandbox")}</span></div>
            ${proof(c)}
            ${STATE_PILL[c.state] || ""}
          </div>`).join("")}
      </section>

      ${q ? `
      <section class="card-needs stack" aria-label="Needs you">
        <span class="st" style="background:var(--needs);color:#fff;align-self:flex-start">Needs you</span>
        <p class="q selectable">${esc(q.text)}</p>
        <p class="from">Asked by: ${esc(cardName(run, q.cardId))}</p>
        <div class="choices">${q.options.map((o, i) => `<button class="btn ${i ? "btn-ghost" : "btn-ink"}" data-answer="${esc(q.id)}" data-choice="${esc(o)}"${q.sending ? " disabled" : ""}>${esc(o)}</button>`).join("")}</div>
      </section>` : `
      <section class="card card-band" aria-label="Questions">
        <div class="calm">${icon("bell", "width:22px;height:22px;color:var(--muted)")}<b>No questions for you</b><span class="muted">When an agent needs a decision, it shows up here and in the menu bar.</span></div>
      </section>`}

      <section class="card-dark stack" style="gap:10px" aria-label="Exam">
        <div class="head"><p class="lab">Exam · ${plural(run.checks.length, "check", "checks")}</p><span class="st" style="background:rgba(255,255,255,.1);color:#d7dbe3">${examLocked ? "Locked" : run.stage === "exam" ? "Running" : "Graded"}</span></div>
        ${run.checks.slice(0, 3).map((k) => `<div class="chk">${icon("lock", "color:#b4bac6")}<span>${esc(k.text)}</span></div>`).join("")}
        <p class="sub">${run.checks.length > 3 ? `+${run.checks.length - 3} more · ` : ""}no agent can see the tests</p>
      </section>
    </div>
    ${note ? `
    <section class="card notes" aria-label="Notes between agents">
      <p class="lab">Notes</p>
      <span class="st">${esc(cardName(run, note.from))}</span>${icon("arrow")}<span class="st">${esc(cardName(run, note.to))}</span>
      <span class="txt selectable">${esc(note.text)}</span><span class="mono muted">${ago(note.at)}</span>
    </section>` : ""}`;
}

// Whether the engine handles an optional command. The demo handles them all.
const can = (cmd) => state.engine !== "live" || !state.can || state.can.includes(cmd);

function stoppedView(run) {
  return `
    ${runHead(run)}
    <section class="card-needs pr" aria-label="Why the run stopped">
      <div class="who">${icon("x", "color:var(--needs-ink)")}<div><b>The run stopped</b><span class="selectable">${esc(run.error)}</span></div></div>
      <div class="actions"><button class="btn btn-ink" data-act="again">Run this task again</button></div>
    </section>`;
}

function receipt(run) {
  const passed = run.checks.filter((k) => k.result === "pass").length;
  const total = run.checks.length;
  const time = (run.endedAt || Date.now()) - run.createdAt;
  const pr = run.pr;
  return `
    ${runHead(run)}
    <div class="rb">
      <section class="card-dark rb-main" aria-label="Exam result">
        <div>
          <span class="lab">Hidden exam</span>
          <p class="num" style="font-size:96px;margin-top:20px;color:#fff">${passed}<small>/ ${total}</small></p>
          <div class="checksq" style="margin-top:16px" role="img" aria-label="${passed} of ${total} checks passed">${run.checks.map((k) => `<i class="${k.result === "pass" ? "" : "f"}"></i>`).join("")}</div>
        </div>
        <div class="stack" style="gap:8px">
          ${[...run.checks].sort((a, b) => (a.result === "pass") - (b.result === "pass")).slice(0, 3).map((k) => `<p style="font-size:14px;color:${k.result === "pass" ? "#d7dbe3" : "#ffc56b"}">${k.result === "fail" ? "Failed: " : k.result === "pass" ? "" : "Not graded: "}${esc(k.text)}</p>`).join("")}
          <p class="sub">Graded in a clean sandbox no agent could see.</p>
        </div>
      </section>
      <section class="card card-band rb-a" aria-label="Time">
        <div><p class="lab">Time</p><p class="num" style="font-size:44px;margin-top:8px">${duration(time)}</p><p class="muted" style="margin-top:6px;font-size:13px">Task to ${run.failed ? "exam result" : "draft pull request"}</p></div>
      </section>
      <section class="card rb-b" aria-label="Agents"><p class="num" style="font-size:40px">${run.cards.length}</p><p class="lab">Agents</p></section>
      <section class="card card-band rb-c" aria-label="Secrets">
        <span class="ic">${icon("shield")}</span>
        <div><p style="font-weight:600;font-size:16px">${run.secrets === null ? "Secret scan not reported" : `${plural(run.secrets, "secret", "secrets")} in boxes`}</p><p class="muted" style="margin-top:4px;font-size:13px">Every sandbox is scanned before it is deleted</p></div>
      </section>
    </div>
    ${run.failed ? `
    <section class="card-needs pr" aria-label="Next step">
      <div class="who">${icon("x", "color:var(--needs-ink)")}<div><b>No pull request</b><span>The exam failed, so nothing was pushed. Plan the task again or give the agents more time.</span></div></div>
      <div class="actions"><button class="btn btn-ink" data-act="again">Run this task again</button></div>
    </section>` : `
    <section class="card-blue pr" aria-label="Pull request">
      <div class="who">${icon("pr")}<div><b>${pr?.number ? `Draft pull request #${pr.number}` : "Draft pull request"}</b><span>With the proof and the exam results. You merge.</span></div></div>
      <div class="actions">
        ${pr?.url ? `<a class="btn btn-white" href="${esc(pr.url)}" target="_blank" rel="noopener" data-external>Open on GitHub</a>` : ""}
        <button class="btn btn-glass" data-act="again">New run of this task</button>
      </div>
    </section>`}`;
}

function main() {
  if (state.view.name !== "run") return home();
  const run = state.runs.get(state.view.runId);
  if (!run) return home();
  if (run.error) return stoppedView(run);
  if (run.failed || run.stage === "done") return receipt(run);
  if (run.stage === "planning") return planning(run);
  if (run.stage === "approve") return approve(run);
  return building(run);
}

/* ---------------- painting ---------------- */

const $side = document.getElementById("side");
const $main = document.getElementById("main-in");
let lastMainKey = "", lastMainHtml = "", lastSideHtml = "";

// A selector that finds the same control again after a repaint.
const FOCUS_KEYS = ["act", "open", "answer", "rmcheck", "example"];
function focusSelector(el) {
  if (el.id) return `#${CSS.escape(el.id)}`;
  const k = FOCUS_KEYS.find((a) => el.dataset?.[a] !== undefined);
  if (!k) return null;
  let sel = `[data-${k}="${CSS.escape(el.dataset[k])}"]`;
  if (el.dataset.choice !== undefined) sel += `[data-choice="${CSS.escape(el.dataset.choice)}"]`;
  return sel;
}

// A repaint while a button is held down would swap the button out and
// swallow the click, so paints wait for the release (for up to a second).
let pressedAt = 0, deferred = false;
document.addEventListener("pointerdown", (ev) => { if (ev.target.closest?.("button, a")) pressedAt = performance.now(); }, true);
function release() {
  pressedAt = 0;
  if (deferred) { deferred = false; setTimeout(() => paint(), 0); }
}
document.addEventListener("pointerup", release, true);
document.addEventListener("pointercancel", release, true);
window.addEventListener("blur", release);

// Repaint the sidebar always; repaint the main pane only when it shows the
// run that changed (or the view changed). Unchanged markup is left alone, and
// the focused control and typed text survive a repaint.
function paint(changedRunId) {
  if (pressedAt && performance.now() - pressedAt < 1000) { deferred = true; return; }
  const active = document.activeElement;
  const keep = active && active !== document.body
    ? { sel: focusSelector(active), value: active.id ? active.value : undefined, start: active.selectionStart, end: active.selectionEnd }
    : null;
  const side = sidebar();
  if (side !== lastSideHtml) { $side.innerHTML = side; lastSideHtml = side; }
  const key = `${state.view.name}:${state.view.runId}`;
  const otherRun = changedRunId && key === lastMainKey && state.view.runId !== changedRunId;
  const sameHome = key === lastMainKey && state.view.name === "home";
  if (!otherRun && !sameHome) {
    const html = main();
    if (key !== lastMainKey || html !== lastMainHtml) {
      $main.innerHTML = html;
      lastMainHtml = html;
      if (key !== lastMainKey) document.getElementById("main").scrollTop = 0;
    }
    lastMainKey = key;
  }
  if (keep?.sel && (!document.activeElement || document.activeElement === document.body)) {
    const el = document.querySelector(keep.sel);
    if (el) {
      if (typeof keep.value === "string") el.value = keep.value;
      el.focus({ preventScroll: true });
      try { el.setSelectionRange(keep.start, keep.end); } catch { /* not a text field */ }
    }
  }
  tray();
  if (!otherRun && !sameHome) clock();
}

let lastTray = "";
function tray() {
  const runs = [...state.runs.values()];
  const needs = runs.reduce((n, r) => n + openQuestions(r).length, 0);
  const running = runs.filter(isActive).length;
  const parts = [];
  if (running) parts.push(`${running} running`);
  if (needs) parts.push(`${needs} need${needs === 1 ? "s" : ""} you`);
  const title = needs ? String(needs) : "";
  const tip = parts.length ? `TaskWing · ${parts.join(" · ")}` : "TaskWing";
  if (title + "|" + tip === lastTray) return;
  lastTray = title + "|" + tip;
  trayStatus(title, tip);
  document.title = needs ? `TaskWing (${needs})` : "TaskWing";
}

// One timer, only while a running run is on screen and the window is visible.
let clockTimer = null;
function clock() {
  const run = state.view.name === "run" && state.runs.get(state.view.runId);
  const want = run && isActive(run) && run.notes.length && !document.hidden;
  if (want && !clockTimer) clockTimer = setInterval(() => paint(state.view.runId), 30000);
  if (!want && clockTimer) { clearInterval(clockTimer); clockTimer = null; }
}
document.addEventListener("visibilitychange", clock);

/* ---------------- actions ---------------- */

let pendingOpen = false;

function open(view) {
  state.view = view;
  paint();
}

function startTask(repo, task) {
  repo = repo.trim(); task = task.trim();
  // A stopped engine takes no commands: keep the typed task instead of losing it.
  if (!repo || !task || state.engine === "stopped") return false;
  pendingOpen = true;
  engine.send({ type: "run.start", repo, task, sandbox: state.draft.sandbox, models: state.draft.models });
  state.draft.task = "";
  saveDraft();
  return true;
}

document.addEventListener("click", (ev) => {
  const t = ev.target.closest("button, a");
  if (!t) return;
  if (t.hasAttribute("data-external")) {
    // The app window can't open new windows: hand the link to the browser.
    if (openUrl(t.getAttribute("href"))) ev.preventDefault();
    return;
  }
  const run = state.runs.get(state.view.runId);
  if (t.dataset.open) return open({ name: "run", runId: t.dataset.open });
  if (t.dataset.example) {
    const [repo, task] = EXAMPLES[Number(t.dataset.example)] || [];
    if (!task) return;
    state.draft.repo = repo; state.draft.task = task; saveDraft();
    const form = document.getElementById("composer");
    if (form) { form.repo.value = repo; form.task.value = task; form.task.focus(); }
    return;
  }
  if (t.dataset.answer && run) {
    const q = run.questions.find((x) => x.id === t.dataset.answer);
    if (!q || !q.open || q.sending) return;
    q.sending = true;
    engine.send({ type: "answer", run_id: run.id, question_id: q.id, choice: t.dataset.choice });
    return paint(run.id);
  }
  if (t.dataset.rmcheck && run) {
    run.checks = run.checks.filter((k) => !(k.by === "user" && k.id === t.dataset.rmcheck));
    return paint(run.id);
  }
  switch (t.dataset.act) {
    case "new": return open({ name: "home", runId: null });
    case "approve":
      if (run?.stage === "approve") { t.disabled = true; engine.send({ type: "plan.approve", run_id: run.id, checks: run.checks.map(({ id, text, by }) => ({ id, text, added_by: by })) }); }
      return;
    case "redo": if (run) engine.send({ type: "plan.redo", run_id: run.id }); return;
    case "pause": if (run) engine.send({ type: "run.pause", run_id: run.id }); return;
    case "resume": if (run) engine.send({ type: "run.resume", run_id: run.id }); return;
    case "examnow": if (run) { t.disabled = true; engine.send({ type: "exam.now", run_id: run.id }); } return;
    case "again":
      if (run) { state.draft.repo = run.repo; state.draft.task = run.task; saveDraft(); open({ name: "home", runId: null }); }
      return;
    default: return;
  }
});

document.addEventListener("submit", (ev) => {
  ev.preventDefault();
  const f = ev.target;
  if (f.id === "composer") {
    if (!startTask(f.repo.value, f.task.value)) f.task.focus();
    return;
  }
  if (f.id === "addcheck") {
    const run = state.runs.get(state.view.runId);
    const input = f.querySelector("input");
    const text = input.value.trim().slice(0, 300);
    if (!run || run.stage !== "approve" || !text) return;
    run.checks.push({ id: `u${Date.now().toString(36)}`, text, by: "user", result: null });
    input.value = "";
    paint(run.id);
    document.getElementById("newcheck")?.focus();
  }
});

document.addEventListener("input", (ev) => {
  const f = ev.target.form;
  if (f?.id !== "composer") return;
  state.draft = { repo: f.repo.value, task: f.task.value, sandbox: f.sandbox.value, models: f.models.value };
  saveDraft();
});
document.addEventListener("change", (ev) => {
  const f = ev.target.form;
  if (f?.id === "composer") { state.draft.sandbox = f.sandbox.value; state.draft.models = f.models.value; saveDraft(); }
});

document.addEventListener("keydown", (ev) => {
  const mod = ev.metaKey || ev.ctrlKey;
  if (mod && ev.key === "Enter" && ev.target.form?.id === "composer") {
    ev.preventDefault();
    ev.target.form.requestSubmit();
  } else if (mod && (ev.key === "n" || ev.key === "N") && !ev.shiftKey && !ev.altKey) {
    ev.preventDefault();
    open({ name: "home", runId: null });
    document.getElementById("task")?.focus();
  }
});

onTrayNew(() => { open({ name: "home", runId: null }); document.getElementById("task")?.focus(); });

/* ---------------- start ---------------- */

// True once the live engine has sent a protocol event.
let heard = false;
// Commands the live engine got before it spoke. If it exits without a word,
// the demo engine replays them, so a task started in that gap isn't lost.
let unheard = [];

function useDemo() {
  const queued = [...(engine.queued || []), ...unheard];
  unheard = [];
  engine = demoEngine(apply);
  state.engine = "demo";
  for (const cmd of queued) engine.send(cmd);
}

paint();
connect((e) => {
  if (e && e.type !== "engine.exit" && !heard) { heard = true; unheard = []; }
  apply(e);
}).then((e) => {
  // The engine already exited and the demo took over.
  if (state.engine === "demo" && e.mode === "live") return paint();
  const queued = engine.queued || [];
  engine = e.mode === "live" ? { send: (cmd) => { if (!heard) unheard.push(cmd); e.send(cmd); } } : e;
  // An engine.exit that arrived first must not be overwritten with "live".
  if (state.engine === "connecting") state.engine = e.mode;
  for (const cmd of queued) engine.send(cmd);
  paint();
});
