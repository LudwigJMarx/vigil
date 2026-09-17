// The operator UI. Plain modules, no build step and no dependency: the page is
// served out of the same binary as the API, and a UI that needs a toolchain to
// change is a UI nobody changes.

const STORAGE_KEY = "vigil.token";

const el = (id) => document.getElementById(id);
const state = { token: localStorage.getItem(STORAGE_KEY) || "", view: "accounts" };

async function api(path, options = {}) {
  const response = await fetch(path, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${state.token}`,
      ...(options.headers || {}),
    },
  });
  if (response.status === 401) {
    localStorage.removeItem(STORAGE_KEY);
    state.token = "";
    showUnlock("That token was refused.");
    throw new Error("unauthorised");
  }
  if (response.status === 204) return null;
  const body = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(body.error || `HTTP ${response.status}`);
  return body;
}

function text(node, value) {
  node.textContent = value === undefined || value === null ? "" : String(value);
  return node;
}

function cell(row, value, className) {
  const td = row.insertCell();
  if (className) td.className = className;
  text(td, value);
  return td;
}

const dateFormat = new Intl.DateTimeFormat(undefined, { dateStyle: "medium" });
const stampFormat = new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" });
const when = (iso, formatter = dateFormat) =>
  !iso || iso.startsWith("0001-01-01") ? "—" : formatter.format(new Date(iso));

function showUnlock(message) {
  el("app").hidden = true;
  el("unlock").hidden = false;
  const error = el("unlock-error");
  error.hidden = !message;
  text(error, message || "");
}

// ── accounts ───────────────────────────────────────────────────────────────

async function renderAccounts() {
  const data = await api("/api/v1/accounts");
  const body = el("accounts-body");
  body.replaceChildren();

  for (const account of data.accounts) {
    const row = body.insertRow();
    row.tabIndex = 0;
    const name = cell(row, account.name);
    if (account.profile) {
      name.append(" ");
      const link = document.createElement("a");
      link.href = `https://${account.profile}`;
      link.rel = "noopener noreferrer";
      link.target = "_blank";
      link.className = "muted";
      link.textContent = "↗";
      link.addEventListener("click", (event) => event.stopPropagation());
      name.append(link);
    }
    const stage = row.insertCell();
    const pill = document.createElement("span");
    pill.className = "pill";
    text(pill, account.stage);
    stage.append(pill);
    cell(row, account.top_reason || "—");
    cell(row, account.signals_14d, "num");
    const score = cell(row, account.score.toFixed(1), "num score");
    if (account.score >= 25) score.classList.add("hot");
    row.addEventListener("click", () => openAccount(account.id));
    row.addEventListener("keydown", (event) => {
      if (event.key === "Enter") openAccount(account.id);
    });
  }

  // Say what was read, not just what came out. An empty table because nothing
  // was captured and an empty table because every kind is unknown to the
  // scoring model are different problems.
  const scope = [
    `${data.accounts.length} account(s), ${data.signals_read} signal(s) read`,
    `scored ${when(data.scored_at, stampFormat)}`,
  ];
  if (data.unscored_kinds.length) {
    scope.push(`no rule for: ${data.unscored_kinds.join(", ")}`);
  }
  text(el("accounts-scope"), scope.join(" · "));
}

async function openAccount(id) {
  const detail = await api(`/api/v1/accounts/${encodeURIComponent(id)}`);
  const view = el("view-account");
  view.replaceChildren();
  view.hidden = false;
  el("view-accounts").hidden = true;

  const back = document.createElement("button");
  back.className = "action";
  back.textContent = "← All accounts";
  back.addEventListener("click", () => switchView("accounts"));
  view.append(back);

  const head = document.createElement("div");
  head.className = "card";
  const title = document.createElement("h2");
  text(title, detail.account.name);
  title.style.margin = "0 0 .25rem";
  const sub = document.createElement("p");
  sub.className = "muted";
  const parts = [
    `${detail.account.stage}`,
    `score ${detail.score.total.toFixed(1)}`,
    `${detail.signals_total} signal(s), showing ${detail.signals_returned}`,
  ];
  if (detail.score.unscored?.length) parts.push(`no rule for: ${detail.score.unscored.join(", ")}`);
  text(sub, parts.join(" · "));
  head.append(title, sub);

  if (detail.score.contributions.length) {
    const why = document.createElement("p");
    const top = detail.score.contributions.slice(0, 3)
      .map((c) => `${c.kind} ${c.points.toFixed(1)} (${Math.round(c.age_days)} d old)`)
      .join(", ");
    text(why, `Made of: ${top}`);
    head.append(why);
  }
  view.append(head);

  const noteCard = document.createElement("div");
  noteCard.className = "card row";
  const noteInput = document.createElement("input");
  noteInput.className = "grow";
  noteInput.placeholder = "What did you learn? (stored on the timeline, scores nothing)";
  const noteButton = document.createElement("button");
  noteButton.className = "action";
  noteButton.textContent = "Add note";
  noteButton.addEventListener("click", async () => {
    if (!noteInput.value.trim()) return;
    await api(`/api/v1/accounts/${encodeURIComponent(id)}/note`, {
      method: "POST",
      body: JSON.stringify({ body: noteInput.value.trim() }),
    });
    openAccount(id);
  });
  noteCard.append(noteInput, noteButton);
  view.append(noteCard);

  if (detail.people.length) {
    const card = document.createElement("div");
    card.className = "card";
    const heading = document.createElement("strong");
    text(heading, "People");
    card.append(heading);
    const list = document.createElement("ul");
    for (const person of detail.people) {
      const item = document.createElement("li");
      text(item, `${person.name}${person.headline ? " — " + person.headline : ""}`);
      list.append(item);
    }
    card.append(list);
    view.append(card);
  }

  const timeline = document.createElement("ol");
  timeline.className = "timeline";
  for (const signal of detail.signals) {
    const item = document.createElement("li");
    const stamp = document.createElement("time");
    stamp.dateTime = signal.occurred_at;
    text(stamp, `${when(signal.occurred_at, stampFormat)} · ${signal.kind} · ${signal.source}`);
    item.append(stamp);
    if (signal.title) {
      const strong = document.createElement("div");
      text(strong, signal.title);
      item.append(strong);
    }
    if (signal.body) {
      const para = document.createElement("div");
      text(para, signal.body);
      item.append(para);
    }
    if (signal.url) {
      const link = document.createElement("a");
      link.href = signal.url;
      link.rel = "noopener noreferrer";
      link.target = "_blank";
      text(link, "source");
      item.append(link);
    }
    timeline.append(item);
  }
  view.append(timeline);
}

