# Changelog

All notable changes to `rdbatch` are documented in this file.

---

## [Unreleased]

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
    - `POST /torrents/requestdl` per file to obtain direct CDN URLs
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

- This project is in active development. The MVP is complete and functional.
- See `README.md` for build instructions and `briefs/` for the full spec and future roadmap.
