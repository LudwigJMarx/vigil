#!/usr/bin/env python3
"""Prueft, dass jeder Commit eines Beitrags eine Signed-off-by-Zeile traegt.

── WARUM ES DAS BRAUCHT ────────────────────────────────────────────────────

Apache-2.0 §5 regelt, unter welcher Lizenz ein eingereichter Beitrag steht.
Es regelt nicht, ob der Einreichende ueberhaupt das Recht hatte, ihn
einzureichen. Genau das erklaert das Developer Certificate of Origin, und die
Erklaerung besteht aus einer Zeile am Ende des Commits:

    Signed-off-by: Vorname Nachname <adresse@example.org>

`git commit -s` setzt sie. Der Wortlaut der Erklaerung steht in DCO im
Wurzelverzeichnis.

Ein Projekt, das ein DCO verlangt und es nicht prueft, hat kein DCO. Es hat
einen Absatz in CONTRIBUTING.md, den die Haelfte der Beitraege nicht erfuellt
und den niemand nachtraeglich einfordert, weil das unangenehm ist. Deshalb
prueft das die CI und nicht der Mensch im Review.

── WAS ES PRUEFT ───────────────────────────────────────────────────────────

Fuer jeden Commit im angegebenen Bereich:

  1. Es gibt mindestens eine Signed-off-by-Zeile.
  2. Mindestens eine davon nennt die Adresse des Autors des Commits. Sonst
     unterschreibt A fuer den Beitrag von B, und das ist keine Erklaerung
     ueber die eigene Arbeit mehr.

Zusammenfuehrungs-Commits (mehr als ein Elternteil) sind ausgenommen: die legt
GitHub an, nicht der Beitragende.

── WAS ES AUSDRUECKLICH NICHT SIEHT ────────────────────────────────────────

Ob die Erklaerung stimmt. Eine Signed-off-by-Zeile ist eine Zusicherung, keine
Pruefung. Das Werkzeug stellt sicher, dass sie abgegeben wurde.

Aufruf:  python3 scripts/pruefe-herkunftszeile.py --bereich <von>..<bis>
         python3 scripts/pruefe-herkunftszeile.py            # origin/main..HEAD
"""
from __future__ import annotations

import argparse
import re
import subprocess
import sys

TRENNER = "\x1e"
SIGNED_OFF = re.compile(r"^\s*Signed-off-by:\s*(?P<name>.*?)\s*<(?P<adresse>[^>]+)>\s*$",
                        re.IGNORECASE | re.MULTILINE)


def git(*argumente: str) -> str:
    lauf = subprocess.run(["git", *argumente], capture_output=True, text=True)
    if lauf.returncode != 0:
        # Laut abbrechen. Ein Pruefer, der git nicht befragen konnte, meldet
        # sonst "0 Befunde" wie einer, der alles geprueft hat.
        raise SystemExit(
            f"herkunftszeile: `git {' '.join(argumente)}` scheiterte "
            f"({lauf.returncode}):\n{lauf.stderr.strip()}"
        )
    return lauf.stdout


def commits(bereich: str) -> list[dict]:
    # %P ist leer beim Wurzel-Commit und enthaelt zwei Hashes bei einer
    # Zusammenfuehrung. %B ist die vollstaendige Nachricht inklusive Trailern.
    roh = git("log", "--format=%H%x1f%ae%x1f%s%x1f%P%x1f%B%x1e", bereich)
    ergebnis = []
    for block in roh.split(TRENNER):
        block = block.strip("\n")
        if not block:
            continue
        hash_, adresse, betreff, eltern, nachricht = block.split("\x1f", 4)
        ergebnis.append({
            "hash": hash_, "adresse": adresse, "betreff": betreff,
            "eltern": eltern.split(), "nachricht": nachricht,
        })
    return ergebnis


def pruefe(commit: dict) -> str | None:
    """Gibt den Grund zurueck, warum dieser Commit durchfaellt, oder None."""
    if len(commit["eltern"]) > 1:
        return None
    unterschriften = [t.group("adresse").lower()
                      for t in SIGNED_OFF.finditer(commit["nachricht"])]
    if not unterschriften:
        return "keine Signed-off-by-Zeile"
    if commit["adresse"].lower() not in unterschriften:
        return (f"unterschrieben von {', '.join(unterschriften)}, "
                f"Autor ist aber {commit['adresse']}")
    return None


def main() -> int:
    zerleger = argparse.ArgumentParser(description="DCO-Zeile je Commit.")
    zerleger.add_argument("--bereich", default="origin/main..HEAD",
                          help="git-Bereich, z. B. abc123..def456")
    argumente = zerleger.parse_args()

    geprueft = commits(argumente.bereich)
    if not geprueft:
        # Ein leerer Bereich ist kein sauberer Beitrag, sondern ein falsch
        # gesetzter Bereich. Die beiden duerfen nicht gleich aussehen.
        print(f"herkunftszeile: kein Commit im Bereich {argumente.bereich}.", file=sys.stderr)
        print("  Entweder zeigt der Bereich daneben, oder der Beitrag ist leer.",
              file=sys.stderr)
        return 1

    befunde = [(c, grund) for c in geprueft if (grund := pruefe(c))]
    zusammenfuehrungen = sum(1 for c in geprueft if len(c["eltern"]) > 1)

    print(
        f"herkunftszeile: {len(geprueft)} Commit(s) in {argumente.bereich} geprueft "
        f"({zusammenfuehrungen} Zusammenfuehrung(en) ausgenommen), {len(befunde)} Befund(e)."
    )
    for commit, grund in befunde:
        print(f"  {commit['hash'][:12]} {commit['betreff'][:60]}: {grund}")

    if befunde:
        print()
        print("Abhilfe fuer den letzten Commit:   git commit --amend -s")
        print("Fuer mehrere:                      git rebase --signoff <basis>")
        print("Der Wortlaut der Erklaerung steht in DCO.")
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
