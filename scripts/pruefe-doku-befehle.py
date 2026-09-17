#!/usr/bin/env python3
"""Prueft, dass jeder in der Doku gezeigte vigil-Befehl den es gibt.

── WARUM ES DAS BRAUCHT ────────────────────────────────────────────────────

Der erste Befehl, den ein Fremder aus der README abtippt, entscheidet, ob er
weiterliest. Ein Befehl, der mit "unknown command" antwortet, kostet das
Projekt diesen Leser, und er faellt niemandem auf, der das Projekt kennt: wer
es kennt, tippt den richtigen Befehl aus dem Gedaechtnis.

Umbenennungen sind der Normalfall, nicht die Ausnahme. Also prueft das ein
Skript statt eines Vorsatzes.

── WAS ES PRUEFT ───────────────────────────────────────────────────────────

Jede Zeile in einem als `bash`, `sh`, `shell`, `zsh` oder `console`
ausgezeichneten Codeblock einer .md-Datei, die mit `vigil ` beginnt, wird gegen die Ausgabe von `go run ./cmd/vigil help` gehalten. Aus
deren Usage-Block entsteht die Liste der Befehle und, wo es sie gibt, ihrer
Unterbefehle. Hat ein Befehl Unterbefehle, muss der in der Doku gezeigte einer
davon sein.

Die Befehlsliste kommt also aus dem gebauten Programm, nicht aus einer zweiten
Liste im Skript. Eine zweite Liste waere genau die Sorte Abschrift, die hier
verhindert werden soll.

Zwei Fassungen davor sind an der Wirklichkeit gescheitert, und beide Faelle
haengen jetzt als Test daran:

1. Der Befehl wurde nur als Zeichenkette im Hilfetext gesucht. Damit ging
   `vigil token rotate` durch, weil "vigil token" darin vorkommt.
2. Jeder Codeblock wurde gelesen, auch die ohne Sprachangabe. Die enthalten
   die AUSGABE, und die faengt mit `vigil 0.1.0 listening on ...` an. Der
   Pruefer hielt die Versionsnummer fuer einen Befehl.

── WAS ES AUSDRUECKLICH NICHT SIEHT ────────────────────────────────────────

Ob die Schalter stimmen, ob die Ausgabe stimmt, ob der Befehl tut was der Text
daneben behauptet. Es prueft den Namen, sonst nichts.

Aufruf:  python3 scripts/pruefe-doku-befehle.py [--wurzel PFAD] [--hilfetext DATEI]

`--hilfetext` ersetzt den Aufruf des Programms durch eine Datei. Es gibt den
Schalter, damit die Tests einen erfundenen Hilfetext gegen einen erfundenen
Doku-Baum halten koennen. Weil eine solche Attrappe das Risiko nur in die Naht
verschiebt, laeuft derselbe Pruefer im Test zusaetzlich einmal ohne Schalter
gegen dieses Verzeichnis.
"""
from __future__ import annotations

import argparse
import re
import subprocess
import sys
from pathlib import Path

UEBERSPRINGEN = {"node_modules", "dist", "build", ".git", ".venv", "__pycache__", "target"}
ZAUN = re.compile(r"^\s*```+\s*(?P<sprache>[A-Za-z0-9_+-]*)")
BEFEHLSSPRACHEN = {"bash", "sh", "shell", "zsh", "console"}
BEFEHL = re.compile(r"^\s*(?:\$\s*)?(?:\./)?vigil\s+(?P<rest>[^\n]*)$")


def markdown_dateien(wurzel: Path) -> list[Path]:
    return sorted(
        p for p in wurzel.rglob("*.md")
        if p.is_file() and not UEBERSPRINGEN & set(p.relative_to(wurzel).parts)
    )


def befehle_aus(datei: Path) -> list[tuple[int, str]]:
    """Gibt (Zeilennummer, Argumentteil) je vigil-Aufruf im Codeblock."""
    gefunden: list[tuple[int, str]] = []
    sprache: str | None = None
    for nummer, zeile in enumerate(datei.read_text(encoding="utf-8").splitlines(), start=1):
        zaun = ZAUN.match(zeile)
        if zaun:
            # Ein Block ohne Sprachangabe zeigt die Ausgabe, nicht den Aufruf.
            sprache = None if sprache is not None else zaun.group("sprache").lower()
            continue
        if sprache not in BEFEHLSSPRACHEN:
            continue
        treffer = BEFEHL.match(zeile)
        if treffer:
            gefunden.append((nummer, treffer.group("rest").strip()))
    return gefunden


