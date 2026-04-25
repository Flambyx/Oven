package builder

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Flambyx/oven/internal/providers"
)

type Builder struct {
	WorkDir  string
	Provider providers.DistroProvider
}

func New(workDir string, provider providers.DistroProvider) *Builder {
	return &Builder{WorkDir: workDir, Provider: provider}
}

func (b *Builder) Build(packages []string) error {
	chrootDir := filepath.Join(b.WorkDir, "squashfs-root")

	if err := b.mountPseudoFS(chrootDir); err != nil {
		return err
	}
	defer b.umountPseudoFS(chrootDir)

	if err := b.copyResolvConf(chrootDir); err != nil {
		return err
	}

	if len(packages) > 0 {
		fmt.Println("Installing packages...")
		if err := b.Provider.InstallPackages(chrootDir, packages); err != nil {
			return fmt.Errorf("package installation failed: %w", err)
		}
		fmt.Println("Packages installed")
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
	src := "/etc/resolv.conf"
	dest := filepath.Join(chrootDir, "etc", "resolv.conf")

	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("could not read resolv.conf: %w", err)
	}

	if err := os.WriteFile(dest, data, 0644); err != nil {
		return fmt.Errorf("could not write resolv.conf: %w", err)
	}

	return nil
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}