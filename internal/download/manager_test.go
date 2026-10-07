package download

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestBatchDownloadsOverlapAndReportFailure(t *testing.T) {
	if _, err := exec.LookPath("aria2c"); err != nil {
		t.Skip("aria2c not installed")
	}
	var active, peak atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)
			return
		}
		n := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); n > old && !peak.CompareAndSwap(old, n); old = peak.Load() {
		}
		w.Header().Set("Content-Length", "1048576")
		data := make([]byte, 65536)
		for range 16 {
			w.Write(data)
			w.(http.Flusher).Flush()
			time.Sleep(50 * time.Millisecond)
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	dir := t.TempDir()
	var last []Progress
	err := New(0, []string{"-x", "1", "-s", "1", "--max-tries=1"}).Download(ctx, []Item{{URL: server.URL + "/one"}, {URL: server.URL + "/two"}, {URL: server.URL + "/missing"}}, dir, func(rows []Progress) { last = rows })
	if err == nil {
		t.Fatal("missing file should fail the batch")
	}
	if peak.Load() < 2 {
		t.Fatalf("downloads did not overlap: peak %d", peak.Load())
	}
	if len(last) != 3 || last[0].Status != "completed" || last[1].Status != "completed" || last[2].Status != "failed" || last[2].Error == "" {
		t.Fatalf("unexpected final statuses: %+v", last)
	}
	for _, name := range []string{"one", "two"} {
		info, e := os.Stat(filepath.Join(dir, name))
		if e != nil || info.Size() != 1048576 {
			t.Fatalf("invalid downloaded file %s: %v", name, e)
		}
	}
}

func TestCancellationStopsDownload(t *testing.T) {
	if _, err := exec.LookPath("aria2c"); err != nil {
		t.Skip("aria2c not installed")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := New(1, nil).Download(ctx, []Item{{URL: server.URL + "/slow"}}, t.TempDir(), func([]Progress) {})
	if err == nil || time.Since(start) > 5*time.Second {
		t.Fatalf("cancellation did not stop download promptly: %v", err)
	}
}
