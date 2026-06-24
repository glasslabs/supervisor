package api

import (
	"encoding/json"
	"net/http"
	"time"
)

func (s *Server) handleRestart() http.HandlerFunc {
	return func(rw http.ResponseWriter, _ *http.Request) {
		s.supervisor.Restart()
		rw.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) handleStatus() http.HandlerFunc {
	return func(rw http.ResponseWriter, _ *http.Request) {
		info := s.supervisor.Info()

		uptime := ""
		if !info.Started.IsZero() {
			uptime = time.Since(info.Started).Truncate(time.Second).String()
		}

		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(map[string]any{
			"pid":      info.PID,
			"restarts": info.Restarts,
			"uptime":   uptime,
		})
	}
}
