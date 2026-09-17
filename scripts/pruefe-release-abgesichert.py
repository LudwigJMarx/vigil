#!/usr/bin/env python3
"""Prueft, dass kein Job des Release-Workflows an den Pruefungen vorbeikommt.

── WARUM ES DAS BRAUCHT ────────────────────────────────────────────────────

`Pruefungen` laeuft bei Push auf main und bei jedem Beitrag, aber nicht bei
einem Tag. Am 17.09.2026 war v0.1.0 nur deshalb gedeckt, weil der getaggte
Commit vorher ueber main gelaufen war. Ein Tag auf einen Commit, der das nicht
war, waere ohne gofmt, ohne die Tests der Erweiterung und ohne den
Lizenz-Pruefer veroeffentlicht worden. Niemandem waere es aufgefallen: der
Release-Lauf war gruen, weil er nur `go test` kannte.

Die Pruefungen zusaetzlich auf Tags laufen zu lassen behebt das nicht. Sie
liefen dann NEBEN dem Release und hielten nichts auf. Es braucht eine
Abhaengigkeit, und eine Abhaengigkeit ist eine Zeile YAML, die jemand beim
naechsten Umbau entfernt. Also prueft sie ein Skript.

── WAS ES PRUEFT ───────────────────────────────────────────────────────────

In `.github/workflows/veroeffentlichen.yml`:

  1. Mindestens ein Job ruft einen lokalen Workflow auf
     (`uses: ./.github/workflows/....yml`). Das ist das Tor.
  2. Der aufgerufene Workflow existiert und hat `workflow_call:`. Ein Aufruf
     auf eine Datei ohne diesen Ausloeser scheitert erst zur Laufzeit.
  3. Jeder andere Job erreicht das Tor ueber `needs`, direkt oder ueber
     mehrere Stufen. Ein Job ohne diesen Weg baut oder veroeffentlicht,
     waehrend die Pruefungen noch laufen oder schon rot sind.

── WAS ES AUSDRUECKLICH NICHT SIEHT ────────────────────────────────────────

Es liest YAML mit Mustern, nicht mit einem Parser (die Standardbibliothek
bringt keinen mit). Findet es die Form nicht wieder, die es erwartet, bricht
es ab, statt "keine Befunde" zu melden. Ein Pruefer, der nichts erkannt hat,
und einer, der nichts gefunden hat, duerfen nicht gleich aussehen.

Ob die Pruefungen ihrerseits etwas taugen, ist nicht seine Frage. Dafuer gibt
es `pruefer-verdrahtet.py`.

Aufruf:  python3 scripts/pruefe-release-abgesichert.py [--wurzel PFAD]
"""
from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path

RELEASE = Path(".github/workflows/veroeffentlichen.yml")
JOB = re.compile(r"^  (?P<name>[A-Za-z_][A-Za-z0-9_-]*):\s*$")
RUFT_AUF = re.compile(r"^\s*uses:\s*(?P<pfad>\./[^\s#]+\.ya?ml)\s*$", re.MULTILINE)
BRAUCHT_EINZELN = re.compile(r"^\s*needs:\s*(?P<name>[A-Za-z_][A-Za-z0-9_-]*)\s*$", re.MULTILINE)
BRAUCHT_LISTE = re.compile(r"^\s*needs:\s*\[(?P<namen>[^\]]*)\]\s*$", re.MULTILINE)
BRAUCHT_BLOCK = re.compile(r"^\s*needs:\s*$(?P<block>(?:\n\s+-\s*[A-Za-z0-9_-]+)+)", re.MULTILINE)


def jobs(text: str) -> dict[str, str]:
    """Name -> Blocktext, fuer jeden Job unter `jobs:`."""
    zeilen = text.splitlines()
    try:
        start = next(i for i, z in enumerate(zeilen) if z.rstrip() == "jobs:")
    except StopIteration:
        raise SystemExit(f"release-abgesichert: in {RELEASE} gibt es keinen jobs:-Abschnitt.")

    gefunden: dict[str, list[str]] = {}
    aktuell: str | None = None
    for zeile in zeilen[start + 1:]:
        if zeile.strip() and not zeile.startswith(" "):
            break  # naechster Abschnitt auf oberster Ebene
        treffer = JOB.match(zeile)
        if treffer:
            aktuell = treffer.group("name")
            gefunden[aktuell] = []
            continue
        if aktuell is not None:
            gefunden[aktuell].append(zeile)
    return {name: "\n".join(block) for name, block in gefunden.items()}


