package extractor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"github.com/Flambyx/oven/internal/providers"
)

type Extractor struct {
	WorkDir  string
	Provider providers.DistroProvider
}

func New(workDir string, provider providers.DistroProvider) (*Extractor, error) {
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return nil, fmt.Errorf("could not create work directory: %w", err)
	}
	return &Extractor{WorkDir: workDir, Provider: provider}, nil
}

func (e *Extractor) Extract(isoPath string) error {
	mountDir := filepath.Join(e.WorkDir, "iso-mount")
	squashfsDir := filepath.Join(e.WorkDir, "squashfs-root")

	if err := os.MkdirAll(mountDir, 0755); err != nil {
		return fmt.Errorf("could not create mount directory: %w", err)
	}

	fmt.Println("Mounting ISO...")
	if err := run("mount", "-o", "loop,ro", isoPath, mountDir); err != nil {
		return fmt.Errorf("could not mount ISO: %w", err)
	}
	defer func() {
		fmt.Println("Unmounting ISO...")
		run("umount", mountDir)
	}()

	squashfsPath, err := e.Provider.SquashfsPath(mountDir)
	if err != nil {
		return err
	}
	fmt.Printf("Found filesystem at %s\n", squashfsPath)

	fmt.Println("Extracting filesystem...")
	if err := os.RemoveAll(squashfsDir); err != nil {
		return fmt.Errorf("could not clean squashfs directory: %w", err)
	}
	if err := run("unsquashfs", "-d", squashfsDir, squashfsPath); err != nil {
		return fmt.Errorf("could not extract squashfs: %w", err)
	}

	fmt.Printf("Filesystem extracted to %s\n", squashfsDir)
	return nil
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}