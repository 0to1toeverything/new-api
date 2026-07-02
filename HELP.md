# 🛠 Local Development Help

Run the project locally without Docker (classic frontend). All scripts are self-contained in the repo root.

## Prerequisites

- **Go** ≥ 1.25 (backend)
- **[bun](https://bun.sh)** (frontend build & dev server)

```bash
# install bun (one-liner)
curl -fsSL https://bun.sh/install | bash
```

## First time: fix date-fns version conflict

The classic frontend's `date-fns-tz` is incompatible with the workspace's `date-fns` v4. Fix it once:

```bash
cd web && bun add date-fns-tz@^3 --cwd classic
```

## Quick Start

```bash
# build everything, then run
./local-dev.sh build-all
./local-run.sh

# open http://localhost:3000
# login: root / 123456
```

## Scripts

| Script | What it does |
|---|---|
| `local-dev.sh` | Build backend and/or classic frontend |
| `local-run.sh` | Run pre-built backend on `:3000` |
| `local-web.sh` | Start classic frontend dev server (HMR) on `:5174` |

### `local-dev.sh` sub-commands

```bash
./local-dev.sh build        # build backend only  →  ./new-api
./local-dev.sh build-web    # build classic frontend →  web/classic/dist/
./local-dev.sh build-all    # build classic frontend then backend
./local-dev.sh run          # build backend (if needed) and run
./local-dev.sh clean        # remove ./new-api
```

## How it works

The Go binary embeds both `web/default/dist/` and `web/classic/dist/` at compile time via `//go:embed`. Building the frontend first populates these directories so the compiled binary serves the real UI.

- **`./local-dev.sh build-backend` only** → placeholder pages, not the real UI.
- **`./local-dev.sh build-all`** → builds classic frontend first, then the backend embeds it.
- **`./local-run.sh` + `./local-web.sh`** → backend serves API (`:3000`), classic frontend dev server on `:5174` with HMR. Use this when iterating on UI code.

## Defaults

The scripts set sensible defaults that require no external services:

| Env var | Default | Why |
|---|---|---|
| `SQLITE_PATH` | `one-api.db` | Uses SQLite — no PostgreSQL/MySQL |
| `MEMORY_CACHE_ENABLED` | `true` | In-memory cache — no Redis |
| `SESSION_SECRET` | `local-dev-change-me` | Required for session encryption |
| `BATCH_UPDATE_ENABLED` | `true` | Keeps cache in sync |

Override any by exporting before running:

```bash
export SQLITE_PATH=/tmp/test.db
export REDIS_CONN_STRING=redis://localhost:6379
./local-run.sh
```

## First-time setup

On first launch the backend auto-creates:

- A root user: `root` / `123456`
- The SQLite database at `$SQLITE_PATH`
- Logs under `./logs/`

Reset everything by deleting `one-api.db` and `./logs/`.

## Environment variable reference

| Variable | Default | Notes |
|---|---|---|
| `SQLITE_PATH` | `one-api.db` | SQLite file path |
| `SQL_DSN` | *(unset)* | Set to use MySQL or PostgreSQL instead |
| `MEMORY_CACHE_ENABLED` | `true` | In-memory cache; set `REDIS_CONN_STRING` to use Redis |
| `REDIS_CONN_STRING` | *(unset)* | Redis address (optional) |
| `SESSION_SECRET` | `local-dev-change-me` | Session encryption key |
| `BATCH_UPDATE_ENABLED` | `true` | Batch cache updates |
