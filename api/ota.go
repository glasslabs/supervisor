package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

func (s *Server) handleUpdate() http.HandlerFunc {
	type otaRequest struct {
		URL    string `json:"url"`
		SHA256 string `json:"sha256"`
	}

	return func(rw http.ResponseWriter, req *http.Request) {
		var otaReq otaRequest
		if err := json.NewDecoder(req.Body).Decode(&otaReq); err != nil {
			http.Error(rw, "invalid request body", http.StatusBadRequest)
			return
		}
		if otaReq.URL == "" || otaReq.SHA256 == "" {
			http.Error(rw, "url and sha256 are required", http.StatusBadRequest)
			return
		}

		tmp, err := os.CreateTemp("", "glass-ota-*")
		if err != nil {
			http.Error(rw, "creating temp file", http.StatusInternalServerError)
			return
		}
		tmpName := tmp.Name()
		defer func() { _ = os.Remove(tmpName) }()

		h := sha256.New()
		if err = download(req.Context(), tmp, h, otaReq.URL); err != nil {
			_ = tmp.Close()
			http.Error(rw, fmt.Sprintf("downloading binary: %v", err), http.StatusBadGateway)
			return
		}
		if err = tmp.Close(); err != nil {
			http.Error(rw, "closing temp file", http.StatusInternalServerError)
			return
		}

		got := hex.EncodeToString(h.Sum(nil))
		if got != otaReq.SHA256 {
			http.Error(rw, "sha256 mismatch", http.StatusBadRequest)
			return
		}

		if err = os.Chmod(tmpName, 0o755); err != nil {
			http.Error(rw, "setting permissions", http.StatusInternalServerError)
			return
		}

		if err = os.Rename(tmpName, s.glassBin); err != nil {
			http.Error(rw, fmt.Sprintf("replacing binary: %v", err), http.StatusInternalServerError)
			return
		}

		s.supervisor.Restart()

		rw.WriteHeader(http.StatusNoContent)
	}
}
