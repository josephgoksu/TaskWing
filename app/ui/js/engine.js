// The bridge to the TaskWing engine.
//
// The engine speaks newline-delimited JSON: events out, commands in. Inside
// the desktop app the Rust shell runs `taskwing engine --json` and relays
// both ways. When no engine is installed (or the page runs in a browser),
// the built-in demo engine plays a believable run so the app can be tried.
//
// Events:   engine.hello, run.created, plan.ready, run.stage, card.update, question,
//           question.closed, note, exam.result, pr.opened, run.error, run.finished,
//           engine.exit
// Commands: run.start, plan.approve, plan.redo, answer, exam.now,
//           run.pause, run.resume

const tauri = window.__TAURI__;

export async function connect(onEvent) {
  if (tauri?.core?.invoke) {
    try {
      await tauri.event.listen("engine://event", (e) => onEvent(e.payload));
      await tauri.core.invoke("engine_start");
      return {
        mode: "live",
        send: (cmd) => tauri.core.invoke("engine_send", { cmd }).catch((err) => console.warn("engine_send", err)),
      };
    } catch (err) {
      console.info("No TaskWing engine found, using the demo engine.", err);
    }
  }
  return demoEngine(onEvent);
}

export function demoEngine(onEvent) {
  const demo = new DemoEngine(onEvent);
  return { mode: "demo", send: (cmd) => demo.handle(cmd) };
}

export function trayStatus(title, tooltip) {
  if (tauri?.core?.invoke) tauri.core.invoke("tray_status", { title, tooltip }).catch(() => {});
}

export function onTrayNew(fn) {
  if (tauri?.event?.listen) tauri.event.listen("tray://new", fn).catch(() => {});
}

// Opens an https link in the default browser. Returns false in a plain
// browser, where the link's own target="_blank" does the job.
export function openUrl(url) {
  if (!tauri?.core?.invoke) return false;
  tauri.core.invoke("open_url", { url }).catch((err) => console.warn("open_url", err));
  return true;
}

/* ------------------------------------------------------------------ */
/* Demo engine: plays a run with timers. Marked "Demo engine" in the UI. */

const PLANS = [
  {
    match: /discount|coupon|promo/i,
    cards: [
      ["Discount model and migration", "db/discounts/**", "test"],
      ["Create and check codes", "api/discounts/**", "request", "POST /discounts"],
      ["Apply a code to the order total", "pricing/**", "timeline"],
      ["Code field on the checkout page", "web/checkout/**", "page"],
      ["Admin list of codes", "web/admin/discounts/**", "page"],
    ],
    names: ["POST /discounts", "discount_cents"],
    checks: [
      "A valid code lowers the order total",
      "An expired code is refused with a clear message",
      "A code can't push the total below zero",
      "Admins see every code and how often it was used",
    ],
    ask: { card: 3, text: "Show the discount line above or below the tax line?", options: ["Above", "Below"] },
    note: [1, 2, "Amounts are in cents, never floats."],
  },
  {
    match: /health|status|ping/i,
    cards: [
      ["Health route", "internal/http/health/**", "request", "GET /health"],
      ["Its test", "internal/http/health_test/**", "test"],
    ],
    names: ["GET /health", "version"],
    checks: ["GET /health returns 200 with the version"],
    note: [0, 1, "The version comes from the build, not a constant."],
  },
  {
    match: /dark|theme|colou?r/i,
    cards: [
      ["Theme tokens", "src/styles/theme/**", "file"],
      ["Settings switch", "src/settings/**", "page"],
      ["Remember the choice", "src/store/prefs/**", "test"],
    ],
    names: ["theme", "prefers-color-scheme"],
    checks: [
      "The settings page has a dark mode switch",
      "The choice is kept after a restart",
      "With no choice, the app follows the system",
    ],
    ask: { card: 1, text: "Should dark mode follow the system by default?", options: ["Follow the system", "Start light"] },
    note: [0, 1, "Use the theme tokens, never raw colours."],
  },
];

