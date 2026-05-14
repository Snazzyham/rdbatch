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

// Provider is the interface implemented by all backend providers.
type Provider interface {
	AddMagnet(magnet string) (id string, name string, status string, err error)
	ListTorrents() ([]Torrent, error)
	GetDownloadLinks(torrentID string) ([]string, error)
	Aria2Flags() []string
}
