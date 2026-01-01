package helper

import (
	"path/filepath"
	"strings"
)

func ExtractPublicID(url string) string {
	// URL dạng: https://res.cloudinary.com/<cloud-name>/image/upload/<folder>/<public-id>.<format>
	parts := strings.Split(url, "/")
	n := len(parts)
	if n < 4 {
		return ""
	}
	// Lấy phần <folder>/<public-id>.<format> và bỏ .format
	publicID := strings.Join(parts[n-2:n], "/")
	return strings.TrimSuffix(publicID, filepath.Ext(publicID))
}
