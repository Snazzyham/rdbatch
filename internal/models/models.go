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

type Config struct {
	APIKey string `json:"api_key"`
}
