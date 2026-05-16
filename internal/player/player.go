package player

import (
	"os/exec"
)

func GetPlayerCmd(url string) (*exec.Cmd, string, error) {
	// Try mpv first
	path, err := exec.LookPath("mpv")
	if err == nil {
		return exec.Command(path, url), "mpv", nil
	}

	// Try vlc fallback
	path, err = exec.LookPath("vlc")
	if err == nil {
		// vlc often needs --play-and-exit to be more CLI friendly
		return exec.Command(path, url, "--play-and-exit"), "vlc", nil
	}

	return nil, "", exec.ErrNotFound
}
