package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/soham/rdbatch/internal/log"
	"github.com/soham/rdbatch/internal/models"
)

const baseURL = "https://api.real-debrid.com/rest/1.0"

type Client struct {
	apiKey string
	client *http.Client
}

func New(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) doRequest(method, endpoint string, body io.Reader, contentType string) (*http.Response, error) {
	req, err := http.NewRequest(method, baseURL+endpoint, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	log.Printf("API %s %s", method, baseURL+endpoint)
	return c.client.Do(req)
}

func (c *Client) AddMagnet(magnet string) (*models.AddMagnetResponse, error) {
	data := url.Values{}
	data.Set("magnet", magnet)
	resp, err := c.doRequest("POST", "/torrents/addMagnet", strings.NewReader(data.Encode()), "application/x-www-form-urlencoded")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	log.Printf("AddMagnet status=%d body=%s", resp.StatusCode, string(bodyBytes))

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to add magnet: %s - %s", resp.Status, string(bodyBytes))
	}

	var result models.AddMagnetResponse
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

func (c *Client) SelectFiles(id, files string) error {
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

func (c *Client) GetTorrents() ([]models.Torrent, error) {
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

func (c *Client) GetTorrentInfo(id string) (*models.TorrentInfo, error) {
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

func (c *Client) UnrestrictLink(link string) (*models.UnrestrictedLink, error) {
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

func (c *Client) ValidateMagnet(magnet string) error {
	if !strings.HasPrefix(magnet, "magnet:?") {
		return fmt.Errorf("invalid magnet link: must start with 'magnet:?'")
	}
	if !strings.Contains(magnet, "xt=urn:btih:") {
		return fmt.Errorf("invalid magnet link: missing btih hash")
	}
	return nil
}
