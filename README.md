# rdbatch

A lightweight, terminal-native CLI tool for [Real-Debrid](https://real-debrid.com/) and [Torbox](https://torbox.app/). Add magnets, browse your torrents, and batch-download files via `aria2c` — zero browser interaction required.

![Go](https://img.shields.io/badge/Go-1.22%2B-blue)
![License](https://img.shields.io/badge/license-MIT-green)

---

## Features

- **Multi-provider support** — Real-Debrid and Torbox backends
- **Add magnets** directly from the terminal
- **Auto-select video files** when adding a torrent (skips samples, nfo, txt, etc.)
- **Interactive TUI** to browse, multi-select, and batch-download torrents
- **Unrestricted direct CDN links** via Real-Debrid or Torbox
- **Concurrent aria2c downloads** with configurable limits (or unlimited)
- **Downloads to your current working directory**
- **Lightweight & fast** — single binary, no local database

---

## Installation

### Prerequisites

- [Go](https://go.dev/) 1.22 or later (to build from source)
- [aria2](https://aria2.github.io/) installed and available in your `PATH`

### Build from source

```bash
git clone https://github.com/Snazzyham/rdbatch.git
cd rdbatch
go build -o rdbatch ./cmd/rdbatch
sudo mv rdbatch /usr/local/bin/
```

### Install aria2

```bash
# macOS
brew install aria2

# Ubuntu/Debian
sudo apt install aria2

# Arch
sudo pacman -S aria2
```

---

## Configuration

`rdbatch` requires a provider and the corresponding API key.

### 1. Choose a Provider

Set the `RDBATCH_PROVIDER` environment variable:

```bash
# Real-Debrid
export RDBATCH_PROVIDER=real-debrid

# Torbox
export RDBATCH_PROVIDER=torbox
```

### 2. Set Your API Key

**Environment Variable (recommended)**

```bash
# Real-Debrid
export REALDEBRID_API_KEY="your_api_key_here"

# Torbox
export TORBOX_API_KEY="your_api_key_here"
```

**Config File**

Create `~/.config/rdbatch/config.json`:

```json
{
  "provider": "torbox",
  "realdebrid_api_key": "xxxxxxxx",
  "torbox_api_key": "xxxxxxxx"
}
```

> The `provider` field in the config file is used as a fallback when `RDBATCH_PROVIDER` is not set. Environment variables override config file values.

> **Backward compatibility:** the old `api_key` field is still accepted for Real-Debrid if `realdebrid_api_key` is not present.

You can find your API key at:
- Real-Debrid: https://real-debrid.com/apitoken
- Torbox: https://torbox.app/settings

---

## Usage

### Add a magnet link

```bash
rdbatch fetch "magnet:?xt=urn:btih:..."
```

This will:
1. Validate the magnet link
2. Add it to your provider queue
3. **Auto-select only video files** (with fallback to all files if none detected)

### List & download torrents

```bash
rdbatch list
```

Opens an interactive terminal UI showing your latest 40 torrents.

#### Keyboard Controls

| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate |
| `Space` | Toggle selection |
| `A` | Select all |
| `N` | Select none |
| `Enter` | Download selected |
| `Q` / `Ctrl+C` | Quit |

Each row shows the torrent name, status, size, and added date.

#### Download Options

```bash
# Unlimited concurrent downloads (default)
rdbatch list

# Limit to 5 concurrent aria2 jobs
rdbatch list -c 5
```

Downloads are saved to your **current working directory**:

```bash
cd ~/downloads/movies
rdbatch list
# files appear in ~/downloads/movies
```

---

## Command Reference

```
rdbatch fetch <magnet>     Add a magnet to the active provider
rdbatch list               Open interactive torrent list & downloader
rdbatch list -c 10         Limit concurrent downloads to 10
```

---

## Troubleshooting

### Debug Logging

`rdbatch` writes a detailed log file on every run. This is useful for diagnosing API issues, empty lists, or failed downloads.

**Log location:**
```
~/.local/share/rdbatch/rdbatch.log
```

**Monitor logs in real-time:**
```bash
tail -f ~/.local/share/rdbatch/rdbatch.log
```

Then run `rdbatch list` in another terminal. The log will show:
- API requests and responses (status codes, raw JSON)
- How many torrents were decoded
- Any errors from the provider

### Common Issues

**Missing provider:**
- Ensure `RDBATCH_PROVIDER` is set to either `real-debrid` or `torbox`

**Empty torrent list:**
- Check your API key is valid
- Look at the log file — if the API returns `[]`, your account genuinely has no recent torrents
- If the API returns a 401 error, your API key is invalid or expired

**Downloads fail:**
- Ensure `aria2c` is installed and in your `PATH`
- Check that you have enough fair-use points remaining (Real-Debrid) or active subscription (Torbox)

---

## Project Structure

```
cmd/rdbatch/main.go
internal/
  api/          Provider interface + Real-Debrid & Torbox REST clients
  commands/     Cobra CLI commands
  config/       Provider & API key resolution (env → config file)
  download/     aria2c download manager
  models/       Data structures
  ui/           Bubble Tea interactive list
```

---

## Error Handling

`rdbatch` handles failures gracefully:

- Invalid or expired API keys
- Malformed magnet links
- Missing `aria2c` binary
- Network failures
- Unavailable torrents
- Failed link unrestricting

**One failed download will not abort the entire batch.**

---

## Roadmap

Potential future features (not required for MVP):

- Fuzzy search filter inside TUI (`/` search mode)
- Auto-refresh torrent statuses
- Clipboard integration for magnets
- File-level selection inside the TUI
- `fetch --wait --download` auto-download mode

---

## Contributing

Contributions are welcome! Feel free to open issues or pull requests.

---

## License

MIT License. See [LICENSE](LICENSE) for details.

---

> **Disclaimer:** This project is not affiliated with Real-Debrid or Torbox. Use responsibly and in accordance with each provider's terms of service.
