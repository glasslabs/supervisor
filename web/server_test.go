package web_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/glasslabs/supervisor/web"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_ServeHTTP(t *testing.T) {
	t.Parallel()

	s := web.NewServer()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://glass.local/", nil)
	require.NoError(t, err)
	rec := httptest.NewRecorder()

	s.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/html; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Body.String(), "glass.local")
	assert.Contains(t, rec.Body.String(), "status-pid")
	assert.NotContains(t, rec.Body.String(), "Setup Mode")
}
