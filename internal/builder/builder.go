package builder

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Flambyx/oven/internal/config"
	"github.com/Flambyx/oven/internal/providers"
)

type Builder struct {
	WorkDir  string
	Provider providers.DistroProvider
}

func New(workDir string, provider providers.DistroProvider) *Builder {
	return &Builder{WorkDir: workDir, Provider: provider}
}

func (b *Builder) Build(cfg *config.Config) error {
	chrootDir := filepath.Join(b.WorkDir, "squashfs-root")

	if err := b.mountPseudoFS(chrootDir); err != nil {
		return err
	}
	defer b.umountPseudoFS(chrootDir)

	if err := b.copyResolvConf(chrootDir); err != nil {
		return err
	}

	fmt.Println("Updating package index...")
	if err := b.Provider.UpdatePackageIndex(chrootDir); err != nil {
		return fmt.Errorf("package index update failed: %w", err)
	}

	fmt.Println("Applying system config...")
	if err := b.applySystemConfig(chrootDir, cfg); err != nil {
		return fmt.Errorf("system config failed: %w", err)
	}
	fmt.Println("System config applied")

	if len(cfg.Packages) > 0 {
		fmt.Println("Installing packages...")
		if err := b.Provider.InstallPackages(chrootDir, cfg.Packages); err != nil {
			return fmt.Errorf("package installation failed: %w", err)
		}
		fmt.Println("Packages installed")
	}

	return nil
}

func (b *Builder) applySystemConfig(chrootDir string, cfg *config.Config) error {
	if err := b.Provider.ConfigureLocale(chrootDir, cfg.Locale); err != nil {
		return err
	}

	if err := b.configureTimezone(chrootDir, cfg.Locale.Timezone); err != nil {
		return err
	}

	if err := b.configureUsers(chrootDir, cfg.Users); err != nil {
		return err
	}

	return nil
}

func (b *Builder) configureTimezone(chrootDir, timezone string) error {
	if timezone == "" {
		return nil
	}

	zonefile := filepath.Join("/usr/share/zoneinfo", timezone)
	target := filepath.Join(chrootDir, "etc", "localtime")

	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("could not remove existing localtime: %w", err)
	}

	if err := os.Symlink(zonefile, target); err != nil {
		return fmt.Errorf("could not set timezone: %w", err)
	}

	if err := os.WriteFile(filepath.Join(chrootDir, "etc", "timezone"), []byte(timezone+"\n"), 0644); err != nil {
		return fmt.Errorf("could not write /etc/timezone: %w", err)
	}

	return nil
}

func (b *Builder) configureUsers(chrootDir string, users []config.User) error {
	for _, user := range users {
		if err := b.createUser(chrootDir, user); err != nil {
			return err
		}
	}
	return nil
}

func (b *Builder) createUser(chrootDir string, user config.User) error {
	if err := chroot(chrootDir, "useradd", "--shell", "/bin/bash", "--create-home", user.Name); err != nil {
		return fmt.Errorf("could not create user %s: %w", user.Name, err)
	}

	if user.Sudo {
		if err := chroot(chrootDir, "usermod", "-aG", "sudo", user.Name); err != nil {
			return fmt.Errorf("could not add user %s to sudo: %w", user.Name, err)
		}
	}

	if len(user.SSHKeys) > 0 {
		sshDir := filepath.Join(chrootDir, "home", user.Name, ".ssh")
		if err := os.MkdirAll(sshDir, 0700); err != nil {
			return fmt.Errorf("could not create .ssh directory for %s: %w", user.Name, err)
		}

		authorizedKeys := strings.Join(user.SSHKeys, "\n") + "\n"
		if err := os.WriteFile(filepath.Join(sshDir, "authorized_keys"), []byte(authorizedKeys), 0600); err != nil {
			return fmt.Errorf("could not write authorized_keys for %s: %w", user.Name, err)
		}
	}

	return nil
}

func (b *Builder) mountPseudoFS(chrootDir string) error {
	mounts := []struct {
		fstype string
		source string
		target string
	}{
		{"proc", "proc", filepath.Join(chrootDir, "proc")},
		{"sysfs", "sys", filepath.Join(chrootDir, "sys")},
		{"devtmpfs", "dev", filepath.Join(chrootDir, "dev")},
		{"devpts", "devpts", filepath.Join(chrootDir, "dev", "pts")},
	}

	for _, m := range mounts {
		if err := os.MkdirAll(m.target, 0755); err != nil {
			return fmt.Errorf("could not create mount point %s: %w", m.target, err)
		}
		if err := run("mount", "-t", m.fstype, m.source, m.target); err != nil {
			return fmt.Errorf("could not mount %s: %w", m.target, err)
		}
	}

	return nil
}

func (b *Builder) umountPseudoFS(chrootDir string) {
	targets := []string{
		filepath.Join(chrootDir, "dev", "pts"),
		filepath.Join(chrootDir, "dev"),
		filepath.Join(chrootDir, "sys"),
		filepath.Join(chrootDir, "proc"),
	}

	for _, target := range targets {
		run("umount", target)
	}
}

func (b *Builder) copyResolvConf(chrootDir string) error {
	data, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return fmt.Errorf("could not read resolv.conf: %w", err)
	}

	if err := os.WriteFile(filepath.Join(chrootDir, "etc", "resolv.conf"), data, 0644); err != nil {
		return fmt.Errorf("could not write resolv.conf: %w", err)
	}

	return nil
}

func chroot(chrootDir string, args ...string) error {
	cmd := exec.Command("chroot", append([]string{chrootDir}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}