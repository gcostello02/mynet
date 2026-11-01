package server

import (
	"net/url"
	"path/filepath"
)

func sqliteDSN(path string) string {
	u := url.URL{
		Scheme:   "file",
		Path:     filepath.ToSlash(path),
		RawQuery: "_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)",
	}
	return u.String()
}
