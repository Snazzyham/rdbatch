# rdbatch — Developer Update Brief
## Add Torbox Provider Support (Multi-Provider Architecture)

---

## Overview

This update adds **Torbox** as a second supported provider alongside Real-Debrid.

The tool must now support a `RDBATCH_PROVIDER` environment variable that controls which backend is used. Both providers share the same CLI interface, TUI, and download logic — only the API layer differs.

**No existing Real-Debrid functionality should be broken or removed.**

---

## Summary of Changes Required

| Area | Change Type | Scope |
|---|---|---|
| `internal/config/` | Modify | Add provider field, Torbox API key |
| `internal/api/` | Refactor + Add | Extract interface, add Torbox client |
| `internal/models/` | Modify | Torbox response structs |
| `internal/commands/` | Minimal | Pass provider-aware client |
| `cmd/rdbatch/main.go` | Minimal | Provider resolution at startup |

---

## Step 1 — Add `RDBATCH_PROVIDER` Environment Variable

### Environment Variables

Add the following new environment variables:

```text
RDBATCH_PROVIDER         # "torbox" or "real-debrid" (required, no default)
TORBOX_API_KEY           # Torbox API key
REALDEBRID_API_KEY       # Existing — unchanged
```

### Resolution Rules

- `RDBATCH_PROVIDER` must be read first, before any API client is initialized.
- If `RDBATCH_PROVIDER` is missing or not one of the two valid values, exit with a clear error:

```text
Error: RDBATCH_PROVIDER is not set.
Set it to either "torbox" or "real-debrid".

Example:
  export RDBATCH_PROVIDER=torbox
```

- `RDBATCH_PROVIDER` environment variable takes precedence over the config file value (see Step 2).

---

## Step 2 — Update Config File Schema

### File Location (unchanged)

```text
~/.config/rdbatch/config.json
```

### New Schema

```json
{
  "provider": "torbox",
  "realdebrid_api_key": "xxxxxxxx",
  "torbox_api_key": "xxxxxxxx"
}
```

**Notes:**
- Rename existing `api_key` field to `realdebrid_api_key`. This is a breaking change to the config schema — document it clearly in any README.
- Add `torbox_api_key` field.
- Add `provider` field as a fallback if `RDBATCH_PROVIDER` env var is not set.
- Both API key fields are optional in the config — only the one matching the active provider needs to be present.

### Key Resolution Order (per provider)

**Real-Debrid API key:**
1. `REALDEBRID_API_KEY` environment variable
2. `realdebrid_api_key` in config file

**Torbox API key:**
1. `TORBOX_API_KEY` environment variable
2. `torbox_api_key` in config file

**Provider:**
1. `RDBATCH_PROVIDER` environment variable
2. `provider` in config file

If the resolved provider's API key is missing after checking both sources, exit with:

```text
Error: No API key found for provider "torbox".
Set TORBOX_API_KEY or add "torbox_api_key" to ~/.config/rdbatch/config.json
```

---

## Step 3 — Refactor `internal/api/` to a Provider Interface

This is the largest change. The API layer must be abstracted so commands do not care which provider is active.

### Define a `Provider` Interface

Create a new file `internal/api/provider.go`:

```go
package api

type Torrent struct {
    ID       string
    Name     string
    Status   string
    Size     int64
    Added    time.Time
    Links    []string
}

type Provider interface {
    AddMagnet(magnet string) (id string, name string, status string, err error)
    ListTorrents() ([]Torrent, error)
    GetDownloadLinks(torrentID string) ([]string, error)
}
```

> `GetDownloadLinks` must return final, unrestricted direct download URLs ready to be passed to aria2. All provider-specific multi-step logic (e.g. unrestrict calls, requestdl calls) must be encapsulated inside the provider implementation — callers should never need to know about intermediate steps.

---

### Keep Existing Real-Debrid Client

Rename `internal/api/client.go` (or equivalent) to `internal/api/realdebrid.go`.

Ensure the `RealDebridClient` struct implements the `Provider` interface.

The `GetDownloadLinks` method on `RealDebridClient` must:
1. Call `GET /torrents/info/{id}` to get file links
2. Call `POST /unrestrict/link` for each link
3. Return the final direct URLs

