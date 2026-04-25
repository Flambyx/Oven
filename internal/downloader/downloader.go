package downloader

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"github.com/schollz/progressbar/v3"
	"github.com/Flambyx/oven/internal/providers"
)

func CacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not find home directory: %w", err)
	}
	dir := filepath.Join(home, ".cache", "oven")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("could not create cache directory: %w", err)
	}
	return dir, nil
}

func Fetch(distro, version string) (string, error) {
	provider, ok := providers.Registry[distro]
	if !ok {
		return "", fmt.Errorf("unsupported distro: %s", distro)
	}

	cacheDir, err := CacheDir()
	if err != nil {
		return "", err
	}

	isoPath := filepath.Join(cacheDir, provider.ISOFilename(version))

	// Vérifier le cache
	if _, err := os.Stat(isoPath); err == nil {
		fmt.Println("ISO found in cache, verifying checksum...")
		valid, err := verifyChecksum(isoPath, distro, version, provider)
		if err != nil {
			return "", err
		}
		if valid {
			fmt.Println("Checksum OK, using cached ISO")
			return isoPath, nil
		}
		fmt.Println("Checksum mismatch, re-downloading...")
		os.Remove(isoPath)
	}

	// Télécharger
	url, err := provider.DownloadURL(version)
	if err != nil {
		return "", err
	}
	fmt.Printf("Downloading %s %s...\n", distro, version)
	if err := download(url, isoPath); err != nil {
		return "", err
	}

	// Vérifier après téléchargement
	fmt.Println("🔍 Verifying checksum...")
	valid, err := verifyChecksum(isoPath, distro, version, provider)
	if err != nil {
		return "", err
	}
	if !valid {
		os.Remove(isoPath)
		return "", fmt.Errorf("checksum verification failed for downloaded ISO")
	}

	fmt.Println("Download complete and verified")
	return isoPath, nil
}

func download(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("could not create file: %w", err)
	}
	defer f.Close()

	bar := progressbar.DefaultBytes(
		resp.ContentLength,
		"Downloading",
	)

	if _, err := io.Copy(io.MultiWriter(f, bar), resp.Body); err != nil {
		return fmt.Errorf("could not write file: %w", err)
	}
	return nil
}

func verifyChecksum(isoPath, distro, version string, provider providers.DistroProvider) (bool, error) {
	checksumURL, err := provider.ChecksumURL(version)
	if err != nil {
		return false, err
	}

	resp, err := http.Get(checksumURL)
	if err != nil {
		return false, fmt.Errorf("could not fetch checksums: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("could not read checksums: %w", err)
	}

	filename := provider.ISOFilename(version)
	expectedHash := parseChecksum(string(body), filename)
	if expectedHash == "" {
		return false, fmt.Errorf("checksum not found for %s", filename)
	}

	actualHash, err := hashFile(isoPath)
	if err != nil {
		return false, err
	}

	return actualHash == expectedHash, nil
}

func parseChecksum(content, filename string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasSuffix(line, filename) || strings.Contains(line, filename) {
			parts := strings.Fields(line)
			if len(parts) >= 1 {
				return parts[0]
			}
		}
	}
	return ""
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("could not open file for hashing: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("could not hash file: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}