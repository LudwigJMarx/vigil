#!/usr/bin/env python3
"""Prueft, dass die Erweiterung LinkedIn nicht von sich aus abfragt.

── WARUM ES DAS BRAUCHT ────────────────────────────────────────────────────

vigil steht und faellt mit einer Zusage: die Erweiterung liest nur die Seite,
die gerade offen ist, und nur in dem Moment, in dem der Mensch auf das Symbol
klickt. Wer sie zum Hintergrundsammler umbaut, riskiert nicht sein eigenes
Konto, sondern das der Nutzer: LinkedIn sperrt das Konto, das die Anfragen
stellt.

Diese Zusage ist in einer README schnell geschrieben und beim naechsten
Feature genauso schnell gebrochen, ohne dass es jemandem auffaellt. Vier
Zeilen in `manifest.json` reichen. Also prueft sie ein Skript.

── WAS ES PRUEFT ───────────────────────────────────────────────────────────

In `extension/manifest.json`:
  1. kein `host_permissions` und kein `optional_host_permissions`
  2. keine der Berechtigungen `alarms`, `webRequest`, `background`, `tabs`,
     `cookies`, `webNavigation`, `declarativeNetRequest`
  3. kein `background`-Abschnitt (Service Worker)
  4. kein `content_scripts` mit `matches` (das laeuft ohne Klick)

In `extension/src/*.ts`:
  5. kein `setInterval`
  6. kein `chrome.alarms`
  7. kein `fetch` auf eine linkedin.com-Adresse

── WAS ES AUSDRUECKLICH NICHT SIEHT ────────────────────────────────────────

Es liest Text, keinen ausgefuehrten Code. Wer denselben Aufruf ueber
`globalThis["fetch"]` und eine zusammengesetzte Zeichenkette baut, kommt daran
vorbei. Das ist die bekannte Grenze: der Pruefer haelt einen Umbau aus
Bequemlichkeit auf, keinen Umbau aus Absicht.

Aufruf:  python3 scripts/pruefe-keine-hintergrundabfrage.py [--wurzel PFAD]
"""
from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

VERBOTENE_BERECHTIGUNGEN = {
    "alarms", "background", "webRequest", "webRequestBlocking", "tabs",
    "cookies", "webNavigation", "declarativeNetRequest",
    "declarativeNetRequestWithHostAccess",
}

QUELLMUSTER = [
    (re.compile(r"\bsetInterval\s*\("), "setInterval: wiederholt etwas ohne Klick"),
    (re.compile(r"\bchrome\.alarms\b"), "chrome.alarms: weckt die Erweiterung ohne Klick"),
    (re.compile(r"""fetch\s*\(\s*[`'"][^`'"]*linkedin\.com"""),
     "fetch gegen linkedin.com: die Erweiterung liest die Seite, sie ruft sie nicht ab"),
]


def pruefe_manifest(pfad: Path) -> list[str]:
    befunde: list[str] = []
    try:
        manifest = json.loads(pfad.read_text(encoding="utf-8"))
    except FileNotFoundError:
        return [f"{pfad} fehlt"]
    except json.JSONDecodeError as fehler:
        return [f"{pfad} ist kein gueltiges JSON: {fehler}"]

    for schluessel in ("host_permissions", "optional_host_permissions"):
        if manifest.get(schluessel):
            befunde.append(
                f"manifest.json: {schluessel} = {manifest[schluessel]!r}. "
                "Damit erreicht die Erweiterung Seiten ohne Klick; activeTab reicht."
            )

    berechtigungen = set(manifest.get("permissions") or [])
    berechtigungen |= set(manifest.get("optional_permissions") or [])
    for verboten in sorted(berechtigungen & VERBOTENE_BERECHTIGUNGEN):
        befunde.append(f"manifest.json: Berechtigung {verboten!r} erlaubt Arbeit ohne Klick.")

    if manifest.get("background"):
        befunde.append(
            "manifest.json: background-Abschnitt vorhanden. Ein Service Worker laeuft, "
            "wenn niemand hinsieht."
        )

    for eintrag in manifest.get("content_scripts") or []:
        if eintrag.get("matches"):
            befunde.append(
                f"manifest.json: content_scripts mit matches={eintrag['matches']!r}. "
                "Das laedt auf jeder passenden Seite, auch ohne Klick."
            )
    return befunde


def pruefe_quellen(dateien: list[Path], wurzel: Path) -> list[str]:
    befunde: list[str] = []
    for datei in dateien:
        text = datei.read_text(encoding="utf-8", errors="ignore")
        for nummer, zeile in enumerate(text.splitlines(), start=1):
            if zeile.lstrip().startswith(("//", "*", "/*")):
                continue  # Ein Kommentar, der das Verbot erklaert, ist kein Verstoss.
            for muster, erklaerung in QUELLMUSTER:
                if muster.search(zeile):
                    ort = datei.relative_to(wurzel).as_posix()
                    befunde.append(f"{ort}:{nummer}: {erklaerung}")
    return befunde


def main() -> int:
    zerleger = argparse.ArgumentParser(description="Erweiterung sammelt nicht im Hintergrund.")
    zerleger.add_argument("--wurzel", default=".", type=Path)
    argumente = zerleger.parse_args()
    wurzel = argumente.wurzel.resolve()

    erweiterung = wurzel / "extension"
    if not erweiterung.is_dir():
        # Nicht gruen melden. "Nichts gefunden" und "am falschen Ort gesucht"
        # duerfen nie gleich aussehen.
        print(f"keine-hintergrundabfrage: {erweiterung} gibt es nicht.", file=sys.stderr)
        print("  Entweder ist die Erweiterung umgezogen, oder --wurzel zeigt daneben.",
              file=sys.stderr)
        return 1

    manifest = erweiterung / "manifest.json"
    quellen = sorted(p for p in (erweiterung / "src").rglob("*.ts") if p.is_file())
    befunde = pruefe_manifest(manifest) + pruefe_quellen(quellen, wurzel)

    print(
        f"keine-hintergrundabfrage: manifest.json und {len(quellen)} Quelldatei(en) geprueft, "
        f"{len(VERBOTENE_BERECHTIGUNGEN)} Berechtigung(en) und {len(QUELLMUSTER)} Muster, "
        f"{len(befunde)} Befund(e)."
    )
    if not quellen:
        print(f"  Achtung: unter {erweiterung}/src liegt keine .ts-Datei. "
              "Ein Pruefer ohne Eingabe meldet dasselbe wie einer ohne Befund.")

    for befund in befunde:
        print(f"  {befund}")
    if befunde:
        print()
        print("Die Erweiterung darf nur lesen, was der Mensch gerade geoeffnet hat.")
        print("Siehe extension/README.md, Abschnitt 'What it does not do'.")
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
