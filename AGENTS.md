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

Lizenz: Apache-2.0. Rechteinhaber ist „The vigil Authors", nicht eine Person,
damit die Angabe mitwächst und eine spätere Umlizenzierung ohne die Beitragenden
nicht aus Versehen möglich aussieht.

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

### Was eine URL wert ist, hängt davon ab, worauf sie zeigt

LinkedIn rendert "2d". Das löst bei jeder Aufnahme zu einem anderen Zeitpunkt
auf. Die Zeit gehört deshalb nie zur Identität, solange es eine URL gibt: sonst
wäre jede zweite Aufnahme desselben Beitrags ein neues Signal.

Drei Fälle, und der mittlere war bis zum 17.09.2026 falsch:

| URL | Identität |
|---|---|
| Dauerlink (`/feed/update/…`, `/posts/…`) | `source`, `kind`, URL. Der Link benennt genau ein Element |
| Profil- oder Firmenseite (`/in/`, `/company/`, …) | zusätzlich Titel und Text. Die Seite benennt einen **Ort**, keine Beobachtung |
| keine | `source`, `kind`, Konto, Person, Zeit, Titel, Text |

Vorher entschied auch bei der Firmenseite die URL allein. Da jede Erfassung von
dort dieselbe URL trägt, kam die zweite als „already known" zurück und war weg.
Stiller Verlust im Kostüm richtiger Entdoppelung, und das ist die schlimmste
Form, die ein Fehler hier annehmen kann: von aussen sehen beide gleich aus.

Gefunden, indem das Popup der Erweiterung gegen eine laufende Instanz gefahren
wurde, nicht von einem Test. Jetzt hängen drei Tests in `internal/core` und
zwei in `internal/api` daran, letztere durch den Ingest-Endpunkt, weil dort der
Verlust auftrat. Die Unterscheidung nutzt `NormalizeProfile`: sie antwortet
genau bei den Ortsseiten nicht-leer, also gibt es eine Regel dafür und keine
zweite Liste.

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

### Eine neue Abhängigkeit bringt ihren Lizenztext mit

MIT und BSD-3-Clause verlangen beide, dass Urheberrechtsvermerk und Lizenztext
der Weitergabe **in Binärform** beiliegen. Zehn fremde Module landen im Binary.
Ein Release-Archiv ohne `THIRD-PARTY-NOTICES.md` ist ein Verstoß gegen zehn
Lizenzen, und zwar einer ohne Fehlermeldung, ohne roten Lauf und ohne
Beschwerde: er wird einfach mit jedem Download weitergegeben.

`make lizenzen` erzeugt die Datei aus `go list -deps ./cmd/vigil`, nicht aus
einer gepflegten Aufzählung. Wer eine Abhängigkeit hinzufügt und die Datei
nicht mitliefert, bekommt einen roten Lauf.

Am 17.09.2026 hielt die Erkennung drei der vier `modernc.org`-Module für
unbekannt, weil sie nach `name of` suchte und deren dritte Klausel
„Neither the names of the authors" lautet. Ein Etikett, das bei jedem vierten
Modul danebenliegt, ist schlechter als keines: es sieht aus, als hätte jemand
nachgesehen.

### Der Herkunftsnachweis wird geprüft, nicht erbeten

Apache-2.0 §5 regelt die Lizenz eines Beitrags, nicht das Recht, ihn
einzureichen. Dafür gibt es das DCO. Ein Projekt, das ein DCO verlangt und es
nicht prüft, hat keins: es hat einen Absatz, den die Hälfte der Beiträge nicht
erfüllt und den niemand nachträglich einfordert, weil das unangenehm ist.

`pruefe-herkunftszeile.py` läuft in der CI nur bei `pull_request`, weil der
Bereich auf `main` leer wäre. Ein leerer Bereich ist dort mit Absicht ein
Fehler: sonst sähe ein falsch gesetzter Bereich aus wie ein sauberer Beitrag.

Die Unterschrift muss die Adresse des **Autors** des Commits nennen. Sonst
unterschreibt A für die Arbeit von B, und die Erklärung ist keine Erklärung
über die eigene Arbeit mehr.

### Ein veröffentlichtes Release ist unveränderlich

Am 17.09.2026 hat ein Force-Push des Tags `v0.1.0` den Release-Workflow erneut
gestartet und alle fünf Archive still ersetzt. Der Quelltext war derselbe, die
Prüfsummen waren es nicht: Go stempelt `vcs.revision` ins Binary, und die SHA
hatte sich durch das Umschreiben des Verlaufs geändert. `vcs.time` und alles
andere blieben gleich, nachgewiesen mit `go version -m` an beiden Dateien.

Wer die alte Prüfsumme notiert hatte, sieht seitdem eine Abweichung, und eine
Abweichung an einer veröffentlichten Datei sieht aus wie Manipulation.

`overwrite_files: false` lässt den Lauf laut scheitern, statt still zu
ersetzen. `pruefe-release-abgesichert.py` hält das fest und bricht ab, wenn es
den Release-Schritt gar nicht mehr findet.

Daraus folgt eine Regel für den Umgang: ein Tag wird nicht verschoben. Wer
etwas ändern muss, vergibt eine neue Version.

### Ein Binary ohne Stempel meldet seine Modulversion, nicht „dev"

`go install github.com/LudwigJMarx/vigil/cmd/vigil@latest` läuft ohne die
`-ldflags` des Release-Workflows. Bis zum 17.09.2026 meldete ein so
installiertes Release deshalb `vigil dev`, und `/healthz` antwortete `"dev"`.
Genau der Fall, vor dem der Kommentar im Release-Workflow warnt, nur von der
anderen Seite: jeder Fehlerbericht aus einer solchen Instanz nennt keine
Version.

