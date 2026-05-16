package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/soham/rdbatch/internal/log"
	"github.com/soham/rdbatch/internal/models"
)

const rdBaseURL = "https://api.real-debrid.com/rest/1.0"

type RealDebridClient struct {
	apiKey string
	client *http.Client
}

func NewRealDebridClient(apiKey string) *RealDebridClient {
	return &RealDebridClient{
		apiKey: apiKey,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *RealDebridClient) doRequest(method, endpoint string, body io.Reader, contentType string) (*http.Response, error) {
	req, err := http.NewRequest(method, rdBaseURL+endpoint, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	log.Printf("API %s %s", method, rdBaseURL+endpoint)
	return c.client.Do(req)
}

// --- Provider interface implementation ---

func (c *RealDebridClient) AddMagnet(magnet string) (string, string, string, error) {
	if err := c.ValidateMagnet(magnet); err != nil {
		return "", "", "", err
	}

	data := url.Values{}
	data.Set("magnet", magnet)
	resp, err := c.doRequest("POST", "/torrents/addMagnet", strings.NewReader(data.Encode()), "application/x-www-form-urlencoded")
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	log.Printf("AddMagnet status=%d body=%s", resp.StatusCode, string(bodyBytes))

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", "", "", fmt.Errorf("failed to add magnet: %s - %s", resp.Status, string(bodyBytes))
	}

	var result models.AddMagnetResponse
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return "", "", "", fmt.Errorf("failed to decode response: %w", err)
	}

	// Auto-select video files
	if err := c.autoSelectVideoFiles(result.ID); err != nil {
		log.Printf("AddMagnet: auto-select warning: %v", err)
	}

	// Get info for name/status
	info, err := c.GetTorrentInfo(result.ID)
	name := magnet
	status := "magnet_conversion"
	if err == nil {
		if info.Filename != "" {
			name = info.Filename
		}
		if info.Status != "" {
			status = info.Status
		}
	}

	return result.ID, name, status, nil
}

func (c *RealDebridClient) ListTorrents() ([]Torrent, error) {
	rdTorrents, err := c.GetTorrents()
	if err != nil {
		return nil, err
	}

	torrents := make([]Torrent, len(rdTorrents))
	for i, t := range rdTorrents {
		torrents[i] = Torrent{
			ID:     t.ID,
			Name:   t.Filename,
			Status: t.Status,
			Size:   t.Bytes,
			Added:  t.Added.Time,
			Links:  t.Links,
		}
	}
	return torrents, nil
}

func (c *RealDebridClient) ListFiles(torrentID string) ([]File, error) {
	return nil, fmt.Errorf("listing files is not supported for Real-Debrid yet")
}

func (c *RealDebridClient) GetDownloadLinks(torrentID string, fileIDs []string) ([]string, error) {
	info, err := c.GetTorrentInfo(torrentID)
	if err != nil {
		return nil, err
	}

	if len(info.Links) == 0 {
		return nil, fmt.Errorf("no links available for torrent %s", torrentID)
	}

	var urls []string
	for _, link := range info.Links {
		unrestricted, err := c.UnrestrictLink(link)
		if err != nil {
			log.Printf("GetDownloadLinks: unrestrict failed for %s: %v", link, err)
			continue
		}
		urls = append(urls, unrestricted.Download)
	}

	if len(urls) == 0 {
		return nil, fmt.Errorf("no unrestricted links for torrent %s", torrentID)
	}

	return urls, nil
}

// --- Internal Real-Debrid methods ---

func (c *RealDebridClient) autoSelectVideoFiles(id string) error {
	// Poll for torrent info up to 60 seconds
	var info *models.TorrentInfo
	for i := 0; i < 30; i++ {
		var err error
		info, err = c.GetTorrentInfo(id)
		if err == nil && len(info.Files) > 0 {
			break
		}
		time.Sleep(2 * time.Second)
	}

	if info == nil || len(info.Files) == 0 {
		return fmt.Errorf("torrent files not available yet")
	}

	var videoIDs []string
	for _, f := range info.Files {
		if isVideoFile(f.Path) {
			videoIDs = append(videoIDs, fmt.Sprintf("%d", f.ID))
		}
	}

	if len(videoIDs) == 0 {
		return c.SelectFiles(id, "all")
	}

	return c.SelectFiles(id, strings.Join(videoIDs, ","))
}

func isVideoFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v", ".mpeg", ".mpg", ".ts", ".m2ts":
		return true
	}
	return false
}

func (c *RealDebridClient) SelectFiles(id, files string) error {
	data := url.Values{}
	data.Set("files", files)
	resp, err := c.doRequest("POST", fmt.Sprintf("/torrents/selectFiles/%s", id), strings.NewReader(data.Encode()), "application/x-www-form-urlencoded")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	log.Printf("SelectFiles status=%d body=%s", resp.StatusCode, string(bodyBytes))

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to select files: %s - %s", resp.Status, string(bodyBytes))
	}
	return nil
}

func (c *RealDebridClient) GetTorrents() ([]models.Torrent, error) {
	resp, err := c.doRequest("GET", "/torrents?limit=40&page=1", nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	log.Printf("GetTorrents status=%d len=%d body=%s", resp.StatusCode, len(bodyBytes), string(bodyBytes))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get torrents: %s - %s", resp.Status, string(bodyBytes))
	}

	var torrents []models.Torrent
	if err := json.Unmarshal(bodyBytes, &torrents); err != nil {
		return nil, fmt.Errorf("failed to decode torrents: %w", err)
	}
	log.Printf("GetTorrents decoded %d torrents", len(torrents))
	return torrents, nil
}

func (c *RealDebridClient) GetTorrentInfo(id string) (*models.TorrentInfo, error) {
	resp, err := c.doRequest("GET", fmt.Sprintf("/torrents/info/%s", id), nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	log.Printf("GetTorrentInfo status=%d body=%s", resp.StatusCode, string(bodyBytes))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get torrent info: %s - %s", resp.Status, string(bodyBytes))
	}

	var info models.TorrentInfo
	if err := json.Unmarshal(bodyBytes, &info); err != nil {
		return nil, fmt.Errorf("failed to decode torrent info: %w", err)
	}
	return &info, nil
}

func (c *RealDebridClient) UnrestrictLink(link string) (*models.UnrestrictedLink, error) {
	data := url.Values{}
	data.Set("link", link)
	resp, err := c.doRequest("POST", "/unrestrict/link", strings.NewReader(data.Encode()), "application/x-www-form-urlencoded")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	log.Printf("UnrestrictLink status=%d body=%s", resp.StatusCode, string(bodyBytes))

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("failed to unrestrict link: %s - %s", resp.Status, string(bodyBytes))
	}

	var result models.UnrestrictedLink
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to decode unrestricted link: %w", err)
	}
	return &result, nil
}

func (c *RealDebridClient) GetStreamLink(torrentID string, fileID string) (string, error) {
	info, err := c.GetTorrentInfo(torrentID)
	if err != nil {
		return "", err
	}

	if len(info.Links) == 0 {
		return "", fmt.Errorf("no links available for torrent %s", torrentID)
	}

	// Real-Debrid doesn't expose a simple way to map fileID to links here without more logic,
	// but usually the first link or the one corresponding to the largest file is what we want.
	// Since GetDownloadLinks already unrestricted all, but here we just want one.

	link := info.Links[0]
	// If there are multiple links, we could try to find the "best" one, but for now info.Links[0] is usually the main file.

	unrestricted, err := c.UnrestrictLink(link)
	if err != nil {
		return "", err
	}

	return unrestricted.Download, nil
}

func (c *RealDebridClient) Aria2Flags() []string {
	return []string{"-x", "16", "-s", "16", "-k", "1M", "--continue=true", "--auto-file-renaming=false"}
}

func (c *RealDebridClient) ValidateMagnet(magnet string) error {
	if !strings.HasPrefix(magnet, "magnet:?") {
		return fmt.Errorf("invalid magnet link: must start with 'magnet:?'")
	}
	if !strings.Contains(magnet, "xt=urn:btih:") {
		return fmt.Errorf("invalid magnet link: missing btih hash")
	}
	return nil
}