// ── rules ──────────────────────────────────────────────────────────────────

async function renderRules() {
  const data = await api("/api/v1/rules");
  const body = el("rules-body");
  body.replaceChildren();

  for (const rule of data.rules) {
    const row = body.insertRow();
    cell(row, rule.kind);

    const weight = document.createElement("input");
    weight.type = "number";
    weight.step = "0.5";
    weight.value = rule.weight;
    row.insertCell().append(weight);

    const halfLife = document.createElement("input");
    halfLife.type = "number";
    halfLife.step = "1";
    halfLife.min = "0";
    halfLife.value = rule.half_life_days;
    row.insertCell().append(halfLife);

    const enabled = document.createElement("input");
    enabled.type = "checkbox";
    enabled.checked = rule.enabled;
    enabled.style.width = "auto";
    row.insertCell().append(enabled);

    cell(row, rule.note || "—", "muted");

    const save = document.createElement("button");
    save.className = "action";
    save.textContent = "Save";
    save.addEventListener("click", async () => {
      await api(`/api/v1/rules/${encodeURIComponent(rule.kind)}`, {
        method: "PUT",
        body: JSON.stringify({
          kind: rule.kind,
          weight: Number(weight.value),
          half_life_days: Number(halfLife.value),
          enabled: enabled.checked,
          note: rule.note,
        }),
      });
      renderRules();
    });
    row.insertCell().append(save);
  }
}

// ── tokens ─────────────────────────────────────────────────────────────────

async function renderTokens() {
  const data = await api("/api/v1/tokens");
  const body = el("tokens-body");
  body.replaceChildren();

  for (const token of data.tokens) {
    const row = body.insertRow();
    cell(row, token.name);
    cell(row, when(token.created_at));
    cell(row, when(token.last_used_at, stampFormat));
    cell(row, token.revoked_at ? "revoked" : "active");
    const actions = row.insertCell();
    if (!token.revoked_at) {
      const revoke = document.createElement("button");
      revoke.className = "action";
      revoke.textContent = "Revoke";
      revoke.addEventListener("click", async () => {
        await api(`/api/v1/tokens/${encodeURIComponent(token.id)}`, { method: "DELETE" });
        renderTokens();
      });
      actions.append(revoke);
    }
  }
}

// ── plumbing ───────────────────────────────────────────────────────────────

const views = {
  accounts: renderAccounts,
  rules: renderRules,
  tokens: renderTokens,
};

function switchView(name) {
  state.view = name;
  el("view-account").hidden = true;
  for (const key of Object.keys(views)) {
    el(`view-${key}`).hidden = key !== name;
  }
  for (const button of document.querySelectorAll("nav button")) {
    button.setAttribute("aria-current", String(button.dataset.view === name));
  }
  views[name]().catch((error) => text(el("status"), error.message));
}

async function start() {
  const health = await fetch("/healthz").then((r) => r.json()).catch(() => null);
  if (health) {
    text(el("status"),
      `${health.version} · schema ${health.schema_version} · ` +
      `${health.accounts} accounts · ${health.signals} signals · ${health.active_tokens} tokens`);
  } else {
    text(el("status"), "server unreachable");
  }

  if (!state.token) {
    showUnlock(health && health.active_tokens === 0
      ? "No token exists yet. Run: vigil token create --name browser"
      : "");
    return;
  }
  el("unlock").hidden = true;
  el("app").hidden = false;
  switchView(state.view);
}

el("token-save").addEventListener("click", () => {
  const value = el("token-input").value.trim();
  if (!value) return;
  localStorage.setItem(STORAGE_KEY, value);
  state.token = value;
  el("token-input").value = "";
  start();
});

el("new-account-save").addEventListener("click", async () => {
  const name = el("new-account-name").value.trim();
  if (!name) return;
  try {
    await api("/api/v1/accounts", {
      method: "POST",
      body: JSON.stringify({
        name,
        profile: el("new-account-profile").value.trim(),
        stage: el("new-account-stage").value,
      }),
    });
    el("new-account-name").value = "";
    el("new-account-profile").value = "";
    renderAccounts();
  } catch (error) {
    text(el("status"), error.message);
  }
});

el("new-token-save").addEventListener("click", async () => {
  const name = el("new-token-name").value.trim() || "unnamed";
  const created = await api("/api/v1/tokens", {
    method: "POST",
    body: JSON.stringify({ name }),
  });
  el("new-token-name").value = "";
  text(el("new-token-value"), created.secret);
  el("new-token-secret").hidden = false;
  renderTokens();
});

for (const button of document.querySelectorAll("nav button")) {
  button.addEventListener("click", () => switchView(button.dataset.view));
}

start();
