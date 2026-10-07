package fbhttp

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/filebrowser/filebrowser/v2/files"
)

func TestSetContentDisposition(t *testing.T) {
	t.Parallel()

	for _, filename := range []string{"document.pdf", "日本語.txt", "my file.txt"} {
		t.Run(filename, func(t *testing.T) {
			t.Parallel()

			recorder := httptest.NewRecorder()
			req, err := http.NewRequest(http.MethodGet, "/test", http.NoBody)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			setContentDisposition(recorder, req, &files.FileInfo{Name: filename})

			// Files are always shown in the browser, never offered as a download.
			want := "inline; filename*=utf-8''" + url.PathEscape(filename)
			if got := recorder.Header().Get("Content-Disposition"); got != want {
				t.Errorf("Content-Disposition = %q, want %q", got, want)
			}
			if got := recorder.Header().Get("Content-Type"); got != "" {
				t.Errorf("Content-Type = %q, want empty", got)
			}
		})
	}
}
