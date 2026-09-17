package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/LudwigJMarx/vigil/internal/api"
)

// DefaultAddr binds to this machine only. Anyone who wants vigil reachable
// from elsewhere says so explicitly, which is also where the token check
// below takes effect.
const DefaultAddr = "127.0.0.1:8099"

func runServe(args []string, stdout, stderr io.Writer) error {
	set := flag.NewFlagSet("vigil serve", flag.ContinueOnError)
	set.SetOutput(stderr)
	addr := set.String("addr", envOr("VIGIL_ADDR", DefaultAddr), "address to listen on (env VIGIL_ADDR)")
	dbPath := set.String("db", "", "database file (env VIGIL_DB)")
	logFormat := set.String("log", envOr("VIGIL_LOG", "text"), "log format: text or json")
	if err := set.Parse(args); err != nil {
		return err
	}

	var handler slog.Handler
	if *logFormat == "json" {
		handler = slog.NewJSONHandler(stderr, nil)
	} else {
		handler = slog.NewTextHandler(stderr, nil)
	}
	log := slog.New(handler)

	path := resolveDBPath(*dbPath)
	db, err := openStore(path)
	if err != nil {
		return err
	}
	defer db.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	active, err := db.CountActiveTokens(ctx)
	if err != nil {
		return err
	}
	if !loopback(*addr) && active == 0 {
		// An instance on a public address with no credential is an open
		// database. Refusing to start is the only answer that cannot be
		// missed; a warning in a log scrolls past.
		return fmt.Errorf(
			"refusing to listen on %s with no token issued: run 'vigil token create --name browser' first, "+
				"or bind to %s", *addr, DefaultAddr)
	}

	server := &http.Server{
		Addr: *addr,
		Handler: api.New(api.Options{
			Store: db, Log: log, Version: Version,
		}),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "vigil %s listening on http://%s\n", Version, listener.Addr())
	fmt.Fprintf(stdout, "database: %s\n", path)
	if active == 0 {
		fmt.Fprintf(stdout, "no token issued yet: run 'vigil token create --name browser'\n")
	}
	if !loopback(*addr) {
		fmt.Fprintf(stdout,
			"warning: %s is reachable from the network and vigil speaks plain HTTP. "+
				"Put it behind a reverse proxy that terminates TLS.\n", *addr)
	}

	errs := make(chan error, 1)
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errs <- err
			return
		}
		errs <- nil
	}()

	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		fmt.Fprintln(stdout, "shutting down")
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return <-errs
	}
}

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
