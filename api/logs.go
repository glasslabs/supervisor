package api

import (
	"fmt"
	"net/http"
	"strings"
)

func (s *Server) handleLogs() http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		follow := req.URL.Query().Get("follow") == "true"

		rw.Header().Set("Content-Type", "text/plain; charset=utf-8")
		rw.Header().Set("X-Content-Type-Options", "nosniff")

		lines := s.supervisor.Lines()
		if len(lines) > 0 {
			_, _ = fmt.Fprint(rw, strings.Join(lines, "\n")+"\n")
		}

		if !follow {
			return
		}

		flusher, ok := rw.(http.Flusher)
		if !ok {
			return
		}
		flusher.Flush()

		ch := s.supervisor.Follow(req.Context())
		for line := range ch {
			_, _ = fmt.Fprint(rw, line+"\n")
			flusher.Flush()
		}
	}
}
