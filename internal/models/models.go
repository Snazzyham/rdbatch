package models

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// RDTime handles Real-Debrid's inconsistent timestamp formats
type RDTime struct {
	time.Time
}

func (t *RDTime) UnmarshalJSON(data []byte) error {
	// Try integer Unix timestamp first
	var ts int64
	if err := json.Unmarshal(data, &ts); err == nil {
		t.Time = time.Unix(ts, 0)
		return nil
	}

	// Try string
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		// Try RFC3339 / ISO8601
		if parsed, err := time.Parse(time.RFC3339Nano, s); err == nil {
			t.Time = parsed
			return nil
		}
		if parsed, err := time.Parse(time.RFC3339, s); err == nil {
			t.Time = parsed
			return nil
		}
		// Try Unix timestamp as string
		if ts, err := strconv.ParseInt(s, 10, 64); err == nil {
			t.Time = time.Unix(ts, 0)
			return nil
		}
	}

	return fmt.Errorf("cannot parse RDTime: %s", string(data))
}

func (t RDTime) Format(layout string) string {
	if t.IsZero() {
		return "unknown"
	}
	return t.Time.Format(layout)
}

// Config holds the parsed configuration for rdbatch.
type Config struct {
	Provider         string `json:"provider"`
	RealDebridAPIKey string `json:"realdebrid_api_key"`
	TorboxAPIKey     string `json:"torbox_api_key"`
	// APIKey is kept for backward compatibility (maps to RealDebridAPIKey)
	APIKey string `json:"api_key"`
}

// --- Real-Debrid specific models ---

type Torrent struct {
	ID       string   `json:"id"`
	Filename string   `json:"filename"`
	Status   string   `json:"status"`
	Bytes    int64    `json:"bytes"`
	Added    RDTime   `json:"added"`
	Links    []string `json:"links"`
}

type TorrentInfo struct {
	ID       string   `json:"id"`
	Filename string   `json:"filename"`
	Status   string   `json:"status"`
	Bytes    int64    `json:"bytes"`
	Added    RDTime   `json:"added"`
	Links    []string `json:"links"`
	Files    []File   `json:"files"`
}

type File struct {
	ID       int    `json:"id"`
	Path     string `json:"path"`
	Bytes    int64  `json:"bytes"`
	Selected int    `json:"selected"`
}

type UnrestrictedLink struct {
	ID        string `json:"id"`
	Filename  string `json:"filename"`
	Download  string `json:"download"`
	Filesize  int64  `json:"filesize"`
	Link      string `json:"link"`
	Host      string `json:"host"`
	Chunks    int    `json:"chunks"`
	CRC       int    `json:"crc"`
	Supported int    `json:"supported"`
}

type MagnetRequest struct {
	Magnet string `json:"magnet"`
}

type MagnetResponse struct {
	ID  string `json:"id"`
	URI string `json:"uri"`
}

type AddMagnetResponse struct {
	ID  string `json:"id"`
	URI string `json:"uri"`
}

// --- Torbox specific models ---

type TorboxCreateTorrentResponse struct {
	Success bool                   `json:"success"`
	Data    TorboxTorrentCreated   `json:"data"`
}

type TorboxTorrentCreated struct {
	TorrentID     int    `json:"torrent_id"`
	Name          string `json:"name"`
	DownloadState string `json:"download_state"`
}

type TorboxTorrent struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	DownloadState string `json:"download_state"`
	Size          int64  `json:"size"`
	CreatedAt     int64  `json:"created_at"`
}

type TorboxTorrentListResponse struct {
	Success bool            `json:"success"`
	Data    []TorboxTorrent `json:"data"`
}

type TorboxTorrentDetailResponse struct {
	Success bool          `json:"success"`
	Data    TorboxTorrentDetail `json:"data"`
}

type TorboxTorrentDetail struct {
	ID            int              `json:"id"`
	Name          string           `json:"name"`
	DownloadState string           `json:"download_state"`
	Size          int64            `json:"size"`
	CreatedAt     int64            `json:"created_at"`
	Files         []TorboxFile     `json:"files"`
}

type TorboxFile struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}

type TorboxRequestDLResponse struct {
	Success bool   `json:"success"`
	Data    string `json:"data"`
}