def befehlsbaum(hilfe: str) -> dict[str, set[str]]:
    """Liest aus dem Hilfetext, welche Befehle es gibt und welche Unterbefehle.

    Eine Usage-Zeile heisst `  vigil <befehl> [<unterbefehl>] ...`. Als
    Unterbefehl zaehlt nur ein kleingeschriebenes Wort: `NAME`, `ID`, `PATH`
    und alles in eckigen Klammern sind Platzhalter fuer Werte, keine Befehle.
    """
    baum: dict[str, set[str]] = {}
    for zeile in hilfe.splitlines():
        teile = zeile.split()
        if len(teile) < 2 or teile[0] != "vigil":
            continue
        befehl = teile[1]
        if not re.fullmatch(r"[a-z][a-z-]*", befehl):
            continue
        baum.setdefault(befehl, set())
        if len(teile) > 2 and re.fullmatch(r"[a-z][a-z-]*", teile[2]):
            baum[befehl].add(teile[2])
    return baum


def hilfetext(wurzel: Path) -> str:
    lauf = subprocess.run(
        ["go", "run", "./cmd/vigil", "help"],
        cwd=wurzel, capture_output=True, text=True,
    )
    if lauf.returncode != 0:
        # Laut abbrechen. Ein Pruefer, der den Hilfetext nicht bekommen hat,
        # meldet sonst "0 Befunde" wie einer, der alles geprueft hat.
        raise SystemExit(
            f"doku-befehle: `go run ./cmd/vigil help` scheiterte ({lauf.returncode}):\n"
            f"{lauf.stderr.strip()}"
        )
    return lauf.stdout


def main() -> int:
    zerleger = argparse.ArgumentParser(description="Doku-Befehle gegen das Programm halten.")
    zerleger.add_argument("--wurzel", default=".", type=Path)
    zerleger.add_argument("--hilfetext", type=Path, default=None,
                          help="Hilfetext aus dieser Datei statt aus dem Programm (nur fuer Tests)")
    argumente = zerleger.parse_args()
    wurzel = argumente.wurzel.resolve()

    dateien = markdown_dateien(wurzel)
    if argumente.hilfetext is not None:
        hilfe = argumente.hilfetext.read_text(encoding="utf-8")
        quelle = str(argumente.hilfetext)
    else:
        hilfe = hilfetext(wurzel)
        quelle = "go run ./cmd/vigil help"

    unterbefehle = befehlsbaum(hilfe)
    if not unterbefehle:
        raise SystemExit("doku-befehle: der Hilfetext nennt keinen einzigen Befehl.")

    befunde: list[str] = []
    geprueft = 0
    for datei in dateien:
        for nummer, rest in befehle_aus(datei):
            teile = [t for t in rest.split() if not t.startswith("-")]
            if not teile:
                continue
            geprueft += 1
            ort = f"{datei.relative_to(wurzel).as_posix()}:{nummer}"
            befehl = teile[0]
            if befehl not in unterbefehle:
                befunde.append(f"{ort}: `vigil {rest}` - {befehl!r} ist kein Befehl")
                continue
            erwartete = unterbefehle[befehl]
            if not erwartete:
                continue
            if len(teile) < 2 or teile[1] not in erwartete:
                gezeigt = teile[1] if len(teile) > 1 else "(keiner)"
                befunde.append(
                    f"{ort}: `vigil {rest}` - {gezeigt!r} ist kein Unterbefehl von "
                    f"{befehl!r} (erlaubt: {', '.join(sorted(erwartete))})"
                )

    print(
        f"doku-befehle: {len(dateien)} Markdown-Datei(en) gelesen, "
        f"{geprueft} vigil-Aufruf(e) gegen {quelle} geprueft, {len(befunde)} Befund(e)."
    )
    if geprueft == 0:
        print("  Achtung: kein einziger vigil-Aufruf in der Doku gefunden. "
              "Entweder zeigt die Doku keine Befehle, oder das Muster passt nicht mehr.")

    for befund in befunde:
        print(f"  {befund}")
    if befunde:
        print()
        print("Entweder die Doku nachziehen, oder den Befehl in internal/cli/cli.go anlegen.")
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
