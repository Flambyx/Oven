package builder

import (
	"fmt"
	"io"
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

	if len(cfg.Files) > 0 {
		fmt.Println("Copying files...")
		if err := b.copyFiles(chrootDir, cfg.Files); err != nil {
			return fmt.Errorf("file copy failed: %w", err)
		}
		fmt.Println("Files copied")
	}

	return nil
}

func (b *Builder) copyFiles(chrootDir string, files []config.File) error {
	for _, f := range files {
		if err := b.copyFile(chrootDir, f); err != nil {
			return err
		}
	}
	return nil
}

func (b *Builder) copyFile(chrootDir string, f config.File) error {
	srcInfo, err := os.Stat(f.Src)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("source not found: %s", f.Src)
		}
		return fmt.Errorf("could not stat source %s: %w", f.Src, err)
	}

	if srcInfo.IsDir() {
		return b.copyDir(chrootDir, f.Src, f.Dest)
	}

	return b.copySingleFile(chrootDir, f.Src, f.Dest)
}

func (b *Builder) copyDir(chrootDir, srcDir, destDir string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error walking %s: %w", path, err)
		}

		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return fmt.Errorf("could not compute relative path for %s: %w", path, err)
		}

		dest := filepath.Join(destDir, rel)

		if info.IsDir() {
			if err := os.MkdirAll(filepath.Join(chrootDir, dest), info.Mode()); err != nil {
				return fmt.Errorf("could not create directory %s: %w", dest, err)
			}
			return nil
		}

		return b.copySingleFile(chrootDir, path, dest)
	})
}

func (b *Builder) copySingleFile(chrootDir, src, dest string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("could not stat %s: %w", src, err)
	}

	fullDest := filepath.Join(chrootDir, dest)

	if err := os.MkdirAll(filepath.Dir(fullDest), 0755); err != nil {
		return fmt.Errorf("could not create destination directory for %s: %w", dest, err)
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("could not open source file %s: %w", src, err)
	}
	defer in.Close()

	out, err := os.OpenFile(fullDest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("could not create destination file %s: %w", dest, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("could not copy %s to %s: %w", src, dest, err)
	}

	fmt.Printf("  %s -> %s\n", src, dest)
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