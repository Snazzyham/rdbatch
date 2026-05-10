# rdbatch — Developer Brief

## Overview

Build a lightweight terminal-native CLI tool in Go called `rdbatch`.

The tool integrates with the Real-Debrid API and allows users to:

1. Add magnet links to Real-Debrid
2. View recent torrents
3. Batch-select torrents through an interactive checkbox UI
4. Download torrent files directly using aria2

The goal is to eliminate browser interaction entirely.

---

# Core Commands

## Command Structure

```bash id="md7lqf"
rdbatch fetch <magnet>
rdbatch list
```

---

# Command: `rdbatch fetch`

## Purpose

Adds a magnet link to the user’s Real-Debrid torrent queue.

---

## Usage

```bash id="ifzt6o"
rdbatch fetch "magnet:?xt=urn:btih:..."
```

---

## Behavior

### Flow

1. Validate magnet link format
2. Send magnet to Real-Debrid:

   ```http
   POST /torrents/addMagnet
   ```
3. Return:

   * torrent ID
   * filename
   * status

---

## Expected Output

Example:

```text id="y9xb5j"
Added torrent successfully:

Name: Ubuntu.24.04.iso
ID: XXXXXXXXX
Status: magnet_conversion
```

---

## Optional Enhancement

If torrent requires file selection:

* automatically select all files using:

  ```http
  POST /torrents/selectFiles/{id}
  ```

  with:

  ```text
  files=all
  ```

This is strongly recommended for MVP usability.

---

# Command: `rdbatch list`

## Purpose

Displays recent Real-Debrid torrents in an interactive terminal UI and allows batch downloading.

---

## Usage

```bash id="pq1hq9"
rdbatch list
```

---

# Behavior

## Step 1 — Fetch Torrents

Use:

```http id="gny3kk"
GET /torrents
```

Retrieve:

* latest 40 torrents
* newest first

---

## Step 2 — Render Interactive TUI

Display checkbox-style list.

Example:

```text id="egimfh"
[ ] Ubuntu ISO
[x] Movie.Name.2025
[x] TV.Show.S03E04
[ ] Linux.Distro

SPACE  toggle
A      select all
N      select none
ENTER  download selected
Q      quit
```

---

# Keyboard Controls

| Key        | Action            |
| ---------- | ----------------- |
| Up/Down    | Navigate          |
| Space      | Toggle selection  |
| A          | Select all        |
| N          | Select none       |
| Enter      | Download selected |
| Q / Ctrl+C | Exit              |

---

# Torrent Display Fields

Each row should show:

* torrent name
* status
* size
* added date

Optional:

* color-code statuses

---

# Download Flow

For each selected torrent:

---

## A — Fetch Torrent Info

```http id="ngd2a6"
GET /torrents/info/{id}
```

---

## B — Extract File Links

Retrieve file URLs from response.

---

## C — Unrestrict Links

```http id="i0g7fp"
POST /unrestrict/link
```

Convert RD links into direct CDN download URLs.

---

## D — Download via aria2c

Spawn:

```bash id="1l7ly9"
aria2c
```

Recommended args:

```bash id="nlsmx5"
aria2c \
  -x 16 \
  -s 16 \
  -k 1M \
  --continue=true \
  --auto-file-renaming=false \
  URL
```

---

# Download Location

Critical behavior:

Downloads must save into the shell’s current working directory.

Example:

```bash id="pk20w8"
cd ~/downloads/movies
rdbatch list
```

Files download into:

```text id="65u5lr"
~/downloads/movies
```

Implementation:

```go id="2aqh13"
cwd, err := os.Getwd()
```

Do NOT use:

* executable directory
* temp folder
* config folder

---

# Authentication

## Preferred Authentication Methods

### Environment Variable

```bash id="s2wlgu"
REALDEBRID_API_KEY
```

### Config File Fallback

```text id="wdbn7o"
~/.config/rdbatch/config.json
```

Example:

```json id="jlwmw8"
{
  "api_key": "xxxxxxxx"
}
```

Environment variable overrides config.

---

# Recommended Libraries

## CLI

Preferred:

```text id="h2kp9o"
github.com/spf13/cobra
```

---

## Terminal UI

Preferred:

```text id="r5i1hc"
github.com/charmbracelet/bubbletea
github.com/charmbracelet/bubbles
```

---

## HTTP

Use standard library:

```go id="m04c26"
net/http
```

---

# Suggested Project Structure

```text id="g0j8ic"
cmd/rdbatch/main.go

internal/api/
internal/config/
internal/download/
internal/ui/
internal/models/
internal/commands/
```

---

# Concurrency

Support configurable simultaneous downloads.

Default:

* 3 concurrent aria2 jobs

Suggested implementation:

* semaphore
* worker pool

---

# Error Handling

Gracefully handle:

* invalid API key
* expired token
* malformed magnet
* aria2 missing
* torrent unavailable
* failed unrestrict
* network failures

Do not abort entire batch if one item fails.

---

# Optional Future Features

Not required for MVP.

---

## Future Enhancements

### Search Filter

Inside TUI:

* fuzzy search
* `/` search mode

---

### Auto Refresh

Refresh torrent statuses every few seconds.

---

### Clipboard Integration

```bash id="03j8s8"
rdbatch fetch
```

Auto-import magnet from clipboard and prompt if user would like to download.

---

### File-Level Selection

Allow selecting individual files within torrent.

---

### Auto-Download Mode

```bash id="i0p6qa"
rdbatch fetch <magnet> --wait --download
```

Behavior:

1. add torrent
2. wait until cached/ready
3. auto-download immediately

This would become the killer feature.

---

# MVP Definition

MVP is complete when:

* `rdbatch fetch <magnet>` adds torrent to RD
* `rdbatch list` shows recent torrents in TUI
* user can multi-select torrents
* selected torrents download via aria2
* downloads save into current working directory
* zero browser interaction required

---

# Non-Goals

Do NOT implement:

* streaming
* torrent searching
* media management
* bittorrent client
* web UI
* local DB
* torrent uploading beyond magnet add

Keep the tool:

* fast
* terminal-native
* minimal
* dependency-light
* Unix-friendly
