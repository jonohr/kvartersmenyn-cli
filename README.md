# kvartersmenyn-cli

A small terminal tool that fetches lunch menus from [kvartersmenyn.se](https://www.kvartersmenyn.se/). You provide one or more areas via config or flags; if missing, you will be prompted to paste a kvartersmenyn URL on startup.

## Getting started

1. Install Go 1.21+.
2. Fetch deps: `go mod tidy`.
3. Run: `go run .`

## Install (binary)

From a cloned repo:

```bash
./install.sh
```

One-liner:

```bash
curl -fsSL https://raw.githubusercontent.com/jonohr/kvartersmenyn-cli/main/install.sh | bash
```

You can choose a destination directory:

```bash
./install.sh --dest ~/.local/bin
```

On macOS, the script will optionally remove the quarantine attribute so the binary can run without Gatekeeper prompts.

Flags:

- `-a, --area` - area slug from the URL, e.g. `garda_161` (can be repeated or comma-separated).
- `-c, --city` - city segment from the URL, e.g. `goteborg` (required when using `--area`; optional for whole-city search).
- `-n, --name` - filter by restaurant name (case-insensitive, fuzzy).
- `-m, --menu` - filter by menu text (case-insensitive, fuzzy).
- `-s, --search` - filter both name and menu (fuzzy); can be combined with `--name`/`--menu` (specific ones win).
- `-d, --day` - day of week to fetch (mon, tue, wed, thu, fri, sat, sun or 1-7). Defaults to today.
- `-C, --cache-dir` - directory for cached HTML (empty string disables). Default per OS: Linux `~/.cache/kvartersmenyn/`, macOS `~/Library/Caches/kvartersmenyn/`, Windows `%LOCALAPPDATA%\\kvartersmenyn\\Cache\\` (can be set in config).
- `-t, --cache-ttl` - how long to reuse cache, e.g. `6h` (default), `1h`, `48h` (can be set in config).
- `-f, --config` - path to YAML config (default: Linux `~/.config/kvartersmenyn/config.yaml`, macOS `~/Library/Application Support/kvartersmenyn/config.yaml`, Windows `%LOCALAPPDATA%\\kvartersmenyn\\config.yaml`).
- `-i, --init-config` - run the interactive config setup and exit.
- `-h, --help` - show help and exit.
- `--version` - show version and exit.

Examples:

```bash
kvartersmenyn-cli -a garda_161
kvartersmenyn-cli -a garda_161 -n ullevi
kvartersmenyn-cli -a garda_161 -m burgare
kvartersmenyn-cli -a garda_161 -s gaby   # searches both name and menu
kvartersmenyn-cli -a garda_161 -d fri
kvartersmenyn-cli -c stockholm -a ostermalm_42
kvartersmenyn-cli -c goteborg -a garda_161 -a johanneberg_43
kvartersmenyn-cli -c goteborg
```

## macOS Gatekeeper

If macOS blocks the downloaded binary because it is unsigned, you can either:

- Right-click the file, choose Open, then confirm once.
- Or run: `xattr -dr com.apple.quarantine /path/to/kvartersmenyn-cli`

## Config file

Create a config file (default path varies per OS, see `--config` above) to avoid passing flags:

```yaml
city: goteborg
areas:
  - area: garda_161
  - area: johanneberg_43
cache_dir: .cache
cache_ttl: 6h
```

`cache_ttl` expects a Go duration (e.g. `6h`). If you provide a plain number (e.g. `6`), it is treated as hours.

You can list multiple areas in the `areas` array. Each item can inherit `city` from the top level or override it with its own `city` value. If you only set `city` and omit `areas`, the whole city is used.

Flags always win over config when both are set. Defaults are used when neither flags nor config specify a field.

If no valid config is found on startup, you will be prompted for a kvartersmenyn URL (city or area) and cache TTL (default 6h); the config is then saved to the default path.