def braucht(block: str) -> set[str]:
    namen: set[str] = set()
    for treffer in BRAUCHT_EINZELN.finditer(block):
        namen.add(treffer.group("name"))
    for treffer in BRAUCHT_LISTE.finditer(block):
        namen |= {t.strip().strip("'\"") for t in treffer.group("namen").split(",") if t.strip()}
    for treffer in BRAUCHT_BLOCK.finditer(block):
        namen |= {z.strip().lstrip("- ").strip() for z in treffer.group("block").splitlines() if z.strip()}
    return namen


def erreicht(start: str, kanten: dict[str, set[str]], ziele: set[str]) -> bool:
    gesehen: set[str] = set()
    offen = list(kanten.get(start, ()))
    while offen:
        name = offen.pop()
        if name in ziele:
            return True
        if name in gesehen:
            continue
        gesehen.add(name)
        offen.extend(kanten.get(name, ()))
    return False


def main() -> int:
    zerleger = argparse.ArgumentParser(description="Release haengt an den Pruefungen.")
    zerleger.add_argument("--wurzel", default=".", type=Path)
    argumente = zerleger.parse_args()
    wurzel = argumente.wurzel.resolve()

    pfad = wurzel / RELEASE
    try:
        text = pfad.read_text(encoding="utf-8")
    except OSError as fehler:
        raise SystemExit(f"release-abgesichert: {pfad} nicht lesbar: {fehler}")

    alle = jobs(text)
    if len(alle) < 2:
        # Ein Release-Workflow mit weniger als zwei Jobs kann kein Tor haben.
        # Wahrscheinlicher ist, dass sich die Form geaendert hat und die Muster
        # daneben greifen. Laut abbrechen statt gruen melden.
        raise SystemExit(
            f"release-abgesichert: in {RELEASE} nur {len(alle)} Job(s) erkannt "
            f"({', '.join(alle) or 'keiner'}). Entweder hat der Workflow seine Form "
            "geaendert, oder die Muster passen nicht mehr."
        )

    tore: dict[str, str] = {}
    befunde: list[str] = []
    for name, block in alle.items():
        treffer = RUFT_AUF.search(block)
        if not treffer:
            continue
        aufgerufen = treffer.group("pfad")
        tore[name] = aufgerufen
        # removeprefix, nicht lstrip: lstrip("./") entfernt JEDES fuehrende
        # "." und "/", macht aus "./.github/..." also "github/..." und meldet
        # eine Datei als fehlend, die es gibt. Beim ersten Lauf passiert.
        ziel = wurzel / aufgerufen.removeprefix("./")
        if not ziel.is_file():
            befunde.append(f"Job {name!r} ruft {aufgerufen} auf, die Datei gibt es nicht.")
        elif not re.search(r"^\s*workflow_call:\s*$", ziel.read_text(encoding="utf-8"), re.MULTILINE):
            befunde.append(
                f"Job {name!r} ruft {aufgerufen} auf, dort fehlt aber der Ausloeser "
                "`workflow_call:`. Der Aufruf scheitert erst zur Laufzeit."
            )

    kanten = {name: braucht(block) for name, block in alle.items()}
    ungedeckt: list[str] = []
    if not tore:
        befunde.append(
            "Kein Job ruft einen lokalen Workflow auf. Damit haengt nichts an den "
            "Pruefungen, und ein Tag veroeffentlicht ungeprueft."
        )
    else:
        for name in alle:
            if name in tore:
                continue
            if not erreicht(name, kanten, set(tore)):
                ungedeckt.append(name)
        for name in ungedeckt:
            befunde.append(
                f"Job {name!r} erreicht ueber `needs` kein Tor "
                f"({', '.join(sorted(tore))}). Er laeuft neben den Pruefungen."
            )

    print(
        f"release-abgesichert: {len(alle)} Job(s) in {RELEASE} geprueft "
        f"({', '.join(sorted(alle))}), Tor(e): {', '.join(sorted(tore)) or 'keins'}, "
        f"{len(befunde)} Befund(e)."
    )
    for befund in befunde:
        print(f"  {befund}")
    if befunde:
        print()
        print("Abhilfe: einen Job `uses: ./.github/workflows/pruefungen.yml` anlegen")
        print("und jeden bauenden oder veroeffentlichenden Job per `needs` daran haengen.")
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
