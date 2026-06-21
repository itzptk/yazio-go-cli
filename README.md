<div align="center">

  # yazio-go-cli

  **A scriptable Go CLI for the unofficial YAZIO API**

  [![CI](https://github.com/itzptk/yazio-go-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/itzptk/yazio-go-cli/actions/workflows/ci.yml)
  [![Release](https://github.com/itzptk/yazio-go-cli/actions/workflows/release.yml/badge.svg)](https://github.com/itzptk/yazio-go-cli/actions/workflows/release.yml)
  [![Go version](https://img.shields.io/github/go-mod/go-version/itzptk/yazio-go-cli)](go.mod)
  [![License: MIT](https://img.shields.io/github/license/itzptk/yazio-go-cli)](LICENSE)

  <br />

  **Read, search, add, and remove YAZIO diary data from your terminal.**  
  Authenticate with your YAZIO account, fetch daily summaries, search products, and automate food logging.

  <br />
</div>

---

> [!WARNING]
> This project relies on YAZIO's unofficial/private API. Endpoints, payloads, or authentication flows may change without notice and break the CLI.

## ✨ Features

| | Feature | Description |
|---|---|---|
| 🔐 | **Email/password login** | Exchange YAZIO credentials for an API token and save it locally |
| ♻️ | **Token persistence** | Persist and refresh API tokens automatically after login |
| 👤 | **User profile** | Fetch the current YAZIO user profile |
| 📊 | **Daily summary** | Read nutrition summary data for today or a specific date |
| 🍽️ | **Diary entries** | List consumed diary items for a date |
| 🔍 | **Product search** | Search products in the YAZIO database |
| ➕ | **Add diary items** | Add consumed products with meal, serving, amount, and date metadata |
| 🗑️ | **Remove entries** | Remove consumed diary entries by entry ID |
| ⚙️ | **Config bootstrap** | Generate a starter config with `yazio config init` |
| 🧾 | **Scriptable output** | Use table output for humans or JSON output for automation |
| 📤 | **Diary CSV export** | Export one day or an inclusive date range of diary entries to CSV |

## ⌨️ Usage

### Quick start

```bash
yazio config init
# Edit ~/.config/yazio-go-cli/config.yaml and set oauth.client_id / oauth.client_secret.
yazio auth login --email you@example.com --password 'your-password'
yazio auth status
```

Or provide credentials through environment variables:

```bash
export YAZIO_CLIENT_ID='your-client-id'
export YAZIO_CLIENT_SECRET='your-client-secret'
export YAZIO_EMAIL=you@example.com
export YAZIO_PASSWORD='your-password'
yazio auth login
```

### Common commands

| Command | Description |
|---|---|
| `yazio user profile` | Fetch the current user profile |
| `yazio summary` | Fetch today's daily summary |
| `yazio summary 2026-06-02` | Fetch the daily summary for a date |
| `yazio consumed 2026-06-02` | List consumed diary entries for a date |
| `yazio export diary 2026-06-02` | Export one diary day as CSV to stdout |
| `yazio export diary --from 2026-06-01 --to 2026-06-07 --file diary.csv` | Export an inclusive diary date range to a CSV file |
| `yazio search banana` | Search products in the YAZIO database |
| `yazio --output json summary 2026-06-02` | Emit JSON for scripting |

CSV exports write this header:

```text
date,category,meal,entry_id,type,product_id,name,producer,amount,serving,serving_quantity,raw_json
```

If you pass `--file`, the CLI writes the CSV file and prints the exported entry count. Without `--file`, CSV goes to stdout for piping into other tools. Product rows use typed columns directly; recipe portions and simple products also include `raw_json` so unsupported YAZIO diary fields are not silently dropped.

### Write operations

```bash
yazio add \
  --product-id 11111111-1111-1111-1111-111111111111 \
  --meal breakfast \
  --amount 100 \
  --serving g \
  --serving-quantity 1 \
  --date 2026-06-02

yazio remove 22222222-2222-2222-2222-222222222222
```

---

## 🚀 Installation

### With Go install

```bash
go install github.com/itzptk/yazio-go-cli/cmd/yazio@latest
```

### Download

When releases are published, grab the latest platform archive and `checksums.txt` from the [Releases page](https://github.com/itzptk/yazio-go-cli/releases).

### Or build from source

```bash
git clone https://github.com/itzptk/yazio-go-cli.git
cd yazio-go-cli
go build -o bin/yazio ./cmd/yazio
```

---

## ⚙️ Configuration

Default config path:

```text
~/.config/yazio-go-cli/config.yaml
```

Generate a starter config:

```bash
yazio config init
```

To overwrite an existing config file:

```bash
yazio config init --force
```

You can also copy the checked-in example manually:

```bash
mkdir -p ~/.config/yazio-go-cli
cp config.example.yaml ~/.config/yazio-go-cli/config.yaml
```

Override the config path per command:

```bash
yazio --config /path/to/config.yaml ...
```

| Key | Description |
|---|---|
| `base_url` | Override the unofficial YAZIO API base URL |
| `output` | Default output format: `table` or `json` |
| `oauth.client_id` | OAuth client ID used for login and token refresh; can also be set with `YAZIO_CLIENT_ID` |
| `oauth.client_secret` | OAuth client secret used for login and token refresh; can also be set with `YAZIO_CLIENT_SECRET` |
| `token` | Populated automatically after `yazio auth login` |

`yazio auth login` and automatic token refresh fail before contacting YAZIO if the OAuth client ID or secret is missing.

---

## 🧱 Stack

| Layer | Technology |
|---|---|
| **Language** | [Go 1.24](https://go.dev/) |
| **CLI Framework** | [Cobra](https://cobra.dev/) |
| **Config** | [Viper](https://github.com/spf13/viper) + YAML |
| **API Client** | Go `net/http` client for the unofficial YAZIO API |
| **Identifiers** | [google/uuid](https://github.com/google/uuid) |
| **Testing** | `go test ./...` |
| **CI/CD** | GitHub Actions — CI, build-only artifacts, beta/stable release flow |

---

## 🛠️ Development

```bash
git clone https://github.com/itzptk/yazio-go-cli.git
cd yazio-go-cli
make test
make build
```

### Available targets

| Command | Description |
|---|---|
| `make fmt` | Format Go source with `go fmt ./...` |
| `make test` | Run `go test ./...` |
| `make build` | Build `bin/yazio` |
| `make docker-test` | Run tests in a Go 1.24 container |
| `make docker-build` | Build `bin/yazio` in a Go 1.24 container |

If you prefer direct Go commands:

```bash
go fmt ./...
go test ./...
go build -o bin/yazio ./cmd/yazio
```

---

## 📦 Release Workflow

The GitHub Actions release flow supports stable releases, beta releases, and build-only artifact checks.

| Trigger | Release Type |
|---|---|
| workflow_dispatch `stable` with explicit version | Latest release `vX.Y.Z` |
| Schedule every 6h or workflow_dispatch `beta` | Pre-release `vX.Y.Z-beta.N` when `main` changed since the last beta |
| workflow_dispatch `build-only` | CI artifact build without creating a GitHub Release |

Release artifacts are published as platform archives named `yazio-go-cli_<version>_<os>_<arch>` with a `checksums.txt` file.

---

## 🤝 Contributing

Issues and pull requests are welcome.

When contributing:

- keep the command surface small and practical
- prefer narrow, well-tested endpoint coverage over broad speculative coverage
- document behavior changes in the README
- verify changes with real command output where possible

For usage questions, bugs, or API breakage reports, open an issue at <https://github.com/itzptk/yazio-go-cli/issues>. Include the CLI command, expected behavior, actual behavior, and whether the response came from the unofficial API.

---

## 🧪 API Notes

This project was informed by existing reverse-engineered/community work:

- `controlado/go-yazio` — unofficial Go client with token/auth patterns
- `juriadams/yazio` — TypeScript client with endpoint shapes for summary, search, and consumed items
- `saganos/yazio_public_api` — older Swagger-style API description and OAuth token notes

Current limitations:

- Email/password auth is the only supported login flow
- Apple/Google web-session import is not implemented
- Because the API is unofficial, long-term compatibility is not guaranteed

The goal here is a focused, scriptable CLI rather than exhaustive API coverage.

---

## 📄 License

MIT © [itzptk](https://github.com/itzptk)
