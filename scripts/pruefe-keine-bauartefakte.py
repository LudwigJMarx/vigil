#!/usr/bin/env python3
"""Prueft, dass keine verfolgte Datei zugleich in .gitignore steht.

── WARUM ES DAS BRAUCHT ────────────────────────────────────────────────────

Am 17.09.2026 lag das gebaute 10-MB-Binary `vigil` seit dem Wurzel-Commit im
Repo, oeffentlich. Hineingeraten ist es durch ein `git add -A` waehrend eines
`git rebase --root`: zu dem Zeitpunkt trug der Baum noch die .gitignore der
Projektvorlage, in der `/vigil` fehlte, und das Binary lag daneben.

Aufgefallen ist es erst Tage spaeter, und das ist der Punkt. `.gitignore`
wirkt nur auf UNVERFOLGTE Dateien. Ist eine Datei einmal verfolgt, verschwindet
sie aus `git status`, obwohl sie ignoriert ist. Ein Blick auf "Arbeitsbaum
sauber" kann "nichts hinzuzufuegen" und "laengst verschluckt" nicht
unterscheiden.

Git kann es: `git ls-files --cached --ignored --exclude-standard` listet genau
die Dateien, die beides sind. Der Befehl ist selten bekannt, deshalb steht er
hier in einem Pruefer und nicht in einer Doku.

── WAS ES PRUEFT ───────────────────────────────────────────────────────────

Eine Datei, die verfolgt wird UND auf eine Regel in .gitignore passt. Das ist
immer ein Widerspruch: entweder gehoert sie ins Repo, dann muss die Regel weg,
oder sie gehoert nicht hinein, dann muss sie aus der Verfolgung.

── WAS ES AUSDRUECKLICH NICHT SIEHT ────────────────────────────────────────

Ein Bauartefakt, das in KEINER .gitignore-Regel steht. Wer eine neue Ausgabe
an einen Ort legt, den niemand ignoriert, faellt hier nicht auf. Der Pruefer
haelt den Widerspruch fest, nicht jede denkbare Unordnung.

Aufruf:  python3 scripts/pruefe-keine-bauartefakte.py [--wurzel PFAD]
"""
from __future__ import annotations

import argparse
import subprocess
import sys
from pathlib import Path


def git(wurzel: Path, *argumente: str) -> str:
    lauf = subprocess.run(["git", "-C", str(wurzel), *argumente],
                          capture_output=True, text=True)
    if lauf.returncode != 0:
        # Laut abbrechen. Ein Pruefer, der git nicht befragen konnte, meldet
        # sonst "0 Befunde" wie einer, der alles gelesen hat.
        raise SystemExit(
            f"keine-bauartefakte: `git {' '.join(argumente)}` scheiterte "
            f"({lauf.returncode}):\n{lauf.stderr.strip()}"
        )
    return lauf.stdout


def main() -> int:
    zerleger = argparse.ArgumentParser(description="Verfolgt und ignoriert zugleich.")
    zerleger.add_argument("--wurzel", default=".", type=Path)
    argumente = zerleger.parse_args()
    wurzel = argumente.wurzel.resolve()

    verfolgt = [z for z in git(wurzel, "ls-files").splitlines() if z.strip()]
    if not verfolgt:
        raise SystemExit(
            f"keine-bauartefakte: unter {wurzel} verfolgt git keine einzige Datei. "
            "Entweder ist das kein Repository, oder --wurzel zeigt daneben."
        )

    befunde = [
        z for z in git(wurzel, "ls-files", "--cached", "--ignored",
                       "--exclude-standard").splitlines() if z.strip()
    ]

    print(
        f"keine-bauartefakte: {len(verfolgt)} verfolgte Datei(en) gegen .gitignore "
        f"gehalten, {len(befunde)} Befund(e)."
    )
    for pfad in befunde:
        groesse = ""
        datei = wurzel / pfad
        if datei.is_file():
            groesse = f" ({datei.stat().st_size // 1024} KiB)"
        print(f"  verfolgt und zugleich ignoriert: {pfad}{groesse}")

    if befunde:
        print()
        print("Entweder die Datei gehoert ins Repo, dann muss die Regel in .gitignore weg,")
        print("oder sie gehoert nicht hinein:  git rm --cached <pfad>")
        print("Achtung: das entfernt sie nur ab jetzt. Im Verlauf bleibt sie liegen.")
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