const GENERIC = {
  cards: [
    ["Data and storage", "internal/store/**", "test"],
    ["API", "internal/api/**", "request", "POST /items"],
    ["Screen", "web/**", "page"],
  ],
  names: ["POST /items"],
  checks: ["The new behaviour works end to end", "Bad input gets a clear error"],
  note: [0, 1, "Keep the field names from the plan."],
};

const OLD_TESTS = "Every test that passes today still passes";

class DemoEngine {
  constructor(emit) {
    this.emit = (e) => setTimeout(() => emit(e), 0);
    this.runs = new Map();
    this.n = 0;
  }

  // One tick chain per run: a new schedule replaces the pending tick, so
  // pause/resume and answers never start a second chain.
  schedule(run, ms) {
    clearTimeout(run.tickTimer);
    run.tickTimer = setTimeout(() => { run.tickTimer = null; this.tick(run); }, ms);
  }

  later(ms, fn) {
    setTimeout(fn, ms);
  }

  handle(cmd) {
    const run = this.runs.get(cmd.run_id);
    switch (cmd.type) {
      case "run.start": return this.start(cmd);
      case "plan.approve": return run && run.stage === "approve" && this.approve(run, cmd.checks);
      case "plan.redo": return run && run.stage === "approve" && this.plan(run);
      case "answer": return run && this.answer(run, cmd.question_id, cmd.choice);
      case "exam.now": return run && run.stage === "build" && this.exam(run);
      case "run.pause": if (run && !run.paused && run.stage === "build") { run.paused = true; this.emit({ type: "run.stage", run_id: run.id, stage: run.stage, paused: true }); } return;
      case "run.resume": if (run && run.paused) { run.paused = false; this.emit({ type: "run.stage", run_id: run.id, stage: run.stage, paused: false }); this.schedule(run, 400); } return;
      default: return;
    }
  }

  start({ repo, task }) {
    const id = `r${Date.now().toString(36)}${(this.n++).toString(36)}`;
    const tpl = PLANS.find((p) => p.match.test(task)) || GENERIC;
    const run = { id, repo, task, tpl, stage: "planning", paused: false, cards: [], asked: false, started: Date.now() };
    this.runs.set(id, run);
    this.emit({ type: "run.created", run: { id, repo, task, created_at: new Date().toISOString() } });
    this.plan(run);
  }

  plan(run) {
    run.stage = "planning";
    this.emit({ type: "run.stage", run_id: run.id, stage: "planning" });
    this.later(1600, () => {
      const t = run.tpl;
      run.cards = t.cards.map(([title, owns, kind, text], i) => ({ id: `c${i + 1}`, title, owns, proof: { kind, text: text || "" }, state: "waiting", steps: 0 }));
      run.stage = "approve";
      this.emit({
        type: "plan.ready", run_id: run.id,
        cards: run.cards.map(({ id, title, owns, proof }) => ({ id, title, owns: [owns], proof: { kind: proof.kind } })),
        names: t.names,
        checks: [...t.checks, OLD_TESTS].map((text, i) => ({ id: `k${i + 1}`, text, added_by: "exam" })),
      });
      this.emit({ type: "run.stage", run_id: run.id, stage: "approve" });
    });
  }

  approve(run, checks) {
    run.checks = (checks || []).map((c, i) => ({ id: c.id || `u${i + 1}`, text: c.text }));
    run.stage = "build";
    run.buildStarted = Date.now();
    this.emit({ type: "run.stage", run_id: run.id, stage: "build" });
    // "Run the exam now" can come before every card has started.
    run.cards.forEach((c, i) => this.later(500 + i * 450, () => {
      if (run.stage === "build" && c.state === "waiting") this.card(run, c, "working", "Starting in a fresh sandbox");
    }));
    this.schedule(run, 2600);
  }

