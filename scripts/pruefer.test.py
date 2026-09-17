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
import re
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

HIER = Path(__file__).resolve().parent
HINTERGRUND = HIER / "pruefe-keine-hintergrundabfrage.py"
HERKUNFT = HIER / "pruefe-herkunftszeile.py"
LIZENZEN = HIER / "pruefe-lizenzhinweise.py"
ABGESICHERT = HIER / "pruefe-release-abgesichert.py"

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


class Herkunftszeile(unittest.TestCase):
    """Der DCO-Pruefer, gegen echte Git-Baeume.

    Es gibt hier keine Attrappe: `git log` ist genau das, was schiefgehen kann,
    und ein Test, der die Commits von Hand hineinreicht, haette den Fall mit
    der abweichenden Autorenadresse nie gesehen.
    """

    def repo(self) -> Path:
        verzeichnis = Path(tempfile.mkdtemp())
        self.git(verzeichnis, "init", "--quiet", "--initial-branch", "main")
        self.git(verzeichnis, "config", "user.name", "Test Person")
        self.git(verzeichnis, "config", "user.email", "test@example.org")
        self.git(verzeichnis, "commit", "--quiet", "--allow-empty",
                 "-m", "chore: basis", "-m", "Signed-off-by: Test Person <test@example.org>")
        return verzeichnis

    def git(self, wurzel: Path, *argumente: str) -> None:
        lauf = subprocess.run(["git", "-C", str(wurzel), *argumente],
                              capture_output=True, text=True)
        self.assertEqual(lauf.returncode, 0, lauf.stderr)

    def laufe(self, wurzel: Path, bereich: str) -> subprocess.CompletedProcess:
        return subprocess.run(
            [sys.executable, str(HERKUNFT), "--bereich", bereich],
            cwd=wurzel, capture_output=True, text=True,
        )

    def test_ein_unterschriebener_commit_geht_durch(self):
        wurzel = self.repo()
        self.git(wurzel, "commit", "--quiet", "--allow-empty", "-s", "-m", "feat: etwas")
        lauf = self.laufe(wurzel, "HEAD~1..HEAD")
        self.assertEqual(lauf.returncode, 0, lauf.stdout + lauf.stderr)
        self.assertIn("1 Commit(s)", lauf.stdout)

    def test_ein_commit_ohne_zeile_faellt_auf(self):
        wurzel = self.repo()
        self.git(wurzel, "commit", "--quiet", "--allow-empty", "-m", "feat: etwas")
        lauf = self.laufe(wurzel, "HEAD~1..HEAD")
        self.assertEqual(lauf.returncode, 1, lauf.stdout)
        self.assertIn("keine Signed-off-by-Zeile", lauf.stdout)

    def test_eine_fremde_unterschrift_zaehlt_nicht(self):
        # Sonst unterschreibt A fuer die Arbeit von B, und die Erklaerung ist
        # keine Erklaerung ueber die eigene Arbeit mehr.
        wurzel = self.repo()
        self.git(wurzel, "commit", "--quiet", "--allow-empty", "-m", "feat: etwas",
                 "-m", "Signed-off-by: Jemand Anders <anders@example.org>")
        lauf = self.laufe(wurzel, "HEAD~1..HEAD")
        self.assertEqual(lauf.returncode, 1, lauf.stdout)
        self.assertIn("Autor ist aber test@example.org", lauf.stdout)

    def test_gross_und_kleinschreibung_der_adresse_ist_egal(self):
        wurzel = self.repo()
        self.git(wurzel, "commit", "--quiet", "--allow-empty", "-m", "feat: etwas",
                 "-m", "Signed-off-by: Test Person <TEST@Example.ORG>")
        lauf = self.laufe(wurzel, "HEAD~1..HEAD")
        self.assertEqual(lauf.returncode, 0, lauf.stdout)

    def test_eine_zusammenfuehrung_braucht_keine_zeile(self):
        # Die legt GitHub an, nicht der Beitragende.
        wurzel = self.repo()
        self.git(wurzel, "checkout", "--quiet", "-b", "zweig")
        self.git(wurzel, "commit", "--quiet", "--allow-empty", "-s", "-m", "feat: zweig")
        self.git(wurzel, "checkout", "--quiet", "main")
        self.git(wurzel, "commit", "--quiet", "--allow-empty", "-s", "-m", "feat: main")
        basis = subprocess.run(["git", "-C", str(wurzel), "rev-parse", "HEAD"],
                               capture_output=True, text=True).stdout.strip()
        self.git(wurzel, "merge", "--quiet", "--no-ff", "--no-verify",
                 "-m", "Merge branch 'zweig'", "zweig")
        lauf = self.laufe(wurzel, f"{basis}..HEAD")
        self.assertEqual(lauf.returncode, 0, lauf.stdout)
        self.assertIn("1 Zusammenfuehrung(en) ausgenommen", lauf.stdout)

    def test_ein_leerer_bereich_meldet_nicht_gruen(self):
        # "0 Befunde" ist auch, was ein Pruefer meldet, dessen Bereich daneben
        # zeigt. In der CI waere das ein Beitrag, den niemand geprueft hat.
        wurzel = self.repo()
        lauf = self.laufe(wurzel, "HEAD..HEAD")
        self.assertEqual(lauf.returncode, 1)
        self.assertIn("kein Commit im Bereich", lauf.stderr)

    def test_ein_kaputter_bereich_bricht_laut_ab(self):
        wurzel = self.repo()
        lauf = self.laufe(wurzel, "gibtesnicht..HEAD")
        self.assertEqual(lauf.returncode, 1)
        self.assertIn("scheiterte", lauf.stderr)


