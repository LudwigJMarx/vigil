# Agenten-Regeln: vigil

Projektspezifische Fallen und Leitplanken: die Dinge, die **hier** beissen und
die man dem Quelltext nicht ansieht. Allgemeine gute Praxis wird vorausgesetzt,
nicht wiederholt. Diese Datei ist die Wahrheit; `CLAUDE.md` zeigt nur hierher.

## Was dieses Projekt ist

Eine quelloffene, selbst betriebene Signalschicht für langen B2B-Vertrieb: pro
Konto eine Zeitleiste, je Signal ein Gewicht und eine Halbwertszeit, daraus eine
Rangfolge. Die eine Entscheidung, aus der alles andere folgt: **vigil stellt
keine Anfrage an LinkedIn und verschickt keine Nachricht.** Es erinnert und
ordnet, was der Mensch selbst erfasst hat.

Zielgruppe ist nicht nur Ludwig. Das Repo ist öffentlich und soll von Fremden
betrieben werden können, ohne Rückfrage. Deshalb sind Quelltext, README und
alle Dateien unter `docs/` **englisch**; diese Datei, `CLAUDE.md`, die Prüfer
in `scripts/` und die Workflows sind deutsch, weil sie Hausregeln sind und
keine Lieferung.

## Regeln, die nicht verhandelbar sind

### Ein fehlender Zeitstempel wird abgelehnt, nicht durch `now` ersetzt

Ein drei Monate alter Beitrag, auf heute datiert, macht das kälteste Konto zum
heissesten der Liste, und hinterher sieht nichts daran falsch aus. `handleIngest`
weist jedes Signal ohne `occurred_at` mit Index und Grund zurück, und ebenso
jedes, das mehr als einen Tag in der Zukunft liegt.

Am 17.09.2026 hat dieselbe Regel in der Erweiterung gefehlt:
`parseRelativeTime` suchte das Muster irgendwo im markierten Text und las
"raised 12 m EUR" als zwölf Minuten. Die Funktion ist seitdem verankert
(`^…$`), und `draftFrom` probiert die Zeilen der Markierung einzeln. Zwei Tests
halten beides fest.

### Der Fingerabdruck enthält die Zeit nur, wenn es keine URL gibt

LinkedIn rendert "2d". Das löst bei jeder Aufnahme zu einem anderen Zeitpunkt
auf. Wäre die Zeit Teil der Identität, wäre jede zweite Aufnahme desselben
Beitrags ein neues Signal, und das Konto bekäme für ein Ereignis mehrfach
Gewicht. Mit URL identifizieren `source`, `kind` und die bereinigte URL das
Signal. Ohne URL zählen Zeit und Text mit, und zwei Notizen eine Minute
auseinander bleiben zwei Notizen.

### Eine Antwort nennt ihren Umfang

`/api/v1/accounts` liefert `signals_read` und `unscored_kinds`. `/healthz` zählt
Konten, Signale und Token statt "ok" zu melden. `ingest` antwortet mit
`received`, `inserted`, `duplicate` und einer benannten Liste `rejected`.

Der Grund ist immer derselbe: eine leere Antwort und ein gescheiterter Aufruf
dürfen nie gleich aussehen. Ein Konto ohne Signale und ein Konto, dessen
Signalarten das Modell nicht kennt, sind verschiedene Probleme.

### Der Punktestand liest immer die ganze Zeitleiste

`?limit=` schneidet nur die zurückgegebene Liste. Wer die Seite bepunktet,
liefert eine Zahl, die davon abhängt, wie danach gefragt wurde. Ein Test in
`internal/api/api_test.go` besteht darauf, dass `limit=2` und kein Limit
denselben Punktestand ergeben.

Ebenso liest die Kontoliste **alle** Signale, nicht ein Zeitfenster: eine Regel
mit Halbwertszeit 0 verfällt nicht, ein Fenster würde also Gewicht
wegschneiden, das der Betreiber ausdrücklich als dauerhaft eingestellt hat.

### Die Vorgaben werden nie in die Datenbank kopiert

