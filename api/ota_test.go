package api_test

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_HandleUpdate(t *testing.T) {
	t.Parallel()

	binaryContent := readFile(t, "testdata/binary.txt")
	sum := sha256.Sum256(binaryContent)
	binarySHA256 := hex.EncodeToString(sum[:])

	zipContent := readFile(t, "testdata/binary.txt.zip")
	sum = sha256.Sum256(zipContent)
	zipSHA256 := hex.EncodeToString(sum[:])

	// Serve a fake binary for download.
	downloadSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(binaryContent)
	}))
	t.Cleanup(downloadSrv.Close)

	downloadZipSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(zipContent)
	}))
	t.Cleanup(downloadZipSrv.Close)

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantBin    bool
	}{
		{
			name:       "downloads and replaces binary",
			body:       `{"url":"` + downloadSrv.URL + `","sha256":"` + binarySHA256 + `"}`,
			wantStatus: http.StatusNoContent,
			wantBin:    true,
		},
		{
			name:       "downloads and replaces zipped binary",
			body:       `{"url":"` + downloadZipSrv.URL + `","sha256":"` + zipSHA256 + `"}`,
			wantStatus: http.StatusNoContent,
			wantBin:    true,
		},
		{
			name:       "invalid json body",
			body:       "not json",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing url",
			body:       `{"sha256":"abc"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing sha256",
			body:       `{"url":"http://example.com/bin"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "sha256 mismatch",
			body:       `{"url":"` + downloadSrv.URL + `","sha256":"000000"}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			glassBin := filepath.Join(t.TempDir(), "glass")

			sup := new(mockSupervisor)
			if test.wantBin {
				sup.On("Restart").Return()
			}

			srv := newServer(sup, nil, glassBin, t.TempDir())

			r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/glass/update", strings.NewReader(test.body))
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)

			if test.wantBin {
				got, err := os.ReadFile(glassBin)
				require.NoError(t, err)
				assert.Equal(t, binaryContent, got)
			}

			sup.AssertExpectations(t)
		})
	}
}

func readFile(t *testing.T, filename string) []byte {
	t.Helper()

	b, err := os.ReadFile(filename)
	require.NoError(t, err)

	return b
}