class Lizenzhinweise(unittest.TestCase):
    """Der Lizenz-Pruefer.

    Es gibt keine Attrappe fuer `go list`: die Modulliste ist genau das, was
    veraltet, und ein Test mit einer von Hand gereichten Liste haette das nie
    bemerkt. Stattdessen laeuft der Pruefer im Verzeichnis des Projekts und
    bekommt mit --wurzel ein Temporaerverzeichnis als Ablageort.
    """

    def laufe(self, wurzel: Path, *zusatz: str) -> subprocess.CompletedProcess:
        return subprocess.run(
            [sys.executable, str(LIZENZEN), "--wurzel", str(wurzel), *zusatz],
            cwd=HIER.parent, capture_output=True, text=True,
        )

    def test_die_eingecheckte_datei_passt_zu_den_gelinkten_modulen(self):
        # Der wichtigste Fall: der Baum, um den es geht. Wird er rot, fehlt in
        # THIRD-PARTY-NOTICES.md der Lizenztext einer Abhaengigkeit, und jedes
        # Release-Archiv wuerde ihn ebenfalls nicht enthalten.
        lauf = self.laufe(HIER.parent)
        self.assertEqual(lauf.returncode, 0, lauf.stdout + lauf.stderr)
        self.assertIn("gelinkte(s) Modul(e)", lauf.stdout)

    def test_eine_fehlende_datei_faellt_auf(self):
        with tempfile.TemporaryDirectory() as leer:
            lauf = self.laufe(Path(leer))
        self.assertEqual(lauf.returncode, 1)
        self.assertIn("fehlt", lauf.stdout)

    def test_eine_veraltete_datei_faellt_auf(self):
        with tempfile.TemporaryDirectory() as tmp:
            wurzel = Path(tmp)
            (wurzel / "THIRD-PARTY-NOTICES.md").write_text(
                "# Third-party notices\n\nvon Hand gepflegt und laengst veraltet\n",
                encoding="utf-8")
            lauf = self.laufe(wurzel)
        self.assertEqual(lauf.returncode, 1)
        self.assertIn("passt nicht", lauf.stdout)

    def test_geschrieben_und_danach_gruen(self):
        with tempfile.TemporaryDirectory() as tmp:
            wurzel = Path(tmp)
            schreiben = self.laufe(wurzel, "--schreiben")
            self.assertEqual(schreiben.returncode, 0, schreiben.stdout + schreiben.stderr)
            inhalt = (wurzel / "THIRD-PARTY-NOTICES.md").read_text(encoding="utf-8")
            self.assertIn("modernc.org/sqlite", inhalt)
            self.assertIn("Permission is hereby granted", inhalt)
            self.assertEqual(self.laufe(wurzel).returncode, 0)

    def test_die_liste_ist_die_vereinigung_ueber_alle_bauziele(self):
        # Am 17.09.2026 rot geworden: auf macOS meldet `go list` zehn Module,
        # auf dem Linux-Laeufer acht. go-isatty und go-strftime stehen hinter
        # Build-Bedingungen. Eine auf einem Rechner erzeugte Datei war damit
        # auf dem anderen falsch, und im Linux-Archiv haetten zwei Lizenztexte
        # gefehlt, die das darwin-Archiv braucht.
        inhalt = (HIER.parent / "THIRD-PARTY-NOTICES.md").read_text(encoding="utf-8")
        for nur_auf_darwin_und_windows in ("github.com/mattn/go-isatty",
                                           "github.com/ncruces/go-strftime"):
            self.assertIn(nur_auf_darwin_und_windows, inhalt,
                          "die Liste ist die des Laufrechners, nicht die Vereinigung")

    def test_die_bauziele_stammen_aus_dem_workflow(self):
        # Zwei Listen laufen auseinander. Der Pruefer liest die Matrix aus dem
        # Veroeffentlichungs-Workflow; dieser Test haelt fest, dass die
        # erzeugte Datei genau diese Ziele nennt.
        workflow = (HIER.parent / ".github" / "workflows"
                    / "veroeffentlichen.yml").read_text(encoding="utf-8")
        aus_workflow = set(re.findall(
            r"\{\s*goos:\s*([a-z0-9]+)\s*,\s*goarch:\s*([a-z0-9]+)\s*\}", workflow))
        self.assertGreaterEqual(len(aus_workflow), 2, "keine Matrix im Workflow gefunden")

        inhalt = (HIER.parent / "THIRD-PARTY-NOTICES.md").read_text(encoding="utf-8")
        aus_datei = set(re.findall(r"^- `([a-z0-9]+)/([a-z0-9]+)`$", inhalt, re.MULTILINE))
        self.assertEqual(aus_datei, aus_workflow)

    def test_jedes_modul_bringt_seinen_lizenztext_mit(self):
        # Ein Eintrag ohne Text erfuellt die Auflage nicht. "see the text
        # below" waere dann eine Luege auf Papier.
        inhalt = (HIER.parent / "THIRD-PARTY-NOTICES.md").read_text(encoding="utf-8")
        abschnitte = inhalt.split("\n## ")[1:]
        self.assertGreaterEqual(len(abschnitte), 5, "zu wenige Module aufgefuehrt")
        for abschnitt in abschnitte:
            name = abschnitt.splitlines()[0]
            self.assertIn("```", abschnitt, f"{name} hat keinen Lizenztext")
            self.assertRegex(abschnitt, r"Copyright", f"{name} nennt keinen Urheber")


