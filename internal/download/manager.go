package download

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"
)

type Item struct{ URL, Name string }
type Progress struct {
	Name, Status, Error, Notice string
	Completed, Total, Speed     int64
}
type Manager struct {
	concurrent int
	flags      []string
	retryWait  time.Duration
}

func New(concurrent int, flags []string) *Manager {
	return &Manager{concurrent: concurrent, flags: flags, retryWait: 15 * time.Second}
}

type rpcClient struct {
	endpoint, secret string
	client           *http.Client
}

func (r *rpcClient) call(ctx context.Context, method string, args []any, result any) error {
	params := append([]any{"token:" + r.secret}, args...)
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": "rdbatch", "method": "aria2." + method, "params": params})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", r.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return err
	}
	if envelope.Error != nil {
		return fmt.Errorf("%s", envelope.Error.Message)
	}
	if result != nil {
		return json.Unmarshal(envelope.Result, result)
	}
	return nil
}

// Download runs one private aria2 process for the entire batch. Updates are snapshots.
func (m *Manager) Download(ctx context.Context, items []Item, dir string, update func([]Progress)) error {
	if len(items) == 0 {
		return nil
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return err
	}
	rpc := &rpcClient{endpoint: fmt.Sprintf("http://127.0.0.1:%d/jsonrpc", port), secret: hex.EncodeToString(secretBytes), client: &http.Client{Timeout: 10 * time.Second}}
	limit := m.concurrent
	if limit <= 0 {
		limit = len(items)
	}
	args := append([]string{}, m.flags...)
	args = append(args, "--dir", dir, "--enable-rpc=true", "--rpc-listen-all=false", "--rpc-listen-port="+strconv.Itoa(port), "--rpc-secret="+rpc.secret, "--max-concurrent-downloads="+strconv.Itoa(limit), "--stop-with-process="+strconv.Itoa(os.Getpid()), "--console-log-level=error", "--summary-interval=0", "--auto-save-interval=5", "--continue=true", "--auto-file-renaming=false", "--always-resume=true")
	cmd := exec.Command("aria2c", args...)
	var logs bytes.Buffer
	cmd.Stdout = &logs
	cmd.Stderr = &logs
	if err := cmd.Start(); err != nil {
		return err
	}
	exited := make(chan struct{})
	go func() { _ = cmd.Wait(); close(exited) }()
	defer func() {
		// SIGINT lets aria2 save its resume files before exiting.
		_ = cmd.Process.Signal(os.Interrupt)
		select {
		case <-exited:
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
			<-exited
		}
	}()
	ready := false
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		if err := rpc.call(ctx, "getVersion", nil, nil); err == nil {
			ready = true
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	if !ready {
		_ = cmd.Process.Kill()
		<-exited
		return fmt.Errorf("aria2 RPC did not start: %s", logs.String())
	}
	states := make([]Progress, len(items))
	gids := make([]string, len(items))
	for i, item := range items {
		name := item.Name
		if name == "" {
			if u, e := url.Parse(item.URL); e == nil {
				name = path.Base(u.Path)
			}
		}
		if name == "" {
			name = "Download " + strconv.Itoa(i+1)
		}
		states[i] = Progress{Name: name, Status: "queued"}
		if err := rpc.call(ctx, "addUri", []any{[]string{item.URL}}, &gids[i]); err != nil {
			states[i].Status = "failed"
			states[i].Error = err.Error()
		}
	}
	return m.monitor(ctx, rpc, items, gids, states, exited, update)
}

func (m *Manager) monitor(ctx context.Context, rpc *rpcClient, items []Item, gids []string, states []Progress, exited <-chan struct{}, update func([]Progress)) error {
	retries := make([]int, len(items))
	nextAttempt := make([]time.Time, len(items))
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-exited:
			return fmt.Errorf("aria2 exited before the batch finished; reselect unfinished files to resume")
		default:
		}
		pending, failed := 0, 0
		for i, gid := range gids {
			if states[i].Status == "completed" || states[i].Status == "failed" {
				if states[i].Status == "failed" {
					failed++
				}
				continue
			}
			if !nextAttempt[i].IsZero() {
				pending++
				if time.Now().Before(nextAttempt[i]) {
					states[i].Notice = fmt.Sprintf("Rate limited. Retrying in %s, attempt %d/5", time.Until(nextAttempt[i]).Round(time.Second), retries[i])
					continue
				}
				var newGID string
				if err := rpc.call(ctx, "addUri", []any{[]string{items[i].URL}}, &newGID); err != nil {
					// addUri is not safe to repeat after a timeout: it may already have queued the file.
					states[i].Status = "failed"
					states[i].Error = fmt.Sprintf("Could not queue retry: %v. Reselect this file to resume.", err)
					continue
				}
				_ = rpc.call(ctx, "removeDownloadResult", []any{gid}, nil)
				gids[i] = newGID
				nextAttempt[i] = time.Time{}
				states[i].Status = "queued"
				states[i].Error, states[i].Notice = "", ""
				continue
			}
			var status struct {
				Status    string `json:"status"`
				Total     string `json:"totalLength"`
				Completed string `json:"completedLength"`
				Speed     string `json:"downloadSpeed"`
				Error     string `json:"errorMessage"`
				Files     []struct {
					Path string `json:"path"`
				} `json:"files"`
			}
			if err := rpc.call(ctx, "tellStatus", []any{gid, []string{"status", "totalLength", "completedLength", "downloadSpeed", "errorMessage", "files"}}, &status); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				var networkError net.Error
				if errors.As(err, &networkError) {
					states[i].Notice = "Progress temporarily unavailable; aria2 is still running"
					pending++
					continue
				}
				return fmt.Errorf("reading download progress: %w", err)
			}
			states[i].Notice = ""
			states[i].Total, _ = strconv.ParseInt(status.Total, 10, 64)
			states[i].Completed, _ = strconv.ParseInt(status.Completed, 10, 64)
			states[i].Speed, _ = strconv.ParseInt(status.Speed, 10, 64)
			if len(status.Files) > 0 && status.Files[0].Path != "" {
				states[i].Name = path.Base(status.Files[0].Path)
			}
			switch status.Status {
			case "complete":
				states[i].Status = "completed"
			case "error", "removed":
				if status.Status == "error" && strings.Contains(status.Error, "status=429") && retries[i] < 5 {
					nextAttempt[i] = time.Now().Add(m.retryWait * time.Duration(1<<retries[i]))
					retries[i]++
					states[i].Status = "retrying"
					states[i].Speed = 0
					states[i].Notice = "Rate limited. Waiting before retrying."
				} else {
					states[i].Status = "failed"
					states[i].Error = status.Error
					if states[i].Error == "" {
						states[i].Error = "Download removed"
					}
				}
			default:
				states[i].Status = status.Status
			}
			if states[i].Status == "failed" {
				failed++
			} else if states[i].Status != "completed" {
				pending++
			}
		}
		update(append([]Progress(nil), states...))
		if pending == 0 {
			if failed > 0 {
				return fmt.Errorf("%d download(s) failed", failed)
			}
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-exited:
			return fmt.Errorf("aria2 exited before the batch finished; reselect unfinished files to resume")
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func CheckAria2() error {
	if _, err := exec.LookPath("aria2c"); err != nil {
		return fmt.Errorf("aria2c not found in PATH: please install aria2")
	}
	return nil
}