`core.DefaultRules()` wird bei jedem `store.Rules()` frisch mit den
gespeicherten Abweichungen überlagert. Eine Vorgabe, die beim Einrichten in
eine Tabelle geschrieben wurde, ist keine Vorgabe mehr: sie friert auf dem Wert
der Version ein, die sie geschrieben hat, und ein Betreiber, der nie eine Regel
angefasst hat, behält still ein Modell, über das spätere Fassungen hinweg
sind.

### Der partielle Index ist Absicht

`CREATE UNIQUE INDEX accounts_profile ON accounts(profile) WHERE profile <> ''`.
Ohne `WHERE` dürfte genau **ein** Konto ohne LinkedIn-Seite existieren. Dasselbe
gilt für `people_profile`. Zwei Tests in `internal/store/store_test.go` halten
das fest.

### `foreign_keys` ist eine Verbindungs-Einstellung

In SQLite ist `PRAGMA foreign_keys` pro Verbindung und standardmäßig **aus**.
Wer ihn vergisst, bekommt keine Fehlermeldung, sondern verwaiste Zeilen, die
nie auffallen. Der Pragma steht in `store.Open`, und ein Test speichert ein
Signal gegen ein Konto, das es nicht gibt, und verlangt, dass das scheitert.

### `:8099` ist keine Loopback-Adresse

Der leere Rechnername bedeutet für Gos Server **alle** Schnittstellen. Die
unbedachteste Bindung ist damit die kürzeste Schreibweise. `loopback()` in
`internal/cli/cli.go` behandelt `""` ausdrücklich als öffentlich; die
Tabelle im Test nennt `:8099`, `0.0.0.0:8099` und `[::]:8099` beim Namen.

### Die Erweiterung darf keinen Sonderfall bekommen

Kein `host_permissions`, kein Service Worker, kein `content_scripts` mit
`matches`, kein Alarm, kein `setInterval`, kein `fetch` gegen linkedin.com.
`scripts/pruefe-keine-hintergrundabfrage.py` setzt das in der CI durch. Wer eine
Ausnahme braucht, ändert zuerst `docs/entscheidungen/0002-kein-scraping.md` und
begründet sie dort.

## Prüfer

Diese vier Sätze sind nicht projektspezifisch, sie sind teuer erlernt und
gelten überall.

**Ein Prüfer, den niemand aufruft, ist eine Datei.** Ein Eintrag in
`package.json` oder `pyproject.toml`, den kein Workflow startet, läuft so oft
wie gar kein Eintrag. `scripts/pruefer-verdrahtet.py` hält das nach und ist
deshalb der erste Schritt in `.github/workflows/pruefungen.yml`.

**Ein Prüfer sagt, was er angesehen hat.** "0 Probleme" ist auch das, was einer
meldet, der am falschen Ort gesucht hat. Jede Ausgabe nennt den Umfang. Eine
leere Antwort und ein gescheiterter Aufruf dürfen nie gleich aussehen.

**Handeln und Prüfen sind zwei Schritte.** Wer etwas umschreibt und im selben
Atemzug meldet, es sei umgeschrieben, prueft sich selbst.

**Ein Ausweg wird genannt, nicht versteckt.** `@nicht-in-der-ci <Grund>` in den
ersten 40 Zeilen einer Datei, und der Grund erscheint in der Ausgabe.

Die Prüfer dieses Projekts:

| Prüfer | Prüft | Sieht ausdrücklich nicht |
|---|---|---|
| `pruefer-verdrahtet.py` | jeder Prüfer unter `scripts/` wird von einem Workflow erreicht | Prüfer, die der Namenskonvention nicht folgen |
| `pruefe-keine-hintergrundabfrage.py` | `extension/manifest.json` und `extension/src/*.ts` auf Hintergrundarbeit | ausgeführtes Verhalten. Ein zusammengesetzter Aufruf kommt vorbei |
| `pruefe-doku-befehle.py` | jeder `vigil …`-Aufruf in einem `bash`-Block existiert laut `vigil help` | ob Schalter, Ausgabe oder Beschreibung stimmen |

`scripts/pruefer.test.py` testet die beiden projekteigenen Prüfer. Beide
Testklassen bauen Bäume, in denen es etwas zu finden gibt, **und** laufen
zusätzlich einmal gegen dieses Verzeichnis, damit nicht nur die Attrappe
geprüft ist.

