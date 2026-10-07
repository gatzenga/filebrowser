package fbhttp

import "testing"

func TestThumbnailKind(t *testing.T) {
	t.Parallel()

	for name, want := range map[string]string{
		"film.mkv":        "video",
		"film.MP4":        "video",
		"clip.mov":        "video",
		"photo.jpg":       "image",
		"photo.PNG":       "image",
		"notes.txt":       "",
		"archive.zip":     "",
		"no-extension":    "",
		".hidden":         "",
		"dotted.name.m4v": "video",
	} {
		if got := thumbnailKind(name); got != want {
			t.Errorf("thumbnailKind(%q) = %q, want %q", name, got, want)
		}
	}
}
