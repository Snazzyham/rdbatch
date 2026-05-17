package search

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/sync/errgroup"
)

const (
	cinemetaBaseURL = "https://v3-cinemeta.strem.io"
	cinemetaTimeout = 10 * time.Second
)

// Cinemeta is a client for the Cinemeta API
type Cinemeta struct {
	client *http.Client
}

// NewCinemeta creates a new Cinemeta client
func NewCinemeta() *Cinemeta {
	return &Cinemeta{
		client: &http.Client{
			Timeout: cinemetaTimeout,
		},
	}
}

// Search queries Cinemeta for movies and series matching the query.
// It searches both catalogs in parallel and merges results.
func (c *Cinemeta) Search(ctx context.Context, query string) ([]TitleResult, error) {
	encodedQuery := url.QueryEscape(query)

	var movieResp, seriesResp cinemetaSearchResponse
	var movieErr, seriesErr error

	g, ctx := errgroup.WithContext(ctx)

	// Search movies
	g.Go(func() error {
		movieResp, movieErr = c.searchCatalog(ctx, "movie", encodedQuery)
		return movieErr
	})

	// Search series
	g.Go(func() error {
		seriesResp, seriesErr = c.searchCatalog(ctx, "series", encodedQuery)
		return seriesErr
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Merge results, preserving API order (already ranked by relevance)
	results := make([]TitleResult, 0, len(movieResp.Metas)+len(seriesResp.Metas))

	for _, m := range movieResp.Metas {
		results = append(results, TitleResult{
			ID:          m.ID,
			Type:        m.Type,
			Name:        m.Name,
			ReleaseInfo: m.ReleaseInfo,
		})
	}

	for _, m := range seriesResp.Metas {
		results = append(results, TitleResult{
			ID:          m.ID,
			Type:        m.Type,
			Name:        m.Name,
			ReleaseInfo: m.ReleaseInfo,
		})
	}

	return results, nil
}

// Meta fetches detailed metadata for a series (used for season/episode lists)
func (c *Cinemeta) Meta(ctx context.Context, imdbID string) (Meta, error) {
	url := fmt.Sprintf("%s/meta/series/%s.json", cinemetaBaseURL, imdbID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Meta{}, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return Meta{}, fmt.Errorf("fetching meta: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Meta{}, fmt.Errorf("cinemeta returned status %d", resp.StatusCode)
	}

	var result cinemetaMetaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return Meta{}, fmt.Errorf("decoding response: %w", err)
	}

	videos := make([]Video, len(result.Meta.Videos))
	for i, v := range result.Meta.Videos {
		videos[i] = Video{
			ID:      v.ID,
			Name:    v.Name,
			Season:  v.Season,
			Episode: v.Episode,
		}
	}

	return Meta{
		Name:   result.Meta.Name,
		Videos: videos,
	}, nil
}

func (c *Cinemeta) searchCatalog(ctx context.Context, catalogType, encodedQuery string) (cinemetaSearchResponse, error) {
	url := fmt.Sprintf("%s/catalog/%s/top/search=%s.json", cinemetaBaseURL, catalogType, encodedQuery)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return cinemetaSearchResponse{}, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return cinemetaSearchResponse{}, fmt.Errorf("fetching %s catalog: %w", catalogType, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return cinemetaSearchResponse{}, fmt.Errorf("cinemeta returned status %d", resp.StatusCode)
	}

	var result cinemetaSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return cinemetaSearchResponse{}, fmt.Errorf("decoding response: %w", err)
	}

	return result, nil
}
