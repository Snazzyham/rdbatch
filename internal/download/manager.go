package download

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
)

type Manager struct {
	semaphore chan struct{}
	unlimited bool
	flags     []string
}

func New(concurrent int, flags []string) *Manager {
	m := &Manager{flags: flags}
	if concurrent <= 0 {
		m.unlimited = true
	} else {
		m.semaphore = make(chan struct{}, concurrent)
	}
	return m
}

func (m *Manager) Download(urls []string, dir string) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(urls))

	for _, url := range urls {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			if !m.unlimited {
				m.semaphore <- struct{}{}
				defer func() { <-m.semaphore }()
			}

			if err := m.downloadOne(u, dir); err != nil {
				errChan <- fmt.Errorf("failed to download %s: %w", u, err)
			}
		}(url)
	}

	wg.Wait()
	close(errChan)

	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("%d download(s) failed", len(errs))
	}
	return nil
}

func (m *Manager) downloadOne(url, dir string) error {
	args := append([]string{}, m.flags...)
	args = append(args, "--dir", dir, url)
	cmd := exec.Command("aria2c", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func CheckAria2() error {
	_, err := exec.LookPath("aria2c")
	if err != nil {
		return fmt.Errorf("aria2c not found in PATH: please install aria2")
	}
	return nil
}
