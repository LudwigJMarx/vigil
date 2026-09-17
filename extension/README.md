# vigil capture

A Chrome extension that sends what **you** are looking at to **your** vigil
instance. It is deliberately small, and most of what it does not do is the
point.

## What it does

1. You open a LinkedIn page yourself and read it.
2. You select the text that matters (optional) and click the vigil icon.
3. A form appears, pre-filled from the page URL, the page title and your
   selection. You choose the signal kind and correct the date.
4. You press Send. The signal goes to the vigil server you configured.

## What it does not do

| Not built | Why |
|---|---|
| Background crawling | It holds no host permissions and registers no alarms. It can only read the tab you are on, in the moment you click the icon. |
| Reading the feed for you | LinkedIn's terms forbid automated collection. An extension that walks the feed gets the *user's* account restricted, not the author's. |
| Sending messages | vigil never writes on your behalf. That is the whole premise. |
| Mining the page DOM | LinkedIn's markup is generated and changes without notice. A selector that silently stops matching produces an account that looks quiet, which is worse than one that looks broken. v1 reads only the URL, the title and your selection: three things that do not move. |

The check `scripts/pruefe-keine-hintergrundabfrage.py` in the repository root
fails the build if a host permission, an alarm or a periodic fetch appears here.

## Install

```
npm install --prefix extension   # nothing to install yet, but it pins the versions
npm run --prefix extension build
```

Then in Chrome: `chrome://extensions` → Developer mode → Load unpacked →
select this `extension/` directory. Open the options page and enter the URL of
your vigil server and a token from `vigil token create`.
