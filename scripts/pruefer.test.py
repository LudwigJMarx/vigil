#!/usr/bin/env python3
"""Tests fuer die projekteigenen Pruefer.

── WARUM DIESE DATEI ───────────────────────────────────────────────────────

Ein Pruefer, der nie einen Befund erzeugt hat, ist von einem, der keinen
erzeugen kann, nicht zu unterscheiden. Beide melden null. Diese Tests bauen
deshalb Baeume, in denen es etwas zu finden gibt, und bestehen darauf, dass es
gefunden wird.

Beide Pruefer werden als Programm ausgefuehrt, nicht als importierte Funktion:
der Rueckgabewert ist das, woran die CI haengt, und ein Test, der nur die
Innereien aufruft, bleibt gruen, wenn `main` den Wert nicht mehr setzt.

Aufruf:  python3 scripts/pruefer.test.py
"""
from __future__ import annotations

import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

HIER = Path(__file__).resolve().parent
HINTERGRUND = HIER / "pruefe-keine-hintergrundabfrage.py"

SAUBERES_MANIFEST = {
    "manifest_version": 3,
    "name": "vigil capture",
    "version": "0.1.0",
    "action": {"default_popup": "src/popup.html"},
    "permissions": ["activeTab", "storage", "scripting"],
}


class KeineHintergrundabfrage(unittest.TestCase):
    def baue(self, manifest: dict, quellen: dict[str, str] | None = None) -> Path:
        verzeichnis = Path(tempfile.mkdtemp())
        self.addCleanup(lambda: None)  # das Temp-Verzeichnis bleibt dem Laeufer ueberlassen
        quelle = verzeichnis / "extension" / "src"
        quelle.mkdir(parents=True)
        (verzeichnis / "extension" / "manifest.json").write_text(
            json.dumps(manifest), encoding="utf-8")
        for name, inhalt in (quellen or {"popup.ts": "console.log('hi');\n"}).items():
            (quelle / name).write_text(inhalt, encoding="utf-8")
        return verzeichnis

    def laufe(self, wurzel: Path) -> subprocess.CompletedProcess:
        return subprocess.run(
            [sys.executable, str(HINTERGRUND), "--wurzel", str(wurzel)],
            capture_output=True, text=True,
        )

    def test_die_echte_erweiterung_ist_sauber(self):
        # Der wichtigste Fall: der Baum, um den es geht.
        lauf = self.laufe(HIER.parent)
        self.assertEqual(lauf.returncode, 0, lauf.stdout + lauf.stderr)
        self.assertIn("0 Befund(e)", lauf.stdout)

    def test_ein_sauberes_manifest_geht_durch(self):
        lauf = self.laufe(self.baue(SAUBERES_MANIFEST))
        self.assertEqual(lauf.returncode, 0, lauf.stdout + lauf.stderr)

    def test_host_permissions_fallen_auf(self):
        manifest = SAUBERES_MANIFEST | {"host_permissions": ["https://*.linkedin.com/*"]}
        lauf = self.laufe(self.baue(manifest))
        self.assertEqual(lauf.returncode, 1)
        self.assertIn("host_permissions", lauf.stdout)

    def test_ein_service_worker_faellt_auf(self):
        manifest = SAUBERES_MANIFEST | {"background": {"service_worker": "sw.js"}}
        lauf = self.laufe(self.baue(manifest))
        self.assertEqual(lauf.returncode, 1)
        self.assertIn("background", lauf.stdout)

    def test_ein_content_script_mit_matches_faellt_auf(self):
        manifest = SAUBERES_MANIFEST | {
            "content_scripts": [{"matches": ["https://www.linkedin.com/*"], "js": ["c.js"]}]
        }
        lauf = self.laufe(self.baue(manifest))
        self.assertEqual(lauf.returncode, 1)
        self.assertIn("content_scripts", lauf.stdout)

    def test_die_alarms_berechtigung_faellt_auf(self):
        manifest = SAUBERES_MANIFEST | {"permissions": ["activeTab", "alarms"]}
        lauf = self.laufe(self.baue(manifest))
        self.assertEqual(lauf.returncode, 1)
        self.assertIn("alarms", lauf.stdout)

    def test_setinterval_im_quelltext_faellt_auf(self):
        lauf = self.laufe(self.baue(
            SAUBERES_MANIFEST,
            {"poll.ts": "setInterval(() => { collect(); }, 60000);\n"},
        ))
        self.assertEqual(lauf.returncode, 1)
        self.assertIn("setInterval", lauf.stdout)

    def test_ein_fetch_gegen_linkedin_faellt_auf(self):
        lauf = self.laufe(self.baue(
            SAUBERES_MANIFEST,
            {"crawl.ts": 'const r = await fetch("https://www.linkedin.com/feed/");\n'},
        ))
        self.assertEqual(lauf.returncode, 1)
        self.assertIn("linkedin.com", lauf.stdout)

    def test_ein_kommentar_der_das_verbot_erklaert_ist_kein_verstoss(self):
        # Sonst entschuldigt oder beschuldigt sich die Dokumentation selbst.
        lauf = self.laufe(self.baue(
            SAUBERES_MANIFEST,
            {"popup.ts": "// Kein setInterval hier, und kein chrome.alarms.\nexport {};\n"},
        ))
        self.assertEqual(lauf.returncode, 0, lauf.stdout)

    def test_eine_fehlende_erweiterung_meldet_nicht_gruen(self):
        # "0 Befunde" ist auch, was ein Pruefer meldet, der am falschen Ort
        # gesucht hat. Die beiden Faelle muessen unterscheidbar bleiben.
        with tempfile.TemporaryDirectory() as leer:
            lauf = self.laufe(Path(leer))
        self.assertEqual(lauf.returncode, 1)
        self.assertIn("gibt es nicht", lauf.stderr)

    def test_ein_leeres_quellverzeichnis_wird_benannt(self):
        verzeichnis = Path(tempfile.mkdtemp())
        (verzeichnis / "extension" / "src").mkdir(parents=True)
        (verzeichnis / "extension" / "manifest.json").write_text(
            json.dumps(SAUBERES_MANIFEST), encoding="utf-8")
        lauf = self.laufe(verzeichnis)
        self.assertEqual(lauf.returncode, 0)
        self.assertIn("keine .ts-Datei", lauf.stdout)


