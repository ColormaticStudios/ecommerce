package telemetry

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"gorm.io/gorm"
)

type ReadinessChecks struct {
	Pool           *sql.DB
	Database       *gorm.DB
	MigrationCheck func(*gorm.DB) error
	WorkersRunning func() bool
}

type Server struct {
	server    *http.Server
	listener  net.Listener
	checks    ReadinessChecks
	accepting atomic.Bool
	releaseID string
}

func NewServer(bindAddress, metricsPath string, metrics http.Handler, checks ReadinessChecks) *Server {
	server := &Server{checks: checks}
	mux := http.NewServeMux()
	mux.Handle(metricsPath, getOnly(metrics))
	mux.HandleFunc("/healthz", server.health)
	mux.HandleFunc("/readyz", server.ready)
	server.server = &http.Server{
		Addr: bindAddress, Handler: mux,
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second,
		WriteTimeout: 30 * time.Second, IdleTimeout: time.Minute,
	}
	return server
}

func (s *Server) Start() (<-chan error, error) {
	listener, err := net.Listen("tcp", s.server.Addr)
	if err != nil {
		return nil, err
	}
	s.listener = listener
	s.accepting.Store(true)
	errorsChannel := make(chan error, 1)
	go func() { errorsChannel <- s.server.Serve(listener) }()
	return errorsChannel, nil
}

func (s *Server) SetReady(ready bool) { s.accepting.Store(ready) }

func (s *Server) SetReleaseID(releaseID string) { s.releaseID = releaseID }

func (s *Server) Shutdown(ctx context.Context) error {
	s.accepting.Store(false)
	if err := s.server.Shutdown(ctx); err != nil {
		_ = s.server.Close()
		return err
	}
	return nil
}

func (s *Server) Address() string {
	if s.listener == nil {
		return s.server.Addr
	}
	return s.listener.Addr().String()
}

func (s *Server) health(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.writeReleaseHeader(writer)
	writeStatus(writer, http.StatusOK, "ok", map[string]string{"process": "ok"})
}

func (s *Server) ready(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.writeReleaseHeader(writer)
	checks := map[string]string{"accepting_traffic": "ok", "database": "ok", "migrations": "ok", "job_workers": "ok"}
	ready := true
	if !s.accepting.Load() {
		checks["accepting_traffic"] = "failed"
		ready = false
	}
	checkContext, cancel := context.WithTimeout(request.Context(), 2*time.Second)
	defer cancel()
	if s.checks.Pool == nil || s.checks.Pool.PingContext(checkContext) != nil {
		checks["database"] = "failed"
		ready = false
	}
	if s.checks.Database == nil || s.checks.MigrationCheck == nil || s.checks.MigrationCheck(s.checks.Database.WithContext(checkContext)) != nil {
		checks["migrations"] = "failed"
		ready = false
	}
	if s.checks.WorkersRunning == nil || !s.checks.WorkersRunning() {
		checks["job_workers"] = "failed"
		ready = false
	}
	if !ready {
		writeStatus(writer, http.StatusServiceUnavailable, "not_ready", checks)
		return
	}
	writeStatus(writer, http.StatusOK, "ready", checks)
}

func (s *Server) writeReleaseHeader(writer http.ResponseWriter) {
	if s.releaseID != "" {
		writer.Header().Set("X-Ecommerce-Release-ID", s.releaseID)
	}
}

func writeStatus(writer http.ResponseWriter, status int, value string, checks map[string]string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]any{"status": value, "checks": checks})
}

func getOnly(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			writer.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handler.ServeHTTP(writer, request)
	})
}

func IsServerClosed(err error) bool { return errors.Is(err, http.ErrServerClosed) }
