package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Flambyx/oven/internal/config"
	"github.com/Flambyx/oven/internal/downloader"
	"github.com/Flambyx/oven/internal/extractor"
	"github.com/Flambyx/oven/internal/providers"
	"github.com/spf13/cobra"
)

var recipeFile string

var rootCmd = &cobra.Command{
	Use:   "oven",
	Short: "Bake bootable Linux ISOs from a simple recipe",
}

var cookCmd = &cobra.Command{
	Use:   "cook",
	Short: "Cook an ISO from a recipe.yaml",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load(recipeFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Cooking %s (%s %s)...\n", cfg.Name, cfg.Base.Distro, cfg.Base.Version)

		provider, ok := providers.Registry[cfg.Base.Distro]
		if !ok {
			fmt.Fprintf(os.Stderr, "Error: unsupported distro: %s\n", cfg.Base.Distro)
			os.Exit(1)
		}

		isoPath, err := downloader.Fetch(cfg.Base.Distro, cfg.Base.Version)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("ISO ready: %s\n", isoPath)

		workDir := filepath.Join(os.TempDir(), "oven-"+cfg.Name)
		ext, err := extractor.New(workDir, provider)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if err := ext.Extract(isoPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	cookCmd.Flags().StringVarP(&recipeFile, "file", "f", "recipe.yaml", "recipe file")
	rootCmd.AddCommand(cookCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}