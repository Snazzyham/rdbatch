# Changelog

All notable changes to `rdbatch` are documented in this file.

---

## [0.3] - 2026-05-17

### Added

- **New `rdbatch search` command**
  - Interactive TUI for discovering torrents without leaving the terminal
  - Search by title through **Cinemeta** metadata API (no API key required)
  - Scrape torrents via **Comet** addon with debrid cache indicators
  - Multi-screen flow: Search → Results → Season picker (TV) → Episode picker (TV) → Torrents
  - Two actions on torrents: `Enter` to add to provider, `W` to stream (cached only)
  - Cache-aware UI: `[⚡]` = cached, `[⬇]` = uncached
  - Toggle uncached visibility with `U` key

- **Cinemeta integration** (`internal/search/cinemeta.go`)
  - Search movies and series: `GET /catalog/{type}/top/search={query}.json`
  - Fetch series metadata: `GET /meta/series/{imdb_id}.json`
  - Parallel requests for movie + series catalogs

- **Comet integration** (`internal/search/comet.go`)
  - Fetch streams: `GET {stream_base}/stream/{type}/{media_id}.json`
  - Parse cache indicators from stream names (⚡/⬇️/🧲)
  - Extract info hashes for magnet reconstruction
  - Filter out informational/error streams

- **Search TUI** (`internal/ui/search.go`)
  - Four-screen state machine: search, seasons, episodes, torrents
  - Preserved state when navigating back with `Esc`
  - Loading spinners, error messages, toast confirmations
  - Keyboard shortcuts: `/` focus search, `U` toggle uncached, `W` watch cached

- **Configuration**
  - New `COMET_URL` environment variable
  - New `comet_url` config file field
  - Automatic `.env` file loading at startup

- **Provider mismatch warning**
  - Displays warning if Comet appears configured for a different debrid than `RDBATCH_PROVIDER`

---

## [0.2] - 2026-05-16

### Added

- **File-level navigation (Torbox)**
  - New `F` keybinding to browse files within a torrent.
  - Support for multi-selecting and downloading specific files from a torrent.
  - New `ListFiles` method in `Provider` interface.

- **Direct Streaming / "Watch" mode**
  - New `W` keybinding to stream video files directly in `mpv` (primary) or `VLC` (fallback).
  - Supports both torrent-level (auto-selects largest file) and file-level streaming.
  - Zero-disk-usage streaming: players stream directly from CDN URLs.
  - Works for both Real-Debrid and Torbox.

- **Internal Player Package** (`internal/player/player.go`)
  - Automated detection of local media players.
  - Integration with `bubbletea` via `tea.ExecProcess`.

### Fixed

- **Torbox `requestdl` Method**
  - Corrected API call from `POST` to `GET` as per official documentation.
  - Fixed "405 Method Not Allowed" errors when retrieving download links.

---

## [0.1] - 2026-05-16

### Added

- **Multi-provider architecture**
  - Added **Torbox** as a second supported provider alongside Real-Debrid
  - New `RDBATCH_PROVIDER` environment variable (required) — must be `"torbox"` or `"real-debrid"`
  - Config file now supports a `provider` field as a fallback
  - New `TORBOX_API_KEY` environment variable and `torbox_api_key` config field
  - `internal/api/provider.go` — provider-agnostic `Provider` interface (`AddMagnet`, `ListTorrents`, `GetDownloadLinks`, `Aria2Flags`)

- **Torbox API client** (`internal/api/torbox.go`)
  - `POST /torrents/createtorrent` — add magnets
  - `GET /torrents/mylist?bypass_cache=true` — list torrents (latest 40, sorted newest-first)
  - Two-step download link resolution:
    - `GET /torrents/mylist?id={id}` to retrieve file list
    - `GET /torrents/requestdl` per file to obtain direct CDN URLs
  - Provider-specific aria2 flags: `-x 1 -s 1`

- **Updated config schema**
  - New `provider`, `realdebrid_api_key`, and `torbox_api_key` fields
  - Backward compatibility: old `api_key` field still accepted as a fallback for Real-Debrid

- **Provider-specific aria2 flags**
  - Real-Debrid: `-x 16 -s 16 -k 1M --continue=true --auto-file-renaming=false`
  - Torbox: `-x 1 -s 1 -k 1M --continue=true --auto-file-renaming=false`

### Changed

- **`internal/api/client.go` → `internal/api/realdebrid.go`**
  - Refactored Real-Debrid client to implement the new `Provider` interface
  - Encapsulated `GetTorrentInfo` + `UnrestrictLink` logic inside `GetDownloadLinks`
  - Encapsulated video file auto-selection logic inside `AddMagnet`

- **`internal/commands/root.go`**
  - Commands are now fully provider-agnostic; the concrete client is injected via `SetProvider()`

- **`internal/ui/list.go`**
  - Now renders `api.Torrent` instead of `models.Torrent`
  - Added Torbox status color mappings:
    - `downloading` / `uploading` → Yellow
    - `cached` / `completed` → Green
    - `paused` → Gray
    - `stalled` / `error` → Red

- **`cmd/rdbatch/main.go`**
  - Resolves provider and API key at startup, then initializes the correct concrete client

### Fixed

- **Blank UI when torrents exist in API response**
  - Root cause: `View()` was calling `SetItems()` every frame and mutating `torrent.Filename` with styled lipgloss strings, causing compounding ANSI escape codes
  - Fix: replaced default list delegate with a custom `itemDelegate` that renders directly without mutating item data; `SetItems` is now called only during initialization

- **JSON timestamp parsing failure**
  - Real-Debrid returns `added` timestamps in inconsistent formats (Unix integer, string integer, or RFC3339)
  - Added a custom `RDTime` type with robust unmarshalling for all three formats
  - Prevents silent JSON decode failures that could result in empty torrent lists

- **Torbox `created_at` string timestamp**
  - Torbox returns `created_at` as a stringified Unix timestamp instead of an integer
  - Added a custom `UnixTime` type that unmarshals both integer and string Unix timestamps
  - Prevents `json: cannot unmarshal string into Go struct field ... of type int64` errors

### Project Setup

- Initialized Go module (`go mod init`) with dependencies:
  - `github.com/spf13/cobra` — CLI framework
  - `github.com/charmbracelet/bubbletea` — TUI framework
  - `github.com/charmbracelet/bubbles` — list component
  - `github.com/charmbracelet/lipgloss` — styling
- Project structure:
  ```
  cmd/rdbatch/main.go
  internal/api/
  internal/commands/
  internal/config/
  internal/download/
  internal/log/
  internal/models/
  internal/ui/
  ```

---

## Notes

- This project is in active development.
- See `README.md` for build instructions and `briefs/` for the full spec and future roadmap.
