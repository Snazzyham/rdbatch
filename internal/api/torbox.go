package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/soham/rdbatch/internal/log"
	"github.com/soham/rdbatch/internal/models"
)

const torboxBaseURL = "https://api.torbox.app/v1/api"

type TorboxClient struct {
	apiKey string
	client *http.Client
}

func NewTorboxClient(apiKey string) *TorboxClient {
	return &TorboxClient{
		apiKey: apiKey,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *TorboxClient) doRequest(method, endpoint string, body io.Reader, contentType string) (*http.Response, error) {
	req, err := http.NewRequest(method, torboxBaseURL+endpoint, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	log.Printf("API %s %s", method, torboxBaseURL+endpoint)
	return c.client.Do(req)
}

// --- Provider interface implementation ---

func (c *TorboxClient) AddMagnet(magnet string) (string, string, string, error) {
	if err := c.ValidateMagnet(magnet); err != nil {
		return "", "", "", err
	}

	data := url.Values{}
	data.Set("magnet", magnet)
	resp, err := c.doRequest("POST", "/torrents/createtorrent", strings.NewReader(data.Encode()), "application/x-www-form-urlencoded")
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	log.Printf("Torbox AddMagnet status=%d body=%s", resp.StatusCode, string(bodyBytes))

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", "", "", fmt.Errorf("failed to add magnet: %s - %s", resp.Status, string(bodyBytes))
	}

	var result models.TorboxCreateTorrentResponse
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return "", "", "", fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.Success {
		return "", "", "", fmt.Errorf("torbox returned unsuccessful response: %s", string(bodyBytes))
	}

	return fmt.Sprintf("%d", result.Data.TorrentID), result.Data.Name, result.Data.DownloadState, nil
}

func (c *TorboxClient) ListTorrents() ([]Torrent, error) {
	resp, err := c.doRequest("GET", "/torrents/mylist?bypass_cache=true", nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	log.Printf("Torbox ListTorrents status=%d len=%d body=%s", resp.StatusCode, len(bodyBytes), string(bodyBytes))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list torrents: %s - %s", resp.Status, string(bodyBytes))
	}

	var result models.TorboxTorrentListResponse
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to decode torrent list: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("torbox returned unsuccessful response: %s", string(bodyBytes))
	}

	// Sort by CreatedAt descending (newest first) and limit to 40
	sort.Slice(result.Data, func(i, j int) bool {
		return result.Data[i].CreatedAt > result.Data[j].CreatedAt
	})

	var torrents []Torrent
	for i, t := range result.Data {
		if i >= 40 {
			break
		}
		torrents = append(torrents, Torrent{
			ID:     fmt.Sprintf("%d", t.ID),
			Name:   t.Name,
			Status: t.DownloadState,
			Size:   t.Size,
			Added:  time.Unix(t.CreatedAt, 0),
		})
	}

	return torrents, nil
}

func (c *TorboxClient) GetDownloadLinks(torrentID string) ([]string, error) {
	// Step A: Get file list
	endpoint := fmt.Sprintf("/torrents/mylist?id=%s&bypass_cache=true", url.QueryEscape(torrentID))
	resp, err := c.doRequest("GET", endpoint, nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	log.Printf("Torbox GetDownloadLinks (info) status=%d body=%s", resp.StatusCode, string(bodyBytes))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get torrent info: %s - %s", resp.Status, string(bodyBytes))
	}

	var result models.TorboxTorrentDetailResponse
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to decode torrent detail: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("torbox returned unsuccessful response: %s", string(bodyBytes))
	}

	if len(result.Data.Files) == 0 {
		return nil, fmt.Errorf("no files found for torrent %s", torrentID)
	}

	// Step B: Request download URL per file
	var urls []string
	for _, file := range result.Data.Files {
		dlURL, err := c.requestDownloadURL(torrentID, fmt.Sprintf("%d", file.ID))
		if err != nil {
			log.Printf("Torbox requestdl failed for file %d: %v", file.ID, err)
			continue
		}
		urls = append(urls, dlURL)
	}

	if len(urls) == 0 {
		return nil, fmt.Errorf("no download URLs obtained for torrent %s", torrentID)
	}

	return urls, nil
}

func (c *TorboxClient) requestDownloadURL(torrentID, fileID string) (string, error) {
	data := url.Values{}
	data.Set("token", c.apiKey)
	data.Set("torrent_id", torrentID)
	data.Set("file_id", fileID)

	resp, err := c.doRequest("POST", "/torrents/requestdl", strings.NewReader(data.Encode()), "application/x-www-form-urlencoded")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	log.Printf("Torbox requestdl status=%d body=%s", resp.StatusCode, string(bodyBytes))

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("failed to request download link: %s - %s", resp.Status, string(bodyBytes))
	}

	var result models.TorboxRequestDLResponse
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return "", fmt.Errorf("failed to decode requestdl response: %w", err)
	}

	if !result.Success {
		return "", fmt.Errorf("torbox returned unsuccessful requestdl response: %s", string(bodyBytes))
	}

	return result.Data, nil
}

func (c *TorboxClient) Aria2Flags() []string {
	return []string{"-x", "1", "-s", "1", "-k", "1M", "--continue=true", "--auto-file-renaming=false"}
}

func (c *TorboxClient) ValidateMagnet(magnet string) error {
	if !strings.HasPrefix(magnet, "magnet:?") {
		return fmt.Errorf("invalid magnet link: must start with 'magnet:?'")
	}
	if !strings.Contains(magnet, "xt=urn:btih:") {
		return fmt.Errorf("invalid magnet link: missing btih hash")
	}
	return nil
}
