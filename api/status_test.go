package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/glasslabs/supervisor/proc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_HandleRestart(t *testing.T) {
	t.Parallel()

	sup := new(mockSupervisor)
	sup.On("Restart").Return()

	srv := newServer(sup, nil, "", t.TempDir())

	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/glass/restart", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)
	sup.AssertExpectations(t)
}

func TestServer_HandleStatus(t *testing.T) {
	t.Parallel()

	started := time.Now().Add(-5 * time.Minute)

	tests := []struct {
		name         string
		info         proc.Info
		wantPID      float64
		wantRestarts float64
		wantUptime   bool
	}{
		{
			name:         "running process",
			info:         proc.Info{PID: 1234, Restarts: 2, Started: started},
			wantPID:      1234,
			wantRestarts: 2,
			wantUptime:   true,
		},
		{
			name:         "not yet started",
			info:         proc.Info{},
			wantPID:      0,
			wantRestarts: 0,
			wantUptime:   false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			sup := new(mockSupervisor)
			sup.On("Info").Return(test.info)

			srv := newServer(sup, nil, "", t.TempDir())

			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/glass/status", nil)
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, r)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var got map[string]any
			require.NoError(t, json.NewDecoder(w.Body).Decode(&got))

			assert.Equal(t, test.wantPID, got["pid"])
			assert.Equal(t, test.wantRestarts, got["restarts"])
			if test.wantUptime {
				assert.NotEmpty(t, got["uptime"])
			} else {
				assert.Equal(t, "", got["uptime"])
			}

			sup.AssertExpectations(t)
		})
	}
}
