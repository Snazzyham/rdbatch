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
		return result.Data[i].CreatedAt.Unix() > result.Data[j].CreatedAt.Unix()
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
			Added:  t.CreatedAt.Time,
		})
	}

	return torrents, nil
}

func (c *TorboxClient) ListFiles(torrentID string) ([]File, error) {
	endpoint := fmt.Sprintf("/torrents/mylist?id=%s&bypass_cache=true", url.QueryEscape(torrentID))
	resp, err := c.doRequest("GET", endpoint, nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	log.Printf("Torbox ListFiles status=%d body=%s", resp.StatusCode, string(bodyBytes))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get torrent files: %s - %s", resp.Status, string(bodyBytes))
	}

	var result models.TorboxTorrentDetailResponse
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to decode torrent detail: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("torbox returned unsuccessful response: %s", string(bodyBytes))
	}

	var files []File
	for _, f := range result.Data.Files {
		files = append(files, File{
			ID:   fmt.Sprintf("%d", f.ID),
			Name: f.Name,
			Size: f.Size,
		})
	}

	return files, nil
}

func (c *TorboxClient) GetDownloadLinks(torrentID string, fileIDs []string) ([]string, error) {
	var targetFiles []string

	if len(fileIDs) > 0 {
		targetFiles = fileIDs
	} else {
		// If no fileIDs provided, get all files
		files, err := c.ListFiles(torrentID)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			targetFiles = append(targetFiles, f.ID)
		}
	}

	if len(targetFiles) == 0 {
		return nil, fmt.Errorf("no files selected for torrent %s", torrentID)
	}

	// Step B: Request download URL per file
	var urls []string
	for _, fileID := range targetFiles {
		dlURL, err := c.requestDownloadURL(torrentID, fileID)
		if err != nil {
			log.Printf("Torbox requestdl failed for file %s: %v", fileID, err)
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
	params := url.Values{}
	params.Set("token", c.apiKey)
	params.Set("torrent_id", torrentID)
	params.Set("file_id", fileID)

	endpoint := fmt.Sprintf("/torrents/requestdl?%s", params.Encode())
	resp, err := c.doRequest("GET", endpoint, nil, "")
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

func (c *TorboxClient) GetStreamLink(torrentID string, fileID string) (string, error) {
	if fileID != "" {
		return c.requestDownloadURL(torrentID, fileID)
	}

	// No fileID provided, find the largest file
	files, err := c.ListFiles(torrentID)
	if err != nil {
		return "", err
	}

	if len(files) == 0 {
		return "", fmt.Errorf("no files found in torrent")
	}

	var largest File
	for _, f := range files {
		if f.Size > largest.Size {
			largest = f
		}
	}

	return c.requestDownloadURL(torrentID, largest.ID)
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
