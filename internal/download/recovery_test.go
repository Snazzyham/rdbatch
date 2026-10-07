package download

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func requireAria2(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("aria2c"); err != nil {
		t.Skip("aria2c not installed")
	}
}

func TestRateLimitRetriesAndCompletes(t *testing.T) {
	requireAria2(t)
	var attempts atomic.Int32
	content := []byte("successful download after rate limiting")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Write(content)
	}))
	defer server.Close()
	manager := New(1, []string{"-x", "1", "-s", "1", "--max-tries=1"})
	manager.retryWait = 10 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	dir := t.TempDir()
	sawRetry := false
	var last []Progress
	err := manager.Download(ctx, []Item{{URL: server.URL + "/movie.bin"}}, dir, func(rows []Progress) {
		last = rows
		if rows[0].Status == "retrying" {
			sawRetry = true
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !sawRetry || last[0].Status != "completed" || attempts.Load() != 3 {
		t.Fatalf("unexpected recovery: attempts=%d retry=%v states=%+v", attempts.Load(), sawRetry, last)
	}
	got, err := os.ReadFile(filepath.Join(dir, "movie.bin"))
	if err != nil || !bytes.Equal(got, content) {
		t.Fatalf("download contents: %q, %v", got, err)
	}
}

func TestRateLimitRetryLimit(t *testing.T) {
	requireAria2(t)
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	manager := New(1, []string{"--max-tries=1"})
	manager.retryWait = time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var last []Progress
	err := manager.Download(ctx, []Item{{URL: server.URL + "/limited.bin"}}, t.TempDir(), func(rows []Progress) { last = rows })
	if err == nil || attempts.Load() != 6 || last[0].Status != "failed" || !strings.Contains(last[0].Error, "status=429") {
		t.Fatalf("retry limit not enforced: attempts=%d states=%+v err=%v", attempts.Load(), last, err)
	}
}

func TestProgressTimeoutDoesNotAbortBatch(t *testing.T) {
	var polls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if polls.Add(1) == 1 {
			time.Sleep(100 * time.Millisecond)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": "rdbatch", "result": map[string]any{"status": "complete", "totalLength": "42", "completedLength": "42", "downloadSpeed": "0"}})
	}))
	defer server.Close()
	rpc := &rpcClient{endpoint: server.URL, client: &http.Client{Timeout: 20 * time.Millisecond}}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	sawNotice := false
	var last []Progress
	err := New(1, nil).monitor(ctx, rpc, []Item{{URL: "unused"}}, []string{"gid"}, []Progress{{Name: "movie", Status: "active", Completed: 21, Total: 42}}, make(chan struct{}), func(rows []Progress) {
		last = rows
		if rows[0].Notice != "" {
			sawNotice = true
			if rows[0].Completed != 21 {
				t.Error("timeout discarded previous progress")
			}
		}
	})
	if err != nil || !sawNotice || last[0].Status != "completed" || last[0].Notice != "" {
		t.Fatalf("progress did not recover: %+v, notice=%v, err=%v", last, sawNotice, err)
	}
}

func TestCancelledDownloadResumesSavedPieces(t *testing.T) {
	requireAria2(t)
	content := bytes.Repeat([]byte("0123456789abcdef"), 262144)
	var resumed atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "" {
			resumed.Store(true)
		}
		http.ServeContent(&slowWriter{ResponseWriter: w}, r, "movie.bin", time.Unix(0, 0), bytes.NewReader(content))
	}))
	defer server.Close()
	dir := t.TempDir()
	item := []Item{{URL: server.URL + "/movie.bin"}}
	manager := New(1, []string{"-x", "1", "-s", "1", "-k", "1M"})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	sawPartial := false
	err := manager.Download(ctx, item, dir, func(rows []Progress) {
		if rows[0].Completed >= 1048576 && rows[0].Completed < int64(len(content)) {
			sawPartial = true
			cancel()
		}
	})
	if err == nil || !sawPartial {
		t.Fatalf("did not interrupt a partial download: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "movie.bin.aria2")); err != nil {
		t.Fatalf("resume file not saved: %v", err)
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel2()
	if err := manager.Download(ctx2, item, dir, func([]Progress) {}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "movie.bin"))
	if err != nil || !bytes.Equal(got, content) || !resumed.Load() {
		t.Fatalf("resume corrupted or restarted file: range=%v, err=%v", resumed.Load(), err)
	}
	if _, err := os.Stat(filepath.Join(dir, "movie.bin.aria2")); !os.IsNotExist(err) {
		t.Fatalf("completed download still has resume file: %v", err)
	}
}

type slowWriter struct{ http.ResponseWriter }

func (w *slowWriter) Write(data []byte) (int, error) {
	n, err := w.ResponseWriter.Write(data)
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
	time.Sleep(20 * time.Millisecond)
	return n, err
}
