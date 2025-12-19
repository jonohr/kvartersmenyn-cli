# kvartersmenyn-cli

Ett litet terminalverktyg som hämtar lunchmenyer från [kvartersmenyn.se](https://www.kvartersmenyn.se/). Standardläget hämtar Gårda i Göteborg (`garda_161`), men du kan ange valfritt område.

## Kom igång

1. Installera Go 1.21+.
2. Hämta beroenden: `go mod tidy`.
3. Kör: `go run .`

Flaggor:

 - `-area` – områdessluggen från URL:en, t.ex. `garda_161` (kan sättas i config). Måste anges om inte satt i config.
 - `-city` – stadssegmentet i URL:en, t.ex. `goteborg` (kan sättas i config). Måste anges om inte satt i config.
- `-file` – valfritt lokalt HTML-dokument (hoppar över nätverksanrop, bra för testning).
- `-search` – filtrera på restaurangnamn (case-insensitive med fuzzy matchning).
- `-cache-dir` – katalog för cachead HTML (tom sträng stänger av). Default `.cache` (kan sättas i config).
- `-cache-ttl` – hur länge cachen får leva, t.ex. `6h` (default), `1h`, `48h` (kan sättas i config).
- `-config` – sökväg till YAML-konfig (default `~/.config/kvartersmenyn/config.yaml`).

Exempel:

```bash
go run . -area garda_161
go run . -area garda_161 -search ullevi
go run . -area garda_161 -cache-ttl 2h
go run . -city stockholm -area ostermalm_42
go run . -file fixtures/garda.html
```

## Konfigurationsfil

Skapa `~/.config/kvartersmenyn/config.yaml` (eller ange egen med `-config`) för att slippa flaggor:

```yaml
city: goteborg
area: garda_161
cache_dir: .cache
cache_ttl: 6h
```

Flaggor vinner alltid över config om båda är satta. Defaultvärden används om varken flagga eller config anger fältet.