  card(run, c, state, detail, proof) {
    c.state = state;
    this.emit({ type: "card.update", run_id: run.id, card: { id: c.id, state, detail, proof: proof || null } });
  }

  tick(run) {
    if (run.stage !== "build" || run.paused) return;
    const t = run.tpl;
    const working = run.cards.filter((c) => c.state === "working");
    if (!working.length) {
      if (run.cards.every((c) => c.state === "passed")) this.later(900, () => this.exam(run));
      return;
    }
    const c = working[Math.floor(Math.random() * working.length)];
    c.steps++;
    const idx = run.cards.indexOf(c);
    if (t.ask && !run.asked && idx === t.ask.card && c.steps >= 1) {
      run.asked = true;
      this.card(run, c, "needs", "Waiting for your answer");
      this.emit({ type: "question", run_id: run.id, question: { id: "q1", card_id: c.id, text: t.ask.text, options: t.ask.options } });
    } else if (c.steps >= 3) {
      this.card(run, c, "passed", "Its own tests pass", proofFor(c));
    } else {
      const lines = ["Reading the code it owns", "Writing tests first", "Making the tests pass", "Committing its work"];
      this.card(run, c, "working", lines[Math.min(c.steps, lines.length - 1)], c.steps === 2 ? proofFor(c, true) : null);
    }
    if (!run.noted && t.note && run.cards.filter((x) => x.state !== "waiting").length > 1) {
      run.noted = true;
      const [from, to, text] = t.note;
      this.emit({ type: "note", run_id: run.id, note: { from: run.cards[from]?.id, to: run.cards[to]?.id, text, at: new Date().toISOString() } });
    }
    this.schedule(run, 1100 + Math.random() * 900);
  }

  answer(run, qid, choice) {
    if (qid !== "q1" || run.answered || run.stage !== "build") return;
    run.answered = choice;
    this.emit({ type: "question.closed", run_id: run.id, question_id: qid, choice });
    const c = run.cards[run.tpl.ask.card];
    this.card(run, c, "working", `Using your answer: ${choice}`);
    if (!run.paused) this.schedule(run, 1200);
  }

  exam(run) {
    if (run.stage !== "build") return;
    run.stage = "exam";
    for (const c of run.cards) if (c.state !== "passed") this.card(run, c, "done", "Stopped for the exam");
    this.emit({ type: "run.stage", run_id: run.id, stage: "exam" });
    this.later(2200, () => {
      const results = run.checks.map((k) => ({ id: k.id, result: "pass" }));
      // A run cut short with "Run the exam now" fails on the unfinished pieces.
      if (run.cards.some((c) => c.state !== "passed")) results[0].result = "fail";
      this.emit({ type: "exam.result", run_id: run.id, checks: results });
      if (results.some((r) => r.result === "fail")) {
        this.emit({ type: "run.finished", run_id: run.id, ok: false, secrets_found: 0, ended_at: new Date().toISOString() });
        run.stage = "failed";
        return;
      }
      run.stage = "pr";
      this.emit({ type: "run.stage", run_id: run.id, stage: "pr" });
      this.later(1400, () => {
        const number = 40 + Math.floor(Math.random() * 60);
        // The demo has no real pull request, so it sends no link.
        this.emit({ type: "pr.opened", run_id: run.id, number, url: null, draft: true });
        this.emit({ type: "run.finished", run_id: run.id, ok: true, secrets_found: 0, ended_at: new Date().toISOString() });
        run.stage = "done";
      });
    });
  }
}

function proofFor(c, partial) {
  const k = c.proof.kind;
  if (k === "request") return { kind: k, text: c.proof.text, status: partial ? 404 : c.proof.text.startsWith("POST") ? 201 : 200 };
  if (k === "test") return { kind: k, text: partial ? "2 of 5 pass" : "tests pass" };
  return { kind: k, text: partial ? "in progress" : "ready" };
}