No changes to the Real-Debrid logic itself are required.

---

### Create New Torbox Client

Create `internal/api/torbox.go`.

#### Base URL

```text
https://api.torbox.app/v1/api
```

#### Authentication

All requests must include:

```http
Authorization: Bearer <TORBOX_API_KEY>
```

---

#### Implement `AddMagnet`

```http
POST /torrents/createtorrent
Content-Type: application/x-www-form-urlencoded

magnet=<magnet_link>
```

Response fields to extract:
- `data.torrent_id` → return as `id`
- `data.name` → return as `name`
- `data.download_state` → return as `status`

---

#### Implement `ListTorrents`

```http
GET /torrents/mylist?bypass_cache=true
```

- Return latest 40 torrents, sorted newest first (sort by `created_at` descending if not already ordered).
- Map response fields:

| Torbox field | `Torrent` struct field |
|---|---|
| `id` | `ID` |
| `name` | `Name` |
| `download_state` | `Status` |
| `size` | `Size` |
| `created_at` | `Added` |

---

#### Implement `GetDownloadLinks`

This is a two-step process for Torbox:

**Step A — Get file list**

```http
GET /torrents/mylist?id=<torrent_id>&bypass_cache=true
```

Extract the `files` array from the response. Each file has a `id` field.

**Step B — Request download URL per file**

For each file:

```http
POST /torrents/requestdl
Content-Type: application/x-www-form-urlencoded

token=<TORBOX_API_KEY>&torrent_id=<torrent_id>&file_id=<file_id>
```

Response field to extract:
- `data` → this is the direct download URL string

Collect all URLs and return them. Do not abort the entire torrent if a single file URL request fails — log the error and continue.

---

## Step 4 — Provider Resolution at Startup

In `cmd/rdbatch/main.go`, resolve the provider once at startup and pass the resulting `Provider` interface down to commands.

Pseudocode:

```go
cfg := config.Load()

var client api.Provider
switch cfg.Provider {
case "torbox":
    client = api.NewTorboxClient(cfg.TorboxAPIKey)
case "real-debrid":
    client = api.NewRealDebridClient(cfg.RealDebridAPIKey)
default:
    fmt.Fprintf(os.Stderr, "Error: unknown provider %q\n", cfg.Provider)
    os.Exit(1)
}
```

All cobra commands must accept the `api.Provider` interface — not a concrete client type.

---

## Step 5 — No Changes Required

The following packages require **no changes** as long as Step 3 is implemented correctly:

- `internal/ui/` — TUI is provider-agnostic
- `internal/download/` — aria2 invocation receives plain URLs
- Concurrency/semaphore logic
- CWD download behavior

---

## Step 6 — Status Color Coding (Torbox Statuses)

If the TUI color-codes torrent statuses, add mappings for Torbox's `download_state` values:

| Torbox status | Suggested color |
|---|---|
| `downloading` | Yellow |
| `uploading` | Yellow |
| `cached` | Green |
| `paused` | Gray |
| `stalled` | Red |
| `completed` | Green |
| `error` | Red |

Real-Debrid status colors should remain unchanged.

---

## Acceptance Criteria

The update is complete when all of the following are true:

- [ ] `RDBATCH_PROVIDER=real-debrid rdbatch fetch <magnet>` works identically to before
- [ ] `RDBATCH_PROVIDER=real-debrid rdbatch list` works identically to before
- [ ] `RDBATCH_PROVIDER=torbox rdbatch fetch <magnet>` adds a torrent to Torbox
- [ ] `RDBATCH_PROVIDER=torbox rdbatch list` shows Torbox torrents in the TUI
- [ ] Multi-select and download works end-to-end for Torbox via aria2
- [ ] Downloads land in the current working directory for both providers
- [ ] Missing `RDBATCH_PROVIDER` produces a clear error and non-zero exit code
- [ ] Missing API key for the active provider produces a clear error and non-zero exit code
- [ ] Config file `provider` field works as fallback when env var is not set
- [ ] Both API keys can coexist in the config file without conflict

---

## Non-Goals for This Update

Do NOT change:

- CLI command names or flags
- TUI layout or keyboard controls
- aria2 invocation arguments
- Download directory logic
- Any Real-Debrid API logic
