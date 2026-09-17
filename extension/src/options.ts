import { normaliseServer } from "./client.ts";
import { DEFAULT_SETTINGS, loadSettings, saveSettings } from "./settings.ts";

const el = <T extends HTMLElement>(id: string) => document.getElementById(id) as T;

function show(message: string, bad = false) {
  const node = el<HTMLParagraphElement>("result");
  node.textContent = message;
  node.classList.toggle("bad", bad);
  node.hidden = false;
}

async function main() {
  const settings = await loadSettings();
  el<HTMLInputElement>("server").value = settings.server || DEFAULT_SETTINGS.server;
  el<HTMLInputElement>("token").value = settings.token;

  el<HTMLFormElement>("form").addEventListener("submit", async (event) => {
    event.preventDefault();
    await saveSettings({
      server: normaliseServer(el<HTMLInputElement>("server").value),
      token: el<HTMLInputElement>("token").value.trim(),
    });
    show("Saved.");
  });

  // A "saved" message proves nothing about whether the server answers. This
  // button is the difference between configured and working.
  el<HTMLButtonElement>("check").addEventListener("click", async () => {
    const server = normaliseServer(el<HTMLInputElement>("server").value);
    try {
      const response = await fetch(`${server}/healthz`);
      const health = await response.json();
      show(
        `vigil ${health.version}: ${health.accounts} account(s), ` +
          `${health.signals} signal(s), ${health.active_tokens} active token(s).`,
      );
    } catch {
      show(`No answer from ${server}.`, true);
    }
  });
}

main().catch((error) => show(String(error), true));
