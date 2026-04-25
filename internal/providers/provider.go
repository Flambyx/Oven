package providers

import "github.com/Flambyx/oven/internal/config"

type DistroProvider interface {
    DownloadURL(version string) (string, error)
    ChecksumURL(version string) (string, error)
    ISOFilename(version string) string
    SquashfsPath(mountDir string) (string, error)
    UpdatePackageIndex(chrootDir string) error
    InstallPackages(chrootDir string, packages []string) error
    ConfigureLocale(chrootDir string, locale config.Locale) error
}

var Registry = map[string]DistroProvider{
	"ubuntu": &UbuntuProvider{},
}