`versionFrom` entscheidet in dieser Reihenfolge: Stempel, dann
`debug.ReadBuildInfo().Main.Version`, dann `dev`. Der Stempel schreibt in
`internal/cli.stamped`, nicht mehr in `Version`, damit die Entscheidung eine
reine Funktion bleibt und einen Test hat, der nicht drei Bauarten braucht.

Nachgewiesen ist bisher nur die Funktion und der ungestempelte Bau. Dass
`go install …@vX` wirklich die Version meldet, zeigt erst das nächste Tag.

### `.gitignore` wirkt nicht rückwirkend

Am 17.09.2026 lag das gebaute 10-MB-Binary `vigil` seit dem Wurzel-Commit im
Repo, öffentlich, bei einer Gesamtgröße von 4,4 MB auf GitHub. Hineingeraten
über ein `git add -A` während eines `git rebase --root`: der Baum trug zu dem
Zeitpunkt noch die `.gitignore` der Projektvorlage, in der `/vigil` fehlte.

Aufgefallen ist es Stunden später, und der Grund ist die eigentliche Lehre:
`.gitignore` gilt nur für **unverfolgte** Dateien. Einmal verfolgt, verschwindet
eine Datei aus `git status`, obwohl sie ignoriert ist. Ein Blick auf
„Arbeitsbaum sauber" kann „nichts hinzuzufügen" und „längst verschluckt" nicht
unterscheiden. Genau die Sorte Prüfung, vor der die Hausregeln warnen, und sie
stand in der eigenen Kontrolle.

`git ls-files --cached --ignored --exclude-standard` zeigt es, und
`pruefe-keine-bauartefakte.py` ruft das in der CI auf. Wer nach einem
`rebase --exec ... git add -A` weiterarbeitet, prüft zusätzlich `git show --stat`
auf den umgeschriebenen Commits.

### Ein Tag veröffentlicht nichts Ungeprüftes

`Pruefungen` läuft bei Push auf main und bei jedem Beitrag, **nicht** bei einem
Tag. Am 17.09.2026 war v0.1.0 nur deshalb gedeckt, weil der getaggte Commit
vorher über main gelaufen war. Ein Tag auf einen Commit, der das nicht war,
wäre ohne gofmt, ohne die Tests der Erweiterung und ohne den Lizenz-Prüfer
ausgeliefert worden, und der Release-Lauf wäre grün gewesen, weil er nur
`go test` kannte.

Die Prüfungen zusätzlich auf Tags laufen zu lassen behebt das nicht: sie liefen
dann **neben** dem Release und hielten nichts auf. `veroeffentlichen.yml` ruft
deshalb `pruefungen.yml` über `workflow_call` auf, und `bauen` hängt per `needs`
an dessen Ergebnis. `pruefe-release-abgesichert.py` hält fest, dass kein Job an
diesem Tor vorbeikommt.

Der Prüfer hat sich beim ersten Lauf selbst erwischt: `lstrip("./")` entfernt
jedes führende `.` und `/`, macht aus `./.github/workflows/…` also
`github/workflows/…` und meldete eine vorhandene Datei als fehlend. Jetzt
`removeprefix`, mit einem Test darauf.

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
| `pruefe-lizenzhinweise.py` | `THIRD-PARTY-NOTICES.md` deckt genau die Module ab, die `go list -deps ./cmd/vigil` meldet | ob die Lizenzen miteinander verträglich sind. Es sammelt, es beurteilt nicht |
| `pruefe-herkunftszeile.py` | jeder Commit eines Beitrags trägt `Signed-off-by` mit der Adresse seines Autors | ob die Zusicherung stimmt. Eine Erklärung ist keine Prüfung |
| `pruefe-release-abgesichert.py` | jeder Job in `veroeffentlichen.yml` erreicht über `needs` den Job, der `pruefungen.yml` aufruft | ob die Prüfungen selbst etwas taugen. Dafür gibt es `pruefer-verdrahtet.py` |
| `pruefe-keine-bauartefakte.py` | keine Datei ist verfolgt und zugleich von `.gitignore` erfasst | ein Bauartefakt, das keine Regel erfasst. Er hält den Widerspruch fest, nicht jede Unordnung |

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
make check                                 # alles, was die CI tut, ausser dem DCO-Schritt
make lizenzen                              # THIRD-PARTY-NOTICES.md nach einer neuen Abhaengigkeit
python3 scripts/pruefer-verdrahtet.py      # ruft die CI jeden Pruefer auf?
```

`make check` lässt den DCO-Schritt aus: der braucht eine Basis zum Vergleichen
und die gibt es erst, wenn der Beitrag existiert.

## Commits

`typ(bereich): betreff` auf **Englisch**, Betreff unter 72 Zeichen,
kleingeschrieben. Der Body erklärt die Beweislage: was behauptet wurde, was die
Belege hergeben, warum dieser Weg und welcher naheliegende nicht funktioniert.
Keine Aufzählung der geänderten Dateien; die steht im Diff.

Jeder Commit trägt `Signed-off-by` mit der Adresse seines Autors, gesetzt von
`git commit -s`. Der Wortlaut der Erklärung steht in `DCO`.

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
| CLA oder Rechteübertragung | Apache-2.0 §5 plus DCO deckt dasselbe ab, ohne Unterschrift, Bot oder Konto. Eine CLA wäre nur nötig, um später umzulizenzieren, und genau das soll nicht möglich sein, ohne die Beitragenden zu fragen |
| AGPL | Sperrlisten in genau den Firmen, deren Vertrieb vigil benutzen soll. Dazu die offene Frage, wie weit §13 auf die Erweiterung reicht, die über HTTP mit dem Server redet. Auslegbarkeit beantwortet eine Rechtsabteilung mit „nein" |
