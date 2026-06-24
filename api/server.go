// Package api implements the HTTP management API for glass-agent.
package api

import (
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/glasslabs/supervisor/dbus"
	"github.com/glasslabs/supervisor/proc"
	"github.com/hamba/logger/v2"
)

// Supervisor is the interface the Server uses to control the glass process.
type Supervisor interface {
	Restart()
	Info() proc.Info
	Lines() []string
	Follow(ctx context.Context) <-chan string
}

// System is the interface the Server uses to control system services via D-Bus.
type System interface {
	Logind() dbus.Logind
	RAUC() dbus.RAUC
}

// Server serves the glass-agent HTTP management API.
type Server struct {
	addr       string
	supervisor Supervisor
	sys        System
	glassBin   string
	dataDir    string

	h http.Handler

	log *logger.Logger
}

// NewServer returns a new Server.
func NewServer(addr string, supervisor Supervisor, system System, glassBin, dataDir string, log *logger.Logger) *Server {
	s := &Server{
		addr:       addr,
		supervisor: supervisor,
		sys:        system,
		glassBin:   glassBin,
		dataDir:    dataDir,
		log:        log,
	}

	s.h = s.routes()

	return s
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /glass/status", s.handleStatus())
	mux.HandleFunc("GET /glass/logs", s.handleLogs())
	mux.HandleFunc("POST /glass/restart", s.handleRestart())
	mux.HandleFunc("POST /glass/update", s.handleUpdate())
	mux.HandleFunc("GET /glass/config", s.handleGetConfig())
	mux.HandleFunc("POST /glass/config", s.handleConfig())
	mux.HandleFunc("POST /glass/secrets", s.handleSecrets())
	mux.HandleFunc("GET /glass/assets", s.handleListAssets())
	mux.HandleFunc("GET /glass/assets/{name}", s.handleGetAsset())
	mux.HandleFunc("POST /glass/assets/{name}", s.handleUploadAsset())
	mux.HandleFunc("DELETE /glass/assets/{name}", s.handleDeleteAsset())
	mux.HandleFunc("POST /os/update", s.handleOSUpdate())
	mux.HandleFunc("GET /os/status", s.handleOSStatus())
	mux.HandleFunc("POST /os/reboot", s.handleOSReboot())

	return mux
}

// ServeHTTP serves an HTTP request.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.h.ServeHTTP(w, r)
}

func download(ctx context.Context, w, h io.Writer, url string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetching url: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	body := io.TeeReader(resp.Body, h)
	rc, err := decompress(body, resp.Header.Get("Content-Type"))
	if err != nil {
		return fmt.Errorf("decompressing response: %w", err)
	}
	defer func() { _ = rc.Close() }()

	_, err = io.Copy(w, rc)
	return err
}

func decompress(r io.Reader, contentType string) (io.ReadCloser, error) {
	switch contentType {
	case "application/gzip":
		gr, err := gzip.NewReader(r)
		if err != nil {
			return nil, fmt.Errorf("creating gzip reader: %w", err)
		}
		return gr, nil
	case "application/zip":
		f, err := os.CreateTemp("", "*.zip")
		if err != nil {
			return nil, fmt.Errorf("creating temporary file: %w", err)
		}
		closeFile := func() {
			_ = f.Close()
			_ = os.Remove(f.Name())
		}

		if _, err = io.Copy(f, r); err != nil {
			closeFile()
			return nil, fmt.Errorf("unzipping file: %w", err)
		}
		if _, err = f.Seek(0, io.SeekStart); err != nil {
			closeFile()
			return nil, fmt.Errorf("seeking file: %w", err)
		}
		stat, err := f.Stat()
		if err != nil {
			closeFile()
			return nil, fmt.Errorf("stating file: %w", err)
		}

		zr, err := zip.NewReader(f, stat.Size())
		if err != nil {
			return nil, fmt.Errorf("creating zip reader: %w", err)
		}
		if len(zr.File) == 0 {
			return nil, fmt.Errorf("zip archive is empty")
		}
		zf, err := zr.File[0].Open()
		if err != nil {
			return nil, fmt.Errorf("opening zip entry: %w", err)
		}
		return closeFuncReader{
			ReadCloser: zf,
			closeFn:    closeFile,
		}, nil
	default:
		return io.NopCloser(r), nil
	}
}

type closeFuncReader struct {
	io.ReadCloser

	closeFn func()
}

func (f closeFuncReader) Close() error {
	err := f.ReadCloser.Close()
	f.closeFn()
	return err
}