PRUEFUNGEN_MIT_AUFRUF = """name: Pruefungen
on:
  push:
    branches: [main]
  workflow_call:
jobs:
  pruefen:
    runs-on: ubuntu-latest
    steps:
      - run: echo
"""

RELEASE_MIT_TOR = """name: Veroeffentlichen
on:
  push:
    tags: ["v*"]
jobs:
  pruefen:
    uses: ./.github/workflows/pruefungen.yml

  bauen:
    needs: pruefen
    runs-on: ubuntu-latest
    steps:
      - run: echo bauen

  veroeffentlichen:
    needs: bauen
    runs-on: ubuntu-latest
    steps:
      - run: echo veroeffentlichen
"""


class Releaseabgesichert(unittest.TestCase):
    """Der Pruefer, der das Tor vor dem Release haelt."""

    def baue(self, release: str, pruefungen: str | None = PRUEFUNGEN_MIT_AUFRUF) -> Path:
        wurzel = Path(tempfile.mkdtemp())
        workflows = wurzel / ".github" / "workflows"
        workflows.mkdir(parents=True)
        (workflows / "veroeffentlichen.yml").write_text(release, encoding="utf-8")
        if pruefungen is not None:
            (workflows / "pruefungen.yml").write_text(pruefungen, encoding="utf-8")
        return wurzel

    def laufe(self, wurzel: Path) -> subprocess.CompletedProcess:
        return subprocess.run(
            [sys.executable, str(ABGESICHERT), "--wurzel", str(wurzel)],
            capture_output=True, text=True,
        )

    def test_der_echte_workflow_haengt_an_den_pruefungen(self):
        # Der wichtigste Fall. Wird er rot, kann ein Tag ungeprueft
        # veroeffentlichen, und der Release-Lauf waere trotzdem gruen.
        lauf = self.laufe(HIER.parent)
        self.assertEqual(lauf.returncode, 0, lauf.stdout + lauf.stderr)
        self.assertIn("Tor(e): pruefen", lauf.stdout)

    def test_ein_punktverzeichnis_im_aufruf_wird_richtig_aufgeloest(self):
        # lstrip("./") entfernt JEDES fuehrende "." und "/", macht aus
        # "./.github/..." also "github/..." und meldet eine vorhandene Datei
        # als fehlend. Beim ersten Lauf genau so passiert.
        lauf = self.laufe(self.baue(RELEASE_MIT_TOR))
        self.assertEqual(lauf.returncode, 0, lauf.stdout)
        self.assertNotIn("gibt es nicht", lauf.stdout)

    def test_ein_job_ohne_needs_faellt_auf(self):
        ohne = RELEASE_MIT_TOR.replace("  bauen:\n    needs: pruefen\n", "  bauen:\n")
        lauf = self.laufe(self.baue(ohne))
        self.assertEqual(lauf.returncode, 1, lauf.stdout)
        self.assertIn("'bauen'", lauf.stdout)

    def test_ein_release_ganz_ohne_tor_faellt_auf(self):
        ohne_tor = """name: Veroeffentlichen
on:
  push:
    tags: ["v*"]
jobs:
  bauen:
    runs-on: ubuntu-latest
    steps:
      - run: echo bauen

  veroeffentlichen:
    needs: bauen
    runs-on: ubuntu-latest
    steps:
      - run: echo
"""
        lauf = self.laufe(self.baue(ohne_tor))
        self.assertEqual(lauf.returncode, 1, lauf.stdout)
        self.assertIn("Kein Job ruft einen lokalen Workflow auf", lauf.stdout)

    def test_ein_aufruf_auf_eine_fehlende_datei_faellt_auf(self):
        lauf = self.laufe(self.baue(RELEASE_MIT_TOR, pruefungen=None))
        self.assertEqual(lauf.returncode, 1, lauf.stdout)
        self.assertIn("gibt es nicht", lauf.stdout)

    def test_ein_aufruf_ohne_workflow_call_faellt_auf(self):
        # Der Aufruf scheitert sonst erst zur Laufzeit, und zwar in dem Lauf,
        # der veroeffentlichen sollte.
        ohne = PRUEFUNGEN_MIT_AUFRUF.replace("  workflow_call:\n", "")
        lauf = self.laufe(self.baue(RELEASE_MIT_TOR, pruefungen=ohne))
        self.assertEqual(lauf.returncode, 1, lauf.stdout)
        self.assertIn("workflow_call", lauf.stdout)

    def test_needs_als_liste_zaehlt_auch(self):
        liste = RELEASE_MIT_TOR.replace("needs: bauen", "needs: [bauen]")
        lauf = self.laufe(self.baue(liste))
        self.assertEqual(lauf.returncode, 0, lauf.stdout)

    def test_ein_workflow_mit_einem_job_bricht_laut_ab(self):
        # Wahrscheinlicher als ein echter Ein-Job-Release ist, dass die Muster
        # danebengreifen. Dann darf nicht "0 Befunde" herauskommen.
        einer = """name: Veroeffentlichen
on:
  workflow_dispatch:
jobs:
  bauen:
    runs-on: ubuntu-latest
    steps:
      - run: echo
"""
        lauf = self.laufe(self.baue(einer))
        self.assertEqual(lauf.returncode, 1)
        self.assertIn("Job(s) erkannt", lauf.stderr)


if __name__ == "__main__":
    unittest.main()
