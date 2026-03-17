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

// ContentTypeByExt returns a MIME content type for the given file extension.
// Returns "application/octet-stream" for unknown extensions.
func ContentTypeByExt(ext string) string {
	if ct, ok := extToContentType[strings.ToLower(ext)]; ok {
		return ct
	}
	return "application/octet-stream"
}
