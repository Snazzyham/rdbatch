package search

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	cometTimeout = 30 * time.Second
)

// Comet is a client for a Comet scraper instance
type Comet struct {
	client     *http.Client
	streamBase string // Base URL for stream endpoints (manifest URL minus /manifest.json)
}

// NewComet creates a new Comet client from a manifest URL.
// The URL must end in /manifest.json.
func NewComet(manifestURL string) (*Comet, error) {
	if !strings.HasSuffix(manifestURL, "/manifest.json") {
		return nil, fmt.Errorf("comet URL must end in /manifest.json")
	}

	streamBase := strings.TrimSuffix(manifestURL, "/manifest.json")

	return &Comet{
		client: &http.Client{
			Timeout: cometTimeout,
		},
		streamBase: streamBase,
	}, nil
}

// Streams fetches torrent streams for a movie or series episode.
// For movies: mediaID is the IMDB ID (e.g., "tt0133093")
// For series: mediaID is "imdbID:season:episode" (e.g., "tt0903747:1:1")
func (c *Comet) Streams(ctx context.Context, mediaType, mediaID string) ([]TorrentResult, error) {
	var endpoint string
	if mediaType == "movie" {
		endpoint = fmt.Sprintf("%s/stream/movie/%s.json", c.streamBase, mediaID)
	} else {
		endpoint = fmt.Sprintf("%s/stream/series/%s.json", c.streamBase, mediaID)
	}

	// Ensure proper URL encoding
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parsing URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching streams: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("comet returned status %d", resp.StatusCode)
	}

	var result cometStreamResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return c.processStreams(result.Streams), nil
}

func (c *Comet) processStreams(streams []cometStream) []TorrentResult {
	results := make([]TorrentResult, 0, len(streams))

	for _, s := range streams {
		// Skip informational/error streams
		if c.isInfoStream(s) {
			continue
		}

		cacheState := c.classifyCache(s.Name)
		infoHash := c.extractInfoHash(s)

		// Skip if we couldn't extract an info hash
		if infoHash == "" {
			continue
		}

		results = append(results, TorrentResult{
			Name:        s.Name,
			Description: s.Description,
			InfoHash:    infoHash,
			StreamURL:   s.URL,
			SizeBytes:   s.BehaviorHints.VideoSize,
			Cache:       cacheState,
		})
	}

	return results
}

// isInfoStream returns true for Comet informational/error streams that should be filtered out
func (c *Comet) isInfoStream(s cometStream) bool {
	// Filter out sync/error streams
	if strings.Contains(s.URL, "/debrid-sync/") {
		return true
	}

	// Filter out streams with error indicators in name
	if strings.Contains(s.Name, "🔄") || strings.Contains(s.Name, "❌") || strings.Contains(s.Name, "⚠️") {
		return true
	}

	return false
}

// classifyCache determines the cache state from the stream name
func (c *Comet) classifyCache(name string) CacheState {
	// ⚡ (U+26A1) = cached
	if strings.Contains(name, "\u26a1") {
		return Cached
	}

	// ⬇️ (U+2B07 U+FE0F) or 🧲 (U+1F9F2) = uncached/P2P
	// Treat both as uncached
	return Uncached
}

// extractInfoHash extracts the info hash from a stream
func (c *Comet) extractInfoHash(s cometStream) string {
	// If there's a top-level infoHash field, use it
	if s.InfoHash != "" {
		return s.InfoHash
	}

	// Otherwise parse from URL: /playback/{info_hash}/...
	if s.URL == "" {
		return ""
	}

	// Parse the URL path
	parsedURL, err := url.Parse(s.URL)
	if err != nil {
		return ""
	}

	// Split path and find the info hash (3rd segment after /playback/)
	parts := strings.Split(parsedURL.Path, "/")
	for i, part := range parts {
		if part == "playback" && i+1 < len(parts) {
			hash := parts[i+1]
			// Validate it's a 40-char hex string
			if len(hash) == 40 && isHex(hash) {
				return hash
			}
		}
	}

	return ""
}

// isHex returns true if s is a valid hexadecimal string
func isHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// GetDebridService extracts the debrid service name from the manifest URL
// This is used to show a warning if it doesn't match RDBATCH_PROVIDER
func (c *Comet) GetDebridService() string {
	// Try to detect from the URL - common patterns:
	// - comet URLs often contain encoded config with service info
	// We can't reliably extract this without decoding the base64 config,
	// but we can check for common patterns in the response names
	return "unknown"
}
