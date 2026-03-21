package depsdev

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestGetVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// r.URL.Path is already decoded by net/http; check RawPath for encoded form.
		if r.URL.RawPath != "/v3/systems/GO/packages/golang.org%2Fx%2Fnet/versions/v0.17.0" {
			t.Errorf("unexpected raw path: %s", r.URL.RawPath)
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `{
			"versionKey": {"system": "GO", "name": "golang.org/x/net", "version": "v0.17.0"},
			"publishedAt": "2023-10-10T00:00:00Z",
			"advisoryKeys": [{"id": "GHSA-1234"}],
			"relatedProjects": [{"projectKey": {"id": "github.com/golang/net"}, "relationType": "ISSUE_TRACKER"}]
		}`)
	}))
	defer srv.Close()

	c := NewClient(2, srv.URL)
	resp, err := c.GetVersion(context.Background(), "GO", "golang.org/x/net", "v0.17.0")
	if err != nil {
		t.Fatal(err)
	}

	if resp.VersionKey.Name != "golang.org/x/net" {
		t.Errorf("Name = %q, want golang.org/x/net", resp.VersionKey.Name)
	}
	if resp.PublishedAt.Year() != 2023 {
		t.Errorf("PublishedAt year = %d, want 2023", resp.PublishedAt.Year())
	}
	if len(resp.AdvisoryKeys) != 1 {
		t.Fatalf("AdvisoryKeys len = %d, want 1", len(resp.AdvisoryKeys))
	}
	if resp.AdvisoryKeys[0].ID != "GHSA-1234" {
		t.Errorf("AdvisoryKeys[0].ID = %q, want GHSA-1234", resp.AdvisoryKeys[0].ID)
	}
}

func TestGetProject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{
			"projectKey": {"id": "github.com/golang/net"},
			"scorecard": {"overallScore": 7.5, "date": "2023-10-01"}
		}`)
	}))
	defer srv.Close()

	c := NewClient(2, srv.URL)
	resp, err := c.GetProject(context.Background(), "github.com/golang/net")
	if err != nil {
		t.Fatal(err)
	}

	if resp.Scorecard == nil {
		t.Fatal("Scorecard is nil")
	}
	if resp.Scorecard.OverallScore != 7.5 {
		t.Errorf("OverallScore = %f, want 7.5", resp.Scorecard.OverallScore)
	}
}

func TestRetryOn429(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, "rate limited")
			return
		}
		fmt.Fprint(w, `{"versionKey": {"system":"GO","name":"test","version":"v1.0.0"}, "publishedAt": "2023-01-01T00:00:00Z"}`)
	}))
	defer srv.Close()

	c := NewClient(1, srv.URL)
	// Override backoff for faster tests — we test the retry logic, not the timing.
	resp, err := c.GetVersion(context.Background(), "GO", "test", "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if resp.VersionKey.Name != "test" {
		t.Errorf("Name = %q, want test", resp.VersionKey.Name)
	}
	if attempts.Load() != 3 {
		t.Errorf("attempts = %d, want 3", attempts.Load())
	}
}

func TestConcurrencyLimit(t *testing.T) {
	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := concurrent.Add(1)
		for {
			old := maxConcurrent.Load()
			if n <= old || maxConcurrent.CompareAndSwap(old, n) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
		concurrent.Add(-1)
		fmt.Fprint(w, `{"versionKey": {"system":"GO","name":"test","version":"v1.0.0"}, "publishedAt": "2023-01-01T00:00:00Z"}`)
	}))
	defer srv.Close()

	c := NewClient(2, srv.URL)
	ctx := context.Background()

	done := make(chan struct{}, 5)
	for range 5 {
		go func() {
			_, _ = c.GetVersion(ctx, "GO", "test", "v1.0.0")
			done <- struct{}{}
		}()
	}
	for range 5 {
		<-done
	}

	if maxConcurrent.Load() > 2 {
		t.Errorf("max concurrent = %d, want <= 2", maxConcurrent.Load())
	}
}

func TestNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := NewClient(1, srv.URL)
	_, err := c.GetVersion(context.Background(), "GO", "nonexistent", "v1.0.0")
	if err == nil {
		t.Fatal("expected error for 404")
	}
}
