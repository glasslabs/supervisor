package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func (s *Server) handleOSUpdate() http.HandlerFunc {
	type osUpdateRequest struct {
		URL string `json:"url"`
	}

	return func(rw http.ResponseWriter, req *http.Request) {
		var updateReq osUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&updateReq); err != nil {
			http.Error(rw, "invalid request body", http.StatusBadRequest)
			return
		}
		if updateReq.URL == "" {
			http.Error(rw, "url is required", http.StatusBadRequest)
			return
		}

		tmp, err := os.CreateTemp("", "glassos-update-*.raucb")
		if err != nil {
			http.Error(rw, "creating temp file", http.StatusInternalServerError)
			return
		}
		tmpName := tmp.Name()
		defer func() { _ = os.Remove(tmpName) }()

		if err = download(req.Context(), tmp, io.Discard, updateReq.URL); err != nil {
			_ = tmp.Close()
			http.Error(rw, fmt.Sprintf("downloading bundle: %v", err), http.StatusBadGateway)
			return
		}
		if err = tmp.Close(); err != nil {
			http.Error(rw, "closing bundle", http.StatusInternalServerError)
			return
		}

		// Use WithoutCancel so a client disconnect does not abort the installation.
		if err = s.sys.RAUC().Install(context.WithoutCancel(req.Context()), tmpName); err != nil {
			http.Error(rw, fmt.Sprintf("could not install bundle: %v", err), http.StatusInternalServerError)
			return
		}

		rw.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) handleOSStatus() http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		status, err := s.sys.RAUC().Status(req.Context())
		if err != nil {
			http.Error(rw, fmt.Sprintf("could not get OS status: %v", err), http.StatusInternalServerError)
			return
		}

		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(status)
	}
}

func (s *Server) handleOSReboot() http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusNoContent)
		if f, ok := rw.(http.Flusher); ok {
			f.Flush()
		}

		// Allow the response to reach the client before the system reboots.
		time.Sleep(500 * time.Millisecond)

		if err := s.sys.Logind().Reboot(context.WithoutCancel(req.Context())); err != nil {
			http.Error(rw, fmt.Sprintf("could not reboot: %v", err), http.StatusInternalServerError)
			return
		}
	}
}
