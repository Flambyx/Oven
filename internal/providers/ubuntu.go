package providers

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"os"
	"path/filepath"
)

type UbuntuProvider struct {
	cachedFilename map[string]string
}

func (u *UbuntuProvider) resolveFilename(version string) (string, error) {
	if u.cachedFilename == nil {
		u.cachedFilename = make(map[string]string)
	}
	if name, ok := u.cachedFilename[version]; ok {
		return name, nil
	}

	url := fmt.Sprintf("https://releases.ubuntu.com/%s/", version)
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("could not fetch Ubuntu releases page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Ubuntu releases page returned HTTP %d — is version %s valid?", resp.StatusCode, version)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("could not read releases page: %w", err)
	}

	re := regexp.MustCompile(`ubuntu-` + regexp.QuoteMeta(version) + `[\w.\-]+-live-server-amd64\.iso`)
	match := re.Find(body)
	if match == nil {
		return "", fmt.Errorf("could not find a server ISO for Ubuntu %s", version)
	}

	filename := strings.TrimSpace(string(match))
	u.cachedFilename[version] = filename
	return filename, nil
}

func (u *UbuntuProvider) DownloadURL(version string) (string, error) {
	filename, err := u.resolveFilename(version)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://releases.ubuntu.com/%s/%s", version, filename), nil
}

func (u *UbuntuProvider) ChecksumURL(version string) (string, error) {
	if version == "" {
		return "", fmt.Errorf("version is required")
	}
	return fmt.Sprintf("https://releases.ubuntu.com/%s/SHA256SUMS", version), nil
}

func (u *UbuntuProvider) ISOFilename(version string) string {
	filename, err := u.resolveFilename(version)
	if err != nil {
		return fmt.Sprintf("ubuntu-%s-live-server-amd64.iso", version)
	}
	return filename
}

func (u *UbuntuProvider) SquashfsPath(mountDir string) (string, error) {
	candidates := []string{
		filepath.Join(mountDir, "casper", "filesystem.squashfs"),
		filepath.Join(mountDir, "casper", "ubuntu-server-minimal.squashfs"),
		filepath.Join(mountDir, "install", "filesystem.squashfs"),
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	var found string
	filepath.Walk(mountDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || found != "" {
			return nil
		}
		if filepath.Ext(path) == ".squashfs" {
			found = path
		}
		return nil
	})

	if found != "" {
		return found, nil
	}

	return "", fmt.Errorf("no squashfs filesystem found in Ubuntu ISO")
}