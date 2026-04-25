package config

import (
	"os"
	"fmt"
	"gopkg.in/yaml.v3"
)

type Config struct {
	APIVersion string   `yaml:"apiVersion"`
	Name       string   `yaml:"name"`
	Base       Base     `yaml:"base"`
	Packages   []string `yaml:"packages"`
	Files      []File   `yaml:"files"`
	Services   Services `yaml:"services"`
	Users      []User   `yaml:"users"`
	Locale     Locale   `yaml:"locale"`
}

type Base struct {
	Distro  string `yaml:"distro"`
	Version string `yaml:"version"`
}

type File struct {
	Src  string `yaml:"src"`
	Dest string `yaml:"dest"`
}

type Services struct {
	Enable  []string `yaml:"enable"`
	Disable []string `yaml:"disable"`
}

type User struct {
	Name    string   `yaml:"name"`
	Sudo    bool     `yaml:"sudo"`
	SSHKeys []string `yaml:"ssh_keys"`
}

type Locale struct {
	Timezone string `yaml:"timezone"`
	Lang     string `yaml:"lang"`
}

func Load(path string) (*Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("recipe file not found: %s", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read recipe file: %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("recipe file is empty: %s", path)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid recipe format: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid recipe: %w", err)
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	if c.Base.Distro == "" {
		return fmt.Errorf("base.distro is required")
	}
	if c.Base.Version == "" {
		return fmt.Errorf("base.version is required")
	}
	if c.Name == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}