Am 17.09.2026 hat genau das zweimal gegriffen, bevor die Doku es tat:
`vigil token rotate` ging durch, weil "vigil token" als Zeichenkette im
Hilfetext vorkommt; und `vigil 0.1.0 listening on …` aus einem Ausgabeblock der
README wurde für einen Aufruf des Befehls "0.1.0" gehalten. Beide Fälle hängen
jetzt als Test an dem Prüfer.

## Tests

**Eine eingesetzte Attrappe verschiebt das Risiko, sie beseitigt es nicht.**
`pruefe-doku-befehle.py` hat `--hilfetext` genau für seine Tests. Deshalb
läuft derselbe Prüfer im Test zusätzlich ohne den Schalter gegen den echten
Baum, mit echtem `go run`.

**Nie auf Quelltext zusichern.** Kein Test liest eine Datei und prueft ihren
Text. Die einzige Ausnahme ist `pruefe-keine-hintergrundabfrage.py` selbst, und
die ist der Zweck des Werkzeugs, nicht sein Test.

**Die Verdrahtung wird mitgeprüft.** `cmd/vigil/main_test.go` baut das echte
Binary, legt ein Token an, startet den Server, liest die Adresse aus seiner
Ausgabe, holt `/healthz`, holt die eingebettete Oberfläche und schickt eine
Aufnahme mit dem Token, das die Kommandozeile ausgegeben hat. Jeder andere Test
im Repo bliebe grün, wenn `main` aufhörte, einen Server zu starten.

**Die Tabelle, die beide Seiten lesen.** `testdata/profile-normalisation.json`
wird von `internal/core/shared_table_test.go` und von
`extension/test/guess.test.ts` gelesen. Go und TypeScript bilden dieselbe Regel
ab; ohne die geteilte Tabelle driften sie, und die Drift ist unsichtbar, weil
der Server dann einfach zwei Konten für eine Firma hält.

Ein neuer Test muss zeigen, dass er fehlschlagen kann. Wer einen Test
hinzufügt, macht ihn einmal absichtlich rot.

## Befehle

```bash
make build                                 # ./vigil, statisch, ohne cgo
make test                                  # go test -race plus die Tests der Erweiterung
make check                                 # alles, was die CI tut, in derselben Reihenfolge
python3 scripts/pruefer-verdrahtet.py      # ruft die CI jeden Pruefer auf?
```

## Commits

`typ(bereich): betreff` auf **Englisch**, Betreff unter 72 Zeichen,
kleingeschrieben. Der Body erklärt die Beweislage: was behauptet wurde, was die
Belege hergeben, warum dieser Weg und welcher naheliegende nicht funktioniert.
Keine Aufzählung der geänderten Dateien; die steht im Diff.

Keine `Co-Authored-By`-Zeile für KI-Assistenten und kein Hinweis auf
maschinelle Mitarbeit, weder im Commit noch im PR-Text. Menschliche Mit-Autoren
bekommen die Zeile.

In Commit-Nachrichten ohne Umlaute und ohne Eszett schreiben.

## Was nicht gebaut wird

| Abgelehnt | Grund |
|---|---|
| Nachrichten verschicken | Der Kern des Produkts. `docs/entscheidungen/0002-kein-scraping.md` |
| Anfragen an LinkedIn | Dieselbe Quelle. Das Risiko trägt das Konto des Nutzers, nicht das des Autors |
| Docker als Hauptweg | Verschiebt die Abhängigkeit, entfernt sie nicht. Auf dieser Maschine gibt es kein Docker, also wäre es ein ungetesteter Installationsweg als empfohlener |
| Postgres | Ein zweiter Prozess für ein paar Megabyte |
| `mattn/go-sqlite3` | Braucht cgo und damit eine C-Werkzeugkette je Ziel. Das kostet die ganze "eine Datei herunterladen"-Geschichte |
| TLS im Server | Löst jeder Reverse Proxy besser. `docs/deploy.md` zeigt einen |
| DOM-Auswertung der LinkedIn-Seiten | Generiertes Markup. Ein Selektor, der still nicht mehr passt, macht ein lautes Konto zu einem leisen |
| Telemetrie | vigil stellt keine Anfrage, die der Betreiber nicht ausgelöst hat |
