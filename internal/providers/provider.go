package providers

type DistroProvider interface {
	DownloadURL(version string) (string, error)
	ChecksumURL(version string) (string, error)
	ISOFilename(version string) string
	SquashfsPath(mountDir string) (string, error)
	InstallPackages(chrootDir string, packages []string) error
}

var Registry = map[string]DistroProvider{
	"ubuntu": &UbuntuProvider{},
}