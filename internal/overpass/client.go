// Package overpass implements a client for the Overpass API, scoped to
// fetching amenity=bench nodes for a bounding box. This is the live,
// per-cell demand fill described in
// .claude/architecture/designs/1-2-cold-start-ingestion.md — never a bulk
// import (see 1-ingestion-pipeline D12).
package overpass

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sanderdejongg/Benchfinder/internal/h3grid"
)

// DefaultBaseURL is the public Overpass API interpreter endpoint.
const DefaultBaseURL = "https://overpass-api.de/api/interpreter"

// RequestTimeout bounds the Overpass HTTP call. It's independent of, and in
// addition to, the 3s PostGIS query deadline in 2-nearby-search — see
// 1-2-cold-start-ingestion D3. Composed worst-case cold-start latency is the
// sum of both.
const RequestTimeout = 5 * time.Second

// Node is an OSM node returned by Overpass, filtered to amenity=bench.
type Node struct {
	ID   int64
	Lat  float64
	Lon  float64
	Tags map[string]string
}

// Client queries the Overpass API for amenity=bench nodes.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient returns a Client querying baseURL. Every call is bounded by
// RequestTimeout.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: RequestTimeout},
	}
}

// FetchBenches returns every amenity=bench node within bbox.
func (c *Client) FetchBenches(ctx context.Context, bbox h3grid.BBox) ([]Node, error) {
	ctx, cancel := context.WithTimeout(ctx, RequestTimeout)
	defer cancel()

	query := fmt.Sprintf(
		`[out:json][timeout:25];node["amenity"="bench"](%f,%f,%f,%f);out body;`,
		bbox.South, bbox.West, bbox.North, bbox.East,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL,
		strings.NewReader("data="+url.QueryEscape(query)))
	if err != nil {
		return nil, fmt.Errorf("overpass: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("overpass: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("overpass: unexpected status %d: %s", resp.StatusCode, body)
	}

	var parsed struct {
		Elements []struct {
			Type string            `json:"type"`
			ID   int64             `json:"id"`
			Lat  float64           `json:"lat"`
			Lon  float64           `json:"lon"`
			Tags map[string]string `json:"tags"`
		} `json:"elements"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("overpass: decode response: %w", err)
	}

	nodes := make([]Node, 0, len(parsed.Elements))
	for _, el := range parsed.Elements {
		if el.Type != "node" {
			continue
		}
		nodes = append(nodes, Node{ID: el.ID, Lat: el.Lat, Lon: el.Lon, Tags: el.Tags})
	}
	return nodes, nil
}
