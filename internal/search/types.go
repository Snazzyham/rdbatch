package search

// TitleResult represents a movie or series from Cinemeta
type TitleResult struct {
	ID          string // IMDB ID
	Type        string // "movie" or "series"
	Name        string
	ReleaseInfo string // Year or year range
}

// Episode represents a TV episode
type Episode struct {
	ID      string // Full ID like "tt0903747:1:1"
	Name    string
	Season  int
	Episode int
}

// SeasonGroup groups episodes by season
type SeasonGroup struct {
	Season   int
	Episodes []Episode
}

// CacheState indicates whether a torrent is cached on the debrid service
type CacheState int

const (
	Cached   CacheState = iota // ⚡ - cached on debrid
	Uncached                   // ⬇️ or 🧲 - uncached or P2P fallback
)

// TorrentResult represents a scraped torrent from Comet
type TorrentResult struct {
	Name        string     // Raw name field, e.g. "[TB⚡] Comet 1080p"
	Description string     // Multi-line description
	InfoHash    string     // 40-char hex
	StreamURL   string     // Comet playback proxy URL
	SizeBytes   int64      // from behaviorHints.videoSize
	Cache       CacheState // Cached or Uncached
}

// Meta represents series metadata from Cinemeta
type Meta struct {
	Name   string
	Videos []Video
}

// Video represents an episode in series metadata
type Video struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Season  int    `json:"season"`
	Episode int    `json:"episode"`
}

// cinemetaSearchResponse represents the response from Cinemeta search
type cinemetaSearchResponse struct {
	Metas []cinemetaMeta `json:"metas"`
}

// cinemetaMetaResponse represents the response from Cinemeta meta endpoint
type cinemetaMetaResponse struct {
	Meta cinemetaMetaDetail `json:"meta"`
}

// cinemetaMeta represents a single title in search results
type cinemetaMeta struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Name        string `json:"name"`
	ReleaseInfo string `json:"releaseInfo"`
}

// cinemetaMetaDetail represents detailed metadata for a series
type cinemetaMetaDetail struct {
	Name   string          `json:"name"`
	Videos []cinemetaVideo `json:"videos"`
}

// cinemetaVideo represents an episode in the meta response
type cinemetaVideo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Season  int    `json:"season"`
	Episode int    `json:"episode"`
}

// cometStreamResponse represents the response from Comet streams endpoint
type cometStreamResponse struct {
	Streams []cometStream `json:"streams"`
}

// cometStream represents a single stream in the Comet response
type cometStream struct {
	Name          string             `json:"name"`
	Description   string             `json:"description"`
	URL           string             `json:"url"`
	InfoHash      string             `json:"infoHash"`
	BehaviorHints cometBehaviorHints `json:"behaviorHints"`
}

// cometBehaviorHints contains additional stream metadata
type cometBehaviorHints struct {
	Filename  string `json:"filename"`
	VideoSize int64  `json:"videoSize"`
}
