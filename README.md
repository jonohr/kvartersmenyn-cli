# kvartersmenyn-cli

A small terminal tool that fetches lunch menus from [kvartersmenyn.se](https://www.kvartersmenyn.se/). You provide city/area via config or flags; if missing, you will be prompted to paste a kvartersmenyn URL on startup.

## Getting started

1. Install Go 1.21+.
2. Fetch deps: `go mod tidy`.
3. Run: `go run .`

Flags:

- `-area` - area slug from the URL, e.g. `garda_161` (can be set in config). Required if not set in config.
- `-city` - city segment from the URL, e.g. `goteborg` (can be set in config). Required if not set in config.
- `-file` - optional local HTML document (skips network, handy for testing).
- `-name` - filter by restaurant name (case-insensitive, fuzzy).
- `-menu` - filter by menu text (case-insensitive, fuzzy).
- `-search` - filter both name and menu (fuzzy); can be combined with `-name`/`-menu` (specific ones win).
- `-cache-dir` - directory for cached HTML (empty string disables). Default per OS: Linux `~/.cache/kvartersmenyn/`, macOS `~/Library/Caches/kvartersmenyn/`, Windows `%LOCALAPPDATA%\\kvartersmenyn\\Cache\\` (can be set in config).
- `-cache-ttl` - how long to reuse cache, e.g. `6h` (default), `1h`, `48h` (can be set in config).
- `-config` - path to YAML config (default: Linux `~/.config/kvartersmenyn/config.yaml`, macOS `~/Library/Application Support/kvartersmenyn/config.yaml`, Windows `%LOCALAPPDATA%\\kvartersmenyn\\config.yaml`).
- `-help` - show help and exit.

Examples:

```bash
go run . -area garda_161
go run . -area garda_161 -name ullevi
go run . -area garda_161 -menu burgare
go run . -area garda_161 -search gaby   # searches both name and menu
go run . -area garda_161 -cache-ttl 2h
go run . -city stockholm -area ostermalm_42
go run . -file fixtures/garda.html
```

## Config file

Create a config file (default path varies per OS, see `-config` above) to avoid passing flags:

```yaml
city: goteborg
area: garda_161
cache_dir: .cache
cache_ttl: 6h
```

Flags always win over config when both are set. Defaults are used when neither flags nor config specify a field.

If no valid config is found on startup, you will be prompted for a kvartersmenyn URL (to extract city/area) and cache TTL (default 6h); the config is then saved to the default path.
