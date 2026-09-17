#!/usr/bin/env python3
"""Prueft die systemd-Unit aus docs/deploy.md mit systemd-analyze verify.

── WARUM ES DAS BRAUCHT ────────────────────────────────────────────────────

In docs/deploy.md steht eine fertige systemd-Unit. Sie stand dort, weil sie
richtig aussah, nicht weil sie jemand geprueft haette. Ein Tippfehler in einer
Direktive (`ProtectKernelTunabels`) faellt beim Lesen nicht auf, und beim
Betreiber faellt er als "Unit laeuft, Haertung wirkt nicht" auf, also gar
nicht.

Die Doku dieses Projekts behauptet, jedes Beispiel sei gelaufen. Fuer diesen
Block war das bis zum 17.09.2026 nicht wahr. `systemd-analyze verify` macht es
wahr, kostet auf dem Laeufer Sekundenbruchteile und braucht keine Installation.

── WAS ES PRUEFT ───────────────────────────────────────────────────────────

Der erste als `ini` ausgezeichnete Block in docs/deploy.md wird als
`vigil.service` geschrieben und `systemd-analyze verify` vorgeworfen.

Zwei Meldungen werden ausgefiltert, weil sie nicht die Unit beschreiben,
sondern den Laeufer: der Benutzer `vigil` existiert dort nicht, und
`/usr/local/bin/vigil` auch nicht. Beides waere auf dem Zielrechner vorhanden.
Jede andere Zeile ist ein Befund.

── WAS ES AUSDRUECKLICH NICHT SIEHT ────────────────────────────────────────

Ob die Unit tut, was der Text daneben behauptet. `systemd-analyze` liest die
Datei, es startet nichts. Und auf einem Rechner ohne systemd (macOS) kann es
gar nichts pruefen; dann sagt es das und gibt 0 zurueck, denn die verbindliche
Pruefung ist die in der CI. Ein stiller Durchlauf waere hier schlimmer als gar
keiner.

Die Caddy- und nginx-Bloecke aus derselben Datei werden NICHT geprueft. Das ist
in docs/deploy.md benannt, nicht verschwiegen.

Aufruf:  python3 scripts/pruefe-betriebsanleitung.py [--wurzel PFAD]
"""
from __future__ import annotations

import argparse
import re
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

QUELLE = Path("docs/deploy.md")
BLOCK = re.compile(r"^```ini\s*$(?P<inhalt>.*?)^```\s*$", re.MULTILINE | re.DOTALL)

# Meldungen ueber die Umgebung des Laeufers, nicht ueber die Unit.
UMGEBUNG = (
    "Unknown user",
    "Unknown group",
    "is not executable",
    "No such file or directory",
    "Failed to prepare filename",
)


def unit(wurzel: Path) -> str:
    pfad = wurzel / QUELLE
    try:
        text = pfad.read_text(encoding="utf-8")
    except OSError as fehler:
        raise SystemExit(f"betriebsanleitung: {pfad} nicht lesbar: {fehler}")

    treffer = BLOCK.search(text)
    if not treffer:
        # Laut abbrechen: ohne Block gibt es nichts zu pruefen, und "0 Befunde"
        # waere dann die Antwort eines Pruefers, der nichts angesehen hat.
        raise SystemExit(
            f"betriebsanleitung: in {QUELLE} steht kein ```ini-Block. Entweder ist die "
            "Unit umgezogen, oder die Auszeichnung hat sich geaendert."
        )
    return treffer.group("inhalt").strip() + "\n"


def main() -> int:
    zerleger = argparse.ArgumentParser(description="systemd-Unit aus der Doku pruefen.")
    zerleger.add_argument("--wurzel", default=".", type=Path)
    argumente = zerleger.parse_args()
    wurzel = argumente.wurzel.resolve()

    inhalt = unit(wurzel)
    direktiven = sum(1 for z in inhalt.splitlines() if "=" in z)

    werkzeug = shutil.which("systemd-analyze")
    if werkzeug is None:
        print(
            f"betriebsanleitung: Unit aus {QUELLE} gelesen ({direktiven} Direktiven), "
            "aber systemd-analyze gibt es auf diesem Rechner nicht."
        )
        print("  Nicht geprueft. Die verbindliche Pruefung laeuft in der CI auf ubuntu.")
        return 0

    with tempfile.TemporaryDirectory() as tmp:
        datei = Path(tmp) / "vigil.service"
        datei.write_text(inhalt, encoding="utf-8")
        lauf = subprocess.run([werkzeug, "verify", str(datei)],
                              capture_output=True, text=True)

    zeilen = [z.strip() for z in (lauf.stdout + lauf.stderr).splitlines() if z.strip()]
    befunde = [z for z in zeilen if not any(m in z for m in UMGEBUNG)]

    print(
        f"betriebsanleitung: Unit aus {QUELLE} mit systemd-analyze geprueft "
        f"({direktiven} Direktiven, {len(zeilen)} Meldung(en), "
        f"{len(zeilen) - len(befunde)} davon ueber die Umgebung), {len(befunde)} Befund(e)."
    )
    for befund in befunde:
        print(f"  {befund}")
    if befunde:
        print()
        print("Abhilfe: die Unit in docs/deploy.md berichtigen.")
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
