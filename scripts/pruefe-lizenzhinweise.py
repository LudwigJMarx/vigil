#!/usr/bin/env python3
"""Prueft, dass THIRD-PARTY-NOTICES.md zu den tatsaechlich gelinkten Modulen passt.

── WARUM ES DAS BRAUCHT ────────────────────────────────────────────────────

Zehn fremde Module landen im vigil-Binary. Alle stehen unter MIT oder
BSD-3-Clause, und beide Lizenzen verlangen dasselbe: bei der Weitergabe in
Binaerform muessen Urheberrechtsvermerk und Lizenztext mitgeliefert werden.
Ein Release-Archiv mit nur LICENSE und NOTICE erfuellt das nicht.

Das faellt niemandem auf. Es gibt keine Fehlermeldung, keinen roten Lauf und
keinen Nutzer, der sich beschwert. Es ist einfach ein Lizenzverstoss, der mit
jedem Download weitergegeben wird. Also prueft das ein Skript, und zwar gegen
die Modulliste, die `go list -deps` aus dem echten Bauziel liest, nicht gegen
eine gepflegte Aufzaehlung.

── WAS ES PRUEFT ───────────────────────────────────────────────────────────

Die Datei THIRD-PARTY-NOTICES.md wird neu erzeugt und mit der eingecheckten
verglichen. Weicht sie ab, ist der Lauf rot und `--schreiben` stellt sie her.
Damit kann eine neue Abhaengigkeit nicht ohne ihren Lizenztext hineinkommen.

Beruecksichtigt wird, was in `./cmd/vigil` gelinkt wird. Die devDependencies
der Erweiterung (TypeScript, @types/chrome) werden nicht mitgeliefert und
stehen deshalb auch nicht drin.

── WAS ES AUSDRUECKLICH NICHT SIEHT ────────────────────────────────────────

Ob die Lizenzen miteinander vertraeglich sind. Es sammelt und vergleicht; es
beurteilt nicht. Ein Modul unter GPL wuerde hier anstandslos aufgenommen und
muesste im Review auffallen.

Aufruf:  python3 scripts/pruefe-lizenzhinweise.py [--schreiben]
"""
from __future__ import annotations

import argparse
import subprocess
import sys
from pathlib import Path

DATEI = "THIRD-PARTY-NOTICES.md"
ZIEL = "./cmd/vigil"
LIZENZNAMEN = ("LICENSE", "LICENCE", "LICENSE.md", "LICENSE.txt", "COPYING", "COPYRIGHT")

KOPF = """# Third-party notices

vigil links the Go modules below into its binary. Each is distributed under a
permissive licence that requires its copyright notice and licence text to
accompany a binary distribution, so both are reproduced here in full, and this
file ships inside every release archive.

This file is generated. Run `python3 scripts/pruefe-lizenzhinweise.py
--schreiben` after changing a dependency; CI fails if it is out of date.

The browser extension's development dependencies (TypeScript, @types/chrome)
are not distributed and are therefore not listed.
"""


def go(*argumente: str) -> str:
    lauf = subprocess.run(["go", *argumente], capture_output=True, text=True)
    if lauf.returncode != 0:
        # Laut abbrechen: ein Pruefer, der die Modulliste nicht bekommen hat,
        # meldet sonst "alles in Ordnung" wie einer, der alles gelesen hat.
        raise SystemExit(
            f"lizenzhinweise: `go {' '.join(argumente)}` scheiterte "
            f"({lauf.returncode}):\n{lauf.stderr.strip()}"
        )
    return lauf.stdout


def module() -> list[tuple[str, str, Path]]:
    """Die fremden Module, die im Bauziel landen: Pfad, Version, Verzeichnis."""
    roh = go("list", "-deps", "-f",
             "{{with .Module}}{{if not .Main}}{{.Path}}\t{{.Version}}\t{{.Dir}}{{end}}{{end}}",
             ZIEL)
    gefunden: dict[str, tuple[str, str, Path]] = {}
    for zeile in roh.splitlines():
        if not zeile.strip():
            continue
        pfad, version, verzeichnis = zeile.split("\t")
        gefunden[pfad] = (pfad, version, Path(verzeichnis))
    return [gefunden[p] for p in sorted(gefunden)]


def lizenzdatei(verzeichnis: Path) -> Path | None:
    for name in LIZENZNAMEN:
        kandidat = verzeichnis / name
        if kandidat.is_file():
            return kandidat
    return None


def art(text: str) -> str:
    """Eine Bequemlichkeit fuer den Leser. Verbindlich ist der Text darunter.

    Die erste Fassung suchte "name of" und hielt deshalb drei der vier
    modernc-Module fuer unbekannt: deren dritte Klausel lautet "Neither the
    names of the authors". Ein Etikett, das bei jedem vierten Modul passt,
    ist schlechter als keines, weil es so aussieht, als haette jemand
    nachgesehen.
    """
    klein = text.lower()
    if "redistributions in binary form" in klein and "neither the name" in klein:
        return "BSD-3-Clause"
    if "substantial portions of the software" in klein:
        return "MIT"
    return "see the text below"


def erzeuge() -> tuple[str, list[str]]:
    """Gibt den Inhalt und die Liste der Module ohne auffindbare Lizenzdatei."""
    teile = [KOPF]
    ohne: list[str] = []
    for pfad, version, verzeichnis in module():
        datei = lizenzdatei(verzeichnis)
        if datei is None:
            ohne.append(pfad)
            continue
        text = datei.read_text(encoding="utf-8", errors="replace").strip()
        teile.append(
            f"\n## {pfad}\n\n"
            f"Version `{version}`, {art(text)}, from `{datei.name}`.\n\n"
            f"```\n{text}\n```\n"
        )
    return "".join(teile), ohne


def main() -> int:
    zerleger = argparse.ArgumentParser(description="Lizenztexte der Abhaengigkeiten.")
    zerleger.add_argument("--schreiben", action="store_true",
                          help="die Datei herstellen statt sie zu vergleichen")
    zerleger.add_argument("--wurzel", default=".", type=Path)
    argumente = zerleger.parse_args()
    wurzel = argumente.wurzel.resolve()

    inhalt, ohne = erzeuge()
    anzahl = inhalt.count("\n## ")
    ziel = wurzel / DATEI

    if anzahl == 0:
        raise SystemExit(
            "lizenzhinweise: kein einziges fremdes Modul gefunden. "
            "Entweder ist das Bauziel falsch, oder go list hat nichts geliefert."
        )

    if argumente.schreiben:
        ziel.write_text(inhalt, encoding="utf-8")
        print(f"lizenzhinweise: {DATEI} mit {anzahl} Modul(en) geschrieben.")
        for pfad in ohne:
            print(f"  ohne auffindbare Lizenzdatei: {pfad}")
        return 1 if ohne else 0

    print(f"lizenzhinweise: {anzahl} gelinkte(s) Modul(e) gegen {DATEI} gehalten.")
    for pfad in ohne:
        print(f"  ohne auffindbare Lizenzdatei: {pfad}")

    if not ziel.is_file():
        print(f"  {DATEI} fehlt.")
        print()
        print("Abhilfe: python3 scripts/pruefe-lizenzhinweise.py --schreiben")
        return 1

    if ziel.read_text(encoding="utf-8") != inhalt:
        print(f"  {DATEI} passt nicht zu den gelinkten Modulen.")
        print()
        print("Abhilfe: python3 scripts/pruefe-lizenzhinweise.py --schreiben")
        return 1

    return 1 if ohne else 0


if __name__ == "__main__":
    sys.exit(main())
