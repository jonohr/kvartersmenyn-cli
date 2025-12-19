# kvartersmenyn-cli

A small terminal tool that fetches lunch menus from [kvartersmenyn.se](https://www.kvartersmenyn.se/). You provide one or more areas via config or flags; if missing, you will be prompted to paste a kvartersmenyn URL on startup.

## Getting started

1. Install Go 1.21+.
2. Fetch deps: `go mod tidy`.
3. Run: `go run .`

Flags:

- `-area` - area slug from the URL, e.g. `garda_161` (can be repeated or comma-separated).
- `-city` - city segment from the URL, e.g. `goteborg` (required when using `-area`).
- `-name` - filter by restaurant name (case-insensitive, fuzzy).
- `-menu` - filter by menu text (case-insensitive, fuzzy).
- `-search` - filter both name and menu (fuzzy); can be combined with `-name`/`-menu` (specific ones win).
- `-cache-dir` - directory for cached HTML (empty string disables). Default per OS: Linux `~/.cache/kvartersmenyn/`, macOS `~/Library/Caches/kvartersmenyn/`, Windows `%LOCALAPPDATA%\\kvartersmenyn\\Cache\\` (can be set in config).
- `-cache-ttl` - how long to reuse cache, e.g. `6h` (default), `1h`, `48h` (can be set in config).
- `-config` - path to YAML config (default: Linux `~/.config/kvartersmenyn/config.yaml`, macOS `~/Library/Application Support/kvartersmenyn/config.yaml`, Windows `%LOCALAPPDATA%\\kvartersmenyn\\config.yaml`).
- `-init-config` - run the interactive config setup and exit.
- `-help` - show help and exit.

Examples:

```bash
go run . -area garda_161
go run . -area garda_161 -name ullevi
go run . -area garda_161 -menu burgare
go run . -area garda_161 -search gaby   # searches both name and menu
go run . -area garda_161 -cache-ttl 2h
go run . -city stockholm -area ostermalm_42
go run . -city goteborg -area garda_161 -area johanneberg_43
go run . -init-config
```

## Config file

Create a config file (default path varies per OS, see `-config` above) to avoid passing flags:

```yaml
city: goteborg
areas:
  - area: garda_161
  - area: johanneberg_43
cache_dir: .cache
cache_ttl: 6h
```

You can list multiple areas in the `areas` array. Each item can inherit `city` from the top level or override it with its own `city` value.

Flags always win over config when both are set. Defaults are used when neither flags nor config specify a field.

If no valid config is found on startup, you will be prompted for a kvartersmenyn URL (city or area) and cache TTL (default 6h); the config is then saved to the default path.
