// Package main is the GlassOS management agent.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	stdlog "log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/glasslabs/supervisor/api"
	"github.com/glasslabs/supervisor/dbus"
	"github.com/glasslabs/supervisor/internal/exec"
	"github.com/glasslabs/supervisor/proc"
	"github.com/glasslabs/supervisor/web"
	"github.com/hamba/logger/v2"
	lctx "github.com/hamba/logger/v2/ctx"
)

var version = "dev"

func main() {
	os.Exit(realMain())
}

func realMain() (code int) {
	addr := flag.String("addr", ":80", "HTTP server listen address")
	glassBin := flag.String("glass-bin", "/usr/lib/glass/glass", "Path to the glass binary")
	dataDir := flag.String("data-dir", "/data", "Path to the data directory")
	logLevel := flag.String("log.level", "info", "Log level (trace, debug, info, warn, error)")
	flag.Parse()

	lvl, err := logger.LevelFromString(*logLevel)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Invalid log level %q: %v\n", *logLevel, err)
		return 1
	}

	log := logger.New(os.Stderr, logger.LogfmtFormat(), lvl)
	log.Info("Starting glass-agent", lctx.Str("version", version))

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	exe := &exec.Executable{
		Path: "/usr/bin/cage",
		Args: []string{
			"--",
			*glassBin, "run",
			"--config", filepath.Join(*dataDir, "config", "config.yaml"),
			"--secrets", filepath.Join(*dataDir, "config", "secrets.yaml"),
			"--assets", filepath.Join(*dataDir, "assets"),
			"--modules", filepath.Join(*dataDir, "modules"),
		},
		Envs:        os.Environ(),
		SysProcAttr: &syscall.SysProcAttr{Setpgid: true},
	}
	super := proc.New(exe, proc.WithCondition(needsConfig(*dataDir)))

	sys, err := dbus.New()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Could not connect to system D-Bus: %v\n", err)
		return 1
	}
	defer func() { _ = sys.Close() }()

	apiSrv := api.NewServer(*addr, super, sys, *glassBin, *dataDir, log)
	webSrv := web.NewServer()

	if err = super.Start(ctx); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Could not start supervisor: %v\n", err)
		return 1
	}

	mux := http.NewServeMux()
	mux.Handle("GET /{$}", webSrv)
	mux.Handle("/", apiSrv)

	srv := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
		ErrorLog:          stdlog.New(log.Writer(logger.Error), "", 0),
	}
	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.WithoutCancel(ctx))
	}()

	log.Info("Starting server", lctx.Str("addr", *addr))

	if err = srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		_, _ = fmt.Fprintf(os.Stderr, "Could not run server: %v\n", err)
		return 1
	}
	return 0
}

func needsConfig(dataDir string) proc.Condition {
	return func() (bool, string) {
		cfgPath := filepath.Join(dataDir, "config", "config.yaml")
		if _, err := os.Stat(cfgPath); err != nil {
			return false, "waiting for config file: " + cfgPath
		}
		return true, ""
	}
}
