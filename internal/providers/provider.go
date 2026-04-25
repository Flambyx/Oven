package providers

type DistroProvider interface {
	DownloadURL(version string) (string, error)
	ChecksumURL(version string) (string, error)
	ISOFilename(version string) string
}

var Registry = map[string]DistroProvider{
	"ubuntu": &UbuntuProvider{},
}