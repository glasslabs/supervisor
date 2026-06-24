// Package web provides the HTTP management web UI for glass-agent.
package web

import (
	_ "embed"
	"html/template"
	"net/http"
)

//go:embed app.html
var appTmplStr string

var appTmpl = template.Must(template.New("app").Parse(appTmplStr))

// Server serves the glass-agent management web UI.
type Server struct {
	tmpl *template.Template
}

// NewServer returns a new Server.
// When apMode is true the UI shows a simplified WiFi provisioning view.
func NewServer() *Server {
	s := &Server{tmpl: appTmpl}
	return s
}

// ServeHTTP handles an HTTP request for the management UI.
func (s *Server) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	host := req.Host
	if host == "" {
		host = req.URL.Host
	}

	data := struct {
		Host string
	}{
		Host: host,
	}

	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.Execute(rw, data); err != nil {
		http.Error(rw, "internal server error", http.StatusInternalServerError)
	}
}
