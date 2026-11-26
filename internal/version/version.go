package version

import (
	"os"
	"strings"
)

var Version = "dev"

func Current() string {
	if v := strings.TrimSpace(os.Getenv("ICONFIGLY_VERSION")); v != "" {
		return strings.TrimPrefix(v, "v")
	}
	if v := strings.TrimSpace(Version); v != "" {
		return strings.TrimPrefix(v, "v")
	}
	return "dev"
}
