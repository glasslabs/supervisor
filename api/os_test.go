package api_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/glasslabs/supervisor/dbus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestServer_HandleOSUpdate(t *testing.T) {
	t.Parallel()

	// Serve a fake bundle for download cases.
	bundleSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("fake-bundle"))
	}))
	t.Cleanup(bundleSrv.Close)

	errorSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(errorSrv.Close)

	tests := []struct {
		name       string
		body       string
		setupRAUC  func(*mockRAUC)
		wantStatus int
	}{
		{
			name:       "invalid json body",
			body:       "not json",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing url",
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "download failure",
			body:       `{"url":"` + errorSrv.URL + `"}`,
			wantStatus: http.StatusBadGateway,
		},
		{
			name: "install failure",
			body: `{"url":"` + bundleSrv.URL + `"}`,
			setupRAUC: func(m *mockRAUC) {
				m.On("Install", mock.AnythingOfType("string")).Return(errors.New("installation failed"))
			},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name: "installs bundle successfully",
			body: `{"url":"` + bundleSrv.URL + `"}`,
			setupRAUC: func(m *mockRAUC) {
				m.On("Install", mock.AnythingOfType("string")).Return(nil)
			},
			wantStatus: http.StatusNoContent,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			rauc := &mockRAUC{}
			if test.setupRAUC != nil {
				test.setupRAUC(rauc)
			}
			sup := &mockSupervisor{}
			srv := newServer(sup, system{rauc: rauc}, "", t.TempDir())

			r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/os/update", strings.NewReader(test.body))
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, r)

			assert.Equal(t, test.wantStatus, w.Code)

			sup.AssertExpectations(t)
			rauc.AssertExpectations(t)
		})
	}
}

func TestServer_HandleOSStatus(t *testing.T) {
	t.Parallel()

	sup := &mockSupervisor{}
	rauc := &mockRAUC{}
	rauc.On("Status").Return(dbus.RAUCStatus{
		Compatible: "GlassOS",
		BootSlot:   "rootfs.0",
	}, nil)
	srv := newServer(sup, system{rauc: rauc}, "", t.TempDir())

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/os/status", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "GlassOS")

	sup.AssertExpectations(t)
	rauc.AssertExpectations(t)
}

func TestServer_HandleOSReboot(t *testing.T) {
	t.Parallel()

	sup := &mockSupervisor{}
	logind := &mockLogind{}
	logind.On("Reboot").Return(nil)
	srv := newServer(sup, system{logind: logind}, "", t.TempDir())

	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/os/reboot", nil)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.ServeHTTP(w, r)
	}()

	require.Eventually(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, 2*time.Second, 10*time.Millisecond)

	assert.Equal(t, http.StatusNoContent, w.Code)

	sup.AssertExpectations(t)
	logind.AssertExpectations(t)
}

