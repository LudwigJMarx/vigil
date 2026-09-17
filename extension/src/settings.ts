import type { Settings } from "./types.ts";

export const DEFAULT_SETTINGS: Settings = { server: "http://127.0.0.1:8099", token: "" };

/**
 * The token lives in chrome.storage.local, not sync. Sync would copy the
 * credential to every browser signed into the same Google account, including
 * ones the owner of the vigil instance does not control.
 */
export async function loadSettings(): Promise<Settings> {
  const stored = await chrome.storage.local.get({ ...DEFAULT_SETTINGS });
  return { ...DEFAULT_SETTINGS, ...stored } as Settings;
}

export async function saveSettings(settings: Settings): Promise<void> {
  await chrome.storage.local.set(settings);
}
