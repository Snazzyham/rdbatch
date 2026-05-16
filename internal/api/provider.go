package api

import "time"

// Torrent is a provider-agnostic representation of a torrent.
type Torrent struct {
	ID     string
	Name   string
	Status string
	Size   int64
	Added  time.Time
	Links  []string
}

// File is a provider-agnostic representation of a file within a torrent.
type File struct {
	ID   string
	Name string
	Size int64
}

// Provider is the interface implemented by all backend providers.
type Provider interface {
	AddMagnet(magnet string) (id string, name string, status string, err error)
	ListTorrents() ([]Torrent, error)
	ListFiles(torrentID string) ([]File, error)
	GetDownloadLinks(torrentID string, fileIDs []string) ([]string, error)
	GetStreamLink(torrentID string, fileID string) (string, error)
	Aria2Flags() []string
}
