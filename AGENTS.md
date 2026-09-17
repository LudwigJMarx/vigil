# Agenten-Regeln: vigil

Projektspezifische Fallen und Leitplanken: die Dinge, die **hier** beißen und
die man dem Quelltext nicht ansieht. Allgemeine gute Praxis wird vorausgesetzt,
nicht wiederholt. Diese Datei ist die Wahrheit; `CLAUDE.md` zeigt nur hierher.

> Beim Anlegen: alles in spitzen Klammern ersetzen, den Abschnitt „Beim Anlegen
> löschen" am Ende entfernen, und den Rest **kürzen statt füllen**. Eine Regel,
> die nur aus Vollständigkeit dasteht, verbraucht dieselbe Aufmerksamkeit wie
> eine, die schon einmal etwas verhindert hat.

## Was dieses Projekt ist

<Zwei bis vier Sätze. Wofür, für wen, und die eine Entscheidung, die alles
andere erklärt.>

## Regeln, die nicht verhandelbar sind

<Je Regel eine Überschrift im Aussagesatz, darunter der Vorfall, der sie
erzwungen hat, mit Datum. Eine Regel ohne Vorfall ist eine Meinung.>

### <Regel als Aussage, nicht als Verbot>

<Was am TT.MM.JJJJ passiert ist, warum die naheliegende Lösung nicht reicht,
und was stattdessen gilt.>

## Prüfer

Diese vier Sätze sind nicht projektspezifisch — sie sind teuer erlernt und
gelten überall. Alles Weitere in diesem Abschnitt ist es sehr wohl.

**Ein Prüfer, den niemand aufruft, ist eine Datei.** Ein Eintrag in
`package.json` oder `pyproject.toml`, den kein Workflow startet, läuft so oft
wie gar kein Eintrag. `scripts/pruefer-verdrahtet.py` hält das nach und ist
deshalb der erste Schritt in `.github/workflows/pruefungen.yml`.

**Ein Prüfer sagt, was er angesehen hat.** „0 Probleme" ist auch das, was einer
meldet, der nichts gefunden hat, weil er am falschen Ort gesucht hat. Jede
Ausgabe nennt den Umfang: *3 von 3 Dateien geprüft, 0 Befunde*. Eine leere
Antwort und ein gescheiterter Aufruf dürfen nie gleich aussehen.

**Handeln und Prüfen sind zwei Schritte.** Wer etwas umschreibt und im selben
Atemzug meldet, es sei umgeschrieben, prüft sich selbst. Der zweite Schritt
leitet die Antwort unabhängig aus dem Ergebnis ab. Ein Umschreiber, der still
nichts tut, erzeugt sonst denselben grünen Lauf wie einer, der funktioniert.

**Ein Ausweg wird genannt, nicht versteckt.** Wo eine Regel nicht gelten soll,
steht der Grund in der Datei (`@nicht-in-der-ci <Grund>`) und erscheint in der
Ausgabe des Prüfers. Eine Ausnahmeliste bekommt eine Obergrenze, die nur
sinken darf — sonst wächst sie.

<Ab hier die Prüfer dieses Projekts: was jeder prüft, was er ausdrücklich NICHT
sieht, und wo seine Ausnahmen stehen.>

## Tests

**Eine eingesetzte Attrappe verschiebt das Risiko, sie beseitigt es nicht.**
Wer eine Abhängigkeit einspritzt, um Logik testbar zu machen, hat die
ungeprüfte Stelle in die Naht verschoben. Die echte Implementierung braucht
dann einen eigenen Test — sonst ist die Attrappe richtig, die Wirklichkeit
falsch, und kein Test kann fehlschlagen.

**Nie auf Quelltext zusichern.** Ein Test, der eine Datei liest und ihren Text
prüft, bestätigt, dass eine Zeichenkette existiert, nicht dass der Code
funktioniert. Er bleibt grün, wenn der Aufruf durch eine leere Funktion ersetzt
wird.

**Die Verdrahtung wird mitgeprüft.** Ein Test, der eine Funktion direkt
aufruft, bleibt grün, wenn niemand diese Funktion mehr startet. Wenigstens ein
Test hält fest, dass der Bauablauf sie noch aufruft.

<Ab hier: Testwerkzeug dieses Projekts, wo Tests liegen, wie sie laufen, was
ein neuer Test mindestens zeigen muss.>

## Befehle

```bash
<einrichten>
<bauen>
<testen>
python3 scripts/pruefer-verdrahtet.py      # ruft die CI jeden Prüfer auf?
```

## Commits

`typ(bereich): betreff` — Betreff unter 72 Zeichen, kleingeschrieben. Der Body
erklärt die **Beweislage**: was behauptet wurde, was die Belege hergeben, warum
dieser Weg und welcher naheliegende nicht funktioniert. Keine Aufzählung der
geänderten Dateien; die steht im Diff.

Keine `Co-Authored-By`-Zeile für KI-Assistenten, kein Hinweis auf maschinelle
Mitarbeit — weder im Commit noch im PR-Text. Menschliche Mit-Autoren bekommen
die Zeile.

<Sprache festlegen: Deutsch bei privaten Projekten, Englisch bei öffentlichen.>

## Was nicht gebaut wird

<Die Liste der bewusst abgelehnten Dinge. Sie verhindert, dass dieselbe Idee
alle sechs Monate neu diskutiert wird — mit dem Grund der Ablehnung.>

---

## Beim Anlegen löschen

Diese Datei kommt aus `vorlagen/projekt/`. Mitgeliefert:

| Datei | Zweck |
|---|---|
| `scripts/pruefer-verdrahtet.py` | prüft, dass jeder Prüfer aufgerufen wird |
| `scripts/pruefer-verdrahtet.test.py` | dessen Test, inklusive Lauf gegen einen echten Baum |
| `.github/workflows/pruefungen.yml` | ruft beide auf |

Die Abschnitte „Prüfer" und „Tests" oben sind absichtlich vorausgefüllt: sie
sind Lehren aus fremden Repos, keine Erfindung. Wo eine davon für dieses
Projekt nicht gilt, wird sie gelöscht und der Grund in den Commit geschrieben.
