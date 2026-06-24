package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_HandleGetConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		setup      func(dataDir string)
		wantStatus int
		wantBody   string
	}{
		{
			name: "returns config when present",
			setup: func(dataDir string) {
				dir := filepath.Join(dataDir, "config")
				require.NoError(t, os.MkdirAll(dir, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("key: value\n"), 0o644))
			},
			wantStatus: http.StatusOK,
			wantBody:   "key: value\n",
		},
		{
			name:       "returns 404 when config is absent",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			dataDir := t.TempDir()
			if test.setup != nil {
				test.setup(dataDir)
			}

			sup := new(mockSupervisor)
			srv := newServer(sup, nil, "", dataDir)

			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/glass/config", nil)
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)
			if test.wantBody != "" {
				assert.Equal(t, test.wantBody, w.Body.String())
			}

			sup.AssertExpectations(t)
		})
	}
}

func TestServer_HandleConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "writes config",
			body:       "key: value\n",
			wantStatus: http.StatusNoContent,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			sup := new(mockSupervisor)

			dataDir := t.TempDir()
			srv := newServer(sup, nil, "", dataDir)

			r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/glass/config", strings.NewReader(test.body))
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)

			if test.wantStatus == http.StatusNoContent {
				got, err := os.ReadFile(filepath.Join(dataDir, "config", "config.yaml"))
				require.NoError(t, err)
				assert.Equal(t, test.body, string(got))
			}

			sup.AssertExpectations(t)
		})
	}
}

func TestServer_HandleSecrets(t *testing.T) {
	t.Parallel()

	sup := new(mockSupervisor)

	dataDir := t.TempDir()
	srv := newServer(sup, nil, "", dataDir)

	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/glass/secrets", strings.NewReader("secret: value\n"))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)

	got, err := os.ReadFile(filepath.Join(dataDir, "config", "secrets.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "secret: value\n", string(got))

	sup.AssertExpectations(t)
}

func TestServer_HandleListAssets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setup     func(dataDir string)
		wantNames []string
	}{
		{
			name: "returns asset names",
			setup: func(dataDir string) {
				dir := filepath.Join(dataDir, "assets")
				require.NoError(t, os.MkdirAll(dir, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(dir, "image.png"), []byte("data"), 0o644))
				require.NoError(t, os.WriteFile(filepath.Join(dir, "logo.svg"), []byte("data"), 0o644))
			},
			wantNames: []string{"image.png", "logo.svg"},
		},
		{
			name:      "returns empty array when directory is absent",
			wantNames: []string{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			dataDir := t.TempDir()
			if test.setup != nil {
				test.setup(dataDir)
			}

			sup := new(mockSupervisor)
			srv := newServer(sup, nil, "", dataDir)

			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/glass/assets", nil)
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, r)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var got []string
			require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
			assert.Equal(t, test.wantNames, got)

			sup.AssertExpectations(t)
		})
	}
}

func TestServer_HandleGetAsset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		setup      func(dataDir string)
		assetName  string
		wantStatus int
		wantBody   string
	}{
		{
			name: "returns asset content",
			setup: func(dataDir string) {
				dir := filepath.Join(dataDir, "assets")
				require.NoError(t, os.MkdirAll(dir, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(dir, "image.png"), []byte("fake png"), 0o644))
			},
			assetName:  "image.png",
			wantStatus: http.StatusOK,
			wantBody:   "fake png",
		},
		{
			name:       "returns 404 when asset is absent",
			assetName:  "missing.png",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			dataDir := t.TempDir()
			if test.setup != nil {
				test.setup(dataDir)
			}

			sup := new(mockSupervisor)
			srv := newServer(sup, nil, "", dataDir)

			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/glass/assets/"+test.assetName, nil)
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)
			if test.wantBody != "" {
				assert.Equal(t, test.wantBody, w.Body.String())
			}

			sup.AssertExpectations(t)
		})
	}
}

func TestServer_HandleUploadAsset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		assetName  string
		body       string
		wantStatus int
	}{
		{
			name:       "uploads asset",
			assetName:  "image.png",
			body:       "fake png data",
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "path traversal is rejected by router",
			assetName:  "../escaped.txt",
			body:       "data",
			wantStatus: http.StatusTemporaryRedirect,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			sup := new(mockSupervisor)

			dataDir := t.TempDir()
			srv := newServer(sup, nil, "", dataDir)

			r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/glass/assets/"+test.assetName, strings.NewReader(test.body))
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)

			if test.wantStatus == http.StatusNoContent {
				got, err := os.ReadFile(filepath.Join(dataDir, "assets", test.assetName))
				require.NoError(t, err)
				assert.Equal(t, test.body, string(got))
			}

			sup.AssertExpectations(t)
		})
	}
}

func TestServer_HandleDeleteAsset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		setup      func(dataDir string)
		assetName  string
		wantStatus int
	}{
		{
			name: "deletes existing asset",
			setup: func(dataDir string) {
				dir := filepath.Join(dataDir, "assets")
				require.NoError(t, os.MkdirAll(dir, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(dir, "image.png"), []byte("data"), 0o644))
			},
			assetName:  "image.png",
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "returns no content when asset does not exist",
			assetName:  "missing.png",
			wantStatus: http.StatusNoContent,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			dataDir := t.TempDir()
			if test.setup != nil {
				test.setup(dataDir)
			}

			sup := new(mockSupervisor)
			srv := newServer(sup, nil, "", dataDir)

			r := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/glass/assets/"+test.assetName, nil)
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)

			sup.AssertExpectations(t)
		})
	}
}
