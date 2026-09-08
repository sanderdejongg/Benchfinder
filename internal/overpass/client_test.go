package overpass_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sanderdejongg/Benchfinder/internal/h3grid"
	"github.com/sanderdejongg/Benchfinder/internal/overpass"
)

func TestClientFetchBenches(t *testing.T) {
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("server: parse form: %v", err)
		}
		gotBody = r.FormValue("data")

		fmt.Fprint(w, `{
			"elements": [
				{"type": "node", "id": 1, "lat": 52.37, "lon": 4.89, "tags": {"amenity": "bench", "backrest": "yes"}},
				{"type": "node", "id": 2, "lat": 52.38, "lon": 4.90, "tags": {}},
				{"type": "way", "id": 3, "lat": 52.39, "lon": 4.91}
			]
		}`)
	}))
	defer server.Close()

	client := overpass.NewClient(server.URL)
	bbox := h3grid.BBox{South: 52.30, West: 4.80, North: 52.40, East: 4.95}

	nodes, err := client.FetchBenches(context.Background(), bbox)
	if err != nil {
		t.Fatalf("FetchBenches returned error: %v", err)
	}

	if len(nodes) != 2 {
		t.Fatalf("FetchBenches returned %d nodes, want 2 (way element should be filtered out)", len(nodes))
	}
	if nodes[0].ID != 1 || nodes[0].Tags["backrest"] != "yes" {
		t.Errorf("nodes[0] = %+v, want id 1 with backrest=yes", nodes[0])
	}
	if nodes[1].ID != 2 {
		t.Errorf("nodes[1] = %+v, want id 2", nodes[1])
	}

	for _, want := range []string{"52.300000", "4.800000", "52.400000", "4.950000"} {
		if !strings.Contains(gotBody, want) {
			t.Errorf("request body %q does not contain bbox value %q", gotBody, want)
		}
	}
}

func TestClientFetchBenchesErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, "rate limited")
	}))
	defer server.Close()

	client := overpass.NewClient(server.URL)
	_, err := client.FetchBenches(context.Background(), h3grid.BBox{})
	if err == nil {
		t.Fatal("FetchBenches returned no error for a non-200 response")
	}
}

func TestClientFetchBenchesMalformedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "not json")
	}))
	defer server.Close()

	client := overpass.NewClient(server.URL)
	_, err := client.FetchBenches(context.Background(), h3grid.BBox{})
	if err == nil {
		t.Fatal("FetchBenches returned no error for a malformed response body")
	}
}
