// The popup is the only place the extension reads a page, and it reads it only
// because the user just clicked the icon. There is no service worker, no alarm
// and no host permission: without that click the extension cannot see LinkedIn
// at all.

import { SIGNAL_KINDS, draftFrom, toRFC3339, type Draft } from "./guess.ts";
import { describe, send, VigilError } from "./client.ts";
import { loadSettings } from "./settings.ts";
import type { IngestRequest, PageFacts } from "./types.ts";

const el = <T extends HTMLElement>(id: string) => document.getElementById(id) as T;

/**
 * readPage runs inside the tab. It is injected as a function rather than a
 * content script file so it only ever exists while the popup is open, and it
 * reads three things that LinkedIn's markup changes cannot break: the address,
 * the document title and whatever the user had selected.
 */
function readPage(): PageFacts {
  return {
    url: location.href,
    title: document.title,
    selection: String(window.getSelection() ?? ""),
  };
}

function setStatus(message: string, bad = false) {
  const node = el<HTMLParagraphElement>("status");
  node.textContent = message;
  node.classList.toggle("bad", bad);
  node.hidden = false;
}

function showResult(message: string, bad = false) {
  const node = el<HTMLParagraphElement>("result");
  node.textContent = message;
  node.classList.toggle("bad", bad);
  node.hidden = false;
}

/** localInputValue renders a Date for <input type="datetime-local">, which
 *  wants local wall-clock time with no zone marker. */
function localInputValue(date: Date): string {
  const pad = (n: number) => String(n).padStart(2, "0");
  return (
    `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}` +
    `T${pad(date.getHours())}:${pad(date.getMinutes())}`
  );
}

function fillForm(draft: Draft) {
  el<HTMLInputElement>("account-name").value = draft.accountName;
  el<HTMLInputElement>("account-profile").value = draft.accountProfile;
  el<HTMLInputElement>("person-name").value = draft.personName;
  el<HTMLInputElement>("title").value = draft.title;
  el<HTMLTextAreaElement>("body").value = draft.body;
  el<HTMLInputElement>("occurred-at").value = localInputValue(draft.occurredAt);

  const select = el<HTMLSelectElement>("kind");
  select.replaceChildren();
  for (const kind of SIGNAL_KINDS) {
    const option = document.createElement("option");
    option.value = kind;
    option.textContent = kind;
    option.selected = kind === draft.kind;
    select.append(option);
  }
}

async function main() {
  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
  if (!tab?.id || !tab.url) {
    setStatus("No page to read here.", true);
    return;
  }

  let facts: PageFacts;
  try {
    const results = await chrome.scripting.executeScript({
      target: { tabId: tab.id },
      func: readPage,
    });
    const injected = results[0]?.result as PageFacts | undefined;
    if (!injected) throw new Error("the page returned nothing");
    facts = injected;
  } catch {
    // activeTab does not extend to chrome:// pages or the extension gallery,
    // and saying so beats an empty form the user cannot explain.
    setStatus("Chrome does not allow reading this page. Open the LinkedIn page first.", true);
    return;
  }

  const draft = draftFrom(facts, new Date());
  fillForm(draft);
  el<HTMLFormElement>("form").hidden = false;

  if (draft.missing.includes("account")) {
    setStatus("Which account is this about? vigil will not guess a name.");
  } else {
    setStatus(`Captured from ${new URL(facts.url).hostname}.`);
  }

  el<HTMLFormElement>("form").addEventListener("submit", async (event) => {
    event.preventDefault();
    const button = el<HTMLButtonElement>("send");
    button.disabled = true;

    const occurred = el<HTMLInputElement>("occurred-at").valueAsNumber;
    const request: IngestRequest = {
      account: {
        name: el<HTMLInputElement>("account-name").value.trim(),
        profile: el<HTMLInputElement>("account-profile").value.trim() || undefined,
        stage: "acquire",
      },
      signals: [
        {
          kind: el<HTMLSelectElement>("kind").value,
          source: "linkedin",
          title: el<HTMLInputElement>("title").value.trim(),
          body: el<HTMLTextAreaElement>("body").value.trim(),
          url: facts.url,
          occurred_at: toRFC3339(Number.isNaN(occurred) ? new Date() : new Date(occurred)),
        },
      ],
    };
    const personName = el<HTMLInputElement>("person-name").value.trim();
    if (personName !== "") {
      request.person = { name: personName, profile: draft.personProfile || undefined };
    }

    try {
      const result = await send(await loadSettings(), request);
      showResult(describe(result), result.inserted === 0 && result.rejected.length > 0);
    } catch (error) {
      showResult(error instanceof VigilError ? error.message : String(error), true);
    } finally {
      button.disabled = false;
    }
  });

  el<HTMLAnchorElement>("open-options").addEventListener("click", (event) => {
    event.preventDefault();
    chrome.runtime.openOptionsPage();
  });
}

main().catch((error) => setStatus(String(error), true));
