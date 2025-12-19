# kvartersmenyn-cli

Ett litet terminalverktyg som hämtar lunchmenyer från [kvartersmenyn.se](https://www.kvartersmenyn.se/). Standardläget hämtar Gårda i Göteborg (`garda_161`), men du kan ange valfritt område.

## Kom igång

1. Installera Go 1.21+.
2. Hämta beroenden: `go mod tidy`.
3. Kör: `go run .`

Flaggor:

- `-area` – områdessluggen från URL:en, t.ex. `garda_161`.
- `-city` – stadssegmentet i URL:en, t.ex. `goteborg`.
- `-file` – valfritt lokalt HTML-dokument (hoppar över nätverksanrop, bra för testning).
- `-search` – filtrera på restaurangnamn (case-insensitive med fuzzy matchning).
- `-cache-dir` – katalog för cachead HTML (tom sträng stänger av). Default `.cache`.
- `-cache-ttl` – hur länge cachen får leva, t.ex. `6h` (default), `1h`, `48h`.

Exempel:

```bash
go run . -area garda_161
go run . -area garda_161 -search ullevi
go run . -area garda_161 -cache-ttl 2h
go run . -city stockholm -area ostermalm_42
go run . -file fixtures/garda.html
```
