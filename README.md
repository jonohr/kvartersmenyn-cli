# kvartersmenyn-cli

Ett litet terminalverktyg som hämtar lunchmenyer från [kvartersmenyn.se](https://www.kvartersmenyn.se/). Du anger city/area via config eller flaggor; saknas det blir du ombedd att klistra in en kvartersmenyn-URL vid start.

## Kom igång

1. Installera Go 1.21+.
2. Hämta beroenden: `go mod tidy`.
3. Kör: `go run .`

Flaggor:

- `-area` – områdessluggen från URL:en, t.ex. `garda_161` (kan sättas i config). Måste anges om inte satt i config.
- `-city` – stadssegmentet i URL:en, t.ex. `goteborg` (kan sättas i config). Måste anges om inte satt i config.
- `-file` – valfritt lokalt HTML-dokument (hoppar över nätverksanrop, bra för testning).
- `-name` – filtrera på restaurangnamn (case-insensitive med fuzzy matchning).
- `-menu` – filtrera på menyrader (case-insensitive med fuzzy matchning).
- `-search` – filtrera både namn och meny (fuzzy); kan kombineras med `-name`/`-menu` (de specifika vinner).
- `-cache-dir` – katalog för cachead HTML (tom sträng stänger av). Default följer OS: Linux `~/.cache/kvartersmenyn/`, macOS `~/Library/Caches/kvartersmenyn/`, Windows `%LOCALAPPDATA%\\kvartersmenyn\\Cache\\` (kan sättas i config).
- `-cache-ttl` – hur länge cachen får leva, t.ex. `6h` (default), `1h`, `48h` (kan sättas i config).
- `-config` – sökväg till YAML-konfig (default: Linux `~/.config/kvartersmenyn/config.yaml`, macOS `~/Library/Application Support/kvartersmenyn/config.yaml`, Windows `%LOCALAPPDATA%\\kvartersmenyn\\config.yaml`).
- `-help` – visa hjälpen och avsluta.

Exempel:

```bash
go run . -area garda_161
go run . -area garda_161 -name ullevi
go run . -area garda_161 -menu burgare
go run . -area garda_161 -search gaby   # söker både namn och meny
go run . -area garda_161 -cache-ttl 2h
go run . -city stockholm -area ostermalm_42
go run . -file fixtures/garda.html
```

## Konfigurationsfil

Skapa configfil (default-sökväg varierar per OS, se `-config` ovan) för att slippa flaggor:

```yaml
city: goteborg
area: garda_161
cache_dir: .cache
cache_ttl: 6h
```

Flaggor vinner alltid över config om båda är satta. Defaultvärden används om varken flagga eller config anger fältet.

Om ingen giltig config hittas vid start blir du interaktivt tillfrågad om en kvartersmenyn-URL (för att plocka city/area) samt cache-ttl (default 6h); därefter sparas config automatiskt på standardstigen.
