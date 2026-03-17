package media

import (
	"os"
	"path/filepath"
	"strings"
)

const TempDirName = "picoclaw_media"

// TempDir returns the shared temporary directory used for downloaded media.
func TempDir() string {
	return filepath.Join(os.TempDir(), TempDirName)
}

// EnsureDir creates the shared media temp directory if it does not exist.
func EnsureDir() (string, error) {
	dir := TempDir()
	return dir, os.MkdirAll(dir, 0755)
}

var extToContentType = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
	".ogg":  "audio/ogg",
	".mp3":  "audio/mpeg",
	".wav":  "audio/wav",
	".mp4":  "video/mp4",
	".pdf":  "application/pdf",
}

// IsAllowedPath checks whether the given file path is under the media temp dir
// or workspace. Does NOT allow all of system temp to prevent the exfiltration
// chain: write_file to /tmp -> send as media.
func IsAllowedPath(path, workspace string) bool {
	cleaned := filepath.Clean(path)
	if !filepath.IsAbs(cleaned) {
		abs, err := filepath.Abs(cleaned)
		if err != nil {
			return false
		}
		cleaned = abs
	}

	isUnder := func(dir string) bool {
		d := filepath.Clean(dir)
		return strings.HasPrefix(cleaned, d+string(filepath.Separator)) || cleaned == d
	}

	if isUnder(TempDir()) {
		return true
	}
	if workspace != "" {
		return isUnder(workspace)
	}
	return false
}

// ContentTypeByExt returns a MIME content type for the given file extension.
// Returns "application/octet-stream" for unknown extensions.
func ContentTypeByExt(ext string) string {
	if ct, ok := extToContentType[strings.ToLower(ext)]; ok {
		return ct
	}
	return "application/octet-stream"
}