DOKU = HIER / "pruefe-doku-befehle.py"

HILFETEXT = """vigil - watch the accounts you sell to.

Usage:
  vigil serve [flags]
  vigil token create --name NAME
  vigil token list
  vigil version
"""


class DokuBefehle(unittest.TestCase):
    def baue(self, markdown: str) -> Path:
        verzeichnis = Path(tempfile.mkdtemp())
        (verzeichnis / "README.md").write_text(markdown, encoding="utf-8")
        (verzeichnis / "hilfe.txt").write_text(HILFETEXT, encoding="utf-8")
        return verzeichnis

    def laufe(self, wurzel: Path) -> subprocess.CompletedProcess:
        return subprocess.run(
            [sys.executable, str(DOKU), "--wurzel", str(wurzel),
             "--hilfetext", str(wurzel / "hilfe.txt")],
            capture_output=True, text=True,
        )

    def test_ein_erfundener_befehl_faellt_auf(self):
        lauf = self.laufe(self.baue("```bash\nvigil sync --all\n```\n"))
        self.assertEqual(lauf.returncode, 1, lauf.stdout)
        self.assertIn("vigil sync --all", lauf.stdout)

    def test_ein_echter_befehl_geht_durch(self):
        lauf = self.laufe(self.baue("```bash\nvigil token create --name browser\n```\n"))
        self.assertEqual(lauf.returncode, 0, lauf.stdout)
        self.assertIn("1 vigil-Aufruf(e)", lauf.stdout)

    def test_ein_erfundener_unterbefehl_faellt_auf(self):
        lauf = self.laufe(self.baue("```bash\nvigil token rotate\n```\n"))
        self.assertEqual(lauf.returncode, 1, lauf.stdout)

    def test_ein_block_ohne_sprachangabe_zeigt_ausgabe_und_wird_uebergangen(self):
        # Der Pruefer hat frueher `vigil 0.1.0 listening on ...` aus einem
        # Ausgabeblock der README fuer einen Aufruf des Befehls "0.1.0" gehalten.
        lauf = self.laufe(self.baue("```\nvigil 0.1.0 listening on http://127.0.0.1:8099\n```\n"))
        self.assertEqual(lauf.returncode, 0, lauf.stdout)
        self.assertIn("0 vigil-Aufruf(e)", lauf.stdout)

    def test_ausserhalb_eines_codeblocks_wird_nichts_geprueft(self):
        # Fliesstext nennt Befehle beilaeufig und oft unvollstaendig. Wer den
        # mitprueft, erzeugt Befunde, die niemand abstellen kann.
        lauf = self.laufe(self.baue("Frueher hiess das vigil sync --all.\n"))
        self.assertEqual(lauf.returncode, 0, lauf.stdout)
        self.assertIn("0 vigil-Aufruf(e)", lauf.stdout)

    def test_ohne_jeden_aufruf_wird_das_gesagt(self):
        lauf = self.laufe(self.baue("Nur Text, kein Block.\n"))
        self.assertEqual(lauf.returncode, 0)
        self.assertIn("kein einziger vigil-Aufruf", lauf.stdout)

    def test_gegen_den_echten_baum_ohne_attrappe(self):
        # Die Naht: hier wird das Programm wirklich gebaut und befragt. Der
        # Block oben haette den Hilfetext von Hand hineingereicht.
        lauf = subprocess.run(
            [sys.executable, str(DOKU), "--wurzel", str(HIER.parent)],
            capture_output=True, text=True,
        )
        self.assertEqual(lauf.returncode, 0, lauf.stdout + lauf.stderr)
        self.assertIn("go run ./cmd/vigil help", lauf.stdout)


if __name__ == "__main__":
    unittest.main()
