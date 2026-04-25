package providers

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Flambyx/oven/internal/config"
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

func cleanEnv() []string {
	return []string{
		"DEBIAN_FRONTEND=noninteractive",
		"HOME=/root",
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"LANG=C",
		"LC_ALL=C",
	}
}

func (u *UbuntuProvider) UpdatePackageIndex(chrootDir string) error {
	cmd := exec.Command("chroot", chrootDir, "apt-get", "update")
	cmd.Env = cleanEnv()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("apt-get update failed: %w", err)
	}
	return nil
}

func (u *UbuntuProvider) InstallPackages(chrootDir string, packages []string) error {
	args := append([]string{chrootDir, "apt-get", "install", "-y", "--fix-missing"}, packages...)
	cmd := exec.Command("chroot", args...)
	cmd.Env = cleanEnv()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("apt-get install failed: %w", err)
	}
	return nil
}

func (u *UbuntuProvider) ConfigureLocale(chrootDir string, locale config.Locale) error {
	if locale.Lang == "" {
		return nil
	}

	installCmd := exec.Command("chroot", chrootDir, "apt-get", "install", "-y", "locales")
	installCmd.Env = cleanEnv()
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("could not install locales package: %w", err)
	}

	localeGen := filepath.Join(chrootDir, "etc", "locale.gen")
	entry := locale.Lang + " UTF-8\n"
	if err := os.WriteFile(localeGen, []byte(entry), 0644); err != nil {
		return fmt.Errorf("could not write locale.gen: %w", err)
	}

	cmd := exec.Command("chroot", chrootDir, "locale-gen")
	cmd.Env = cleanEnv()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("locale-gen failed: %w", err)
	}

	localeConf := fmt.Sprintf("LANG=%s\nLC_ALL=%s\n", locale.Lang, locale.Lang)
	if err := os.WriteFile(filepath.Join(chrootDir, "etc", "locale.conf"), []byte(localeConf), 0644); err != nil {
		return fmt.Errorf("could not write locale.conf: %w", err)
	}

	defaultLocale := fmt.Sprintf("LANG=%s\n", locale.Lang)
	if err := os.WriteFile(filepath.Join(chrootDir, "etc", "default", "locale"), []byte(defaultLocale), 0644); err != nil {
		return fmt.Errorf("could not write /etc/default/locale: %w", err)
	}

	return nil
}