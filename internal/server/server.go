package server

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"golang.org/x/crypto/acme/autocert"

	"zipperfly/internal/config"
	"zipperfly/internal/handlers"
	"zipperfly/internal/metrics"
)

// Server wraps the HTTP server
type Server struct {
	logger *zap.Logger
	cfg    *config.Config
	srv    *http.Server
}

// New creates a new server instance
func New(logger *zap.Logger, cfg *config.Config, m *metrics.Metrics, downloadHandler *handlers.Handler, healthHandler *handlers.HealthHandler) *Server {
	r := mux.NewRouter()

	// Add request ID middleware
	r.Use(handlers.RequestIDMiddleware)

	// Metrics endpoint with optional basic auth
	metricsHandler := promhttp.Handler()
	if cfg.MetricsUsername != "" && cfg.MetricsPassword != "" {
		authMiddleware := handlers.BasicAuth(cfg.MetricsUsername, cfg.MetricsPassword)
		r.Handle("/metrics", authMiddleware(metricsHandler))
	} else {
		r.Handle("/metrics", metricsHandler)
	}

	// Health endpoint
	r.HandleFunc("/health", healthHandler.Health).Methods("GET")

	// Download endpoint
	r.HandleFunc("/{id}", downloadHandler.Download).Methods("GET")

	return &Server{
		logger: logger,
		cfg:    cfg,
		srv:    &http.Server{Handler: r},
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	if s.cfg.EnableHTTPS {
		return s.startHTTPS()
	}
	return s.startHTTP()
}

func (s *Server) startHTTP() error {
	s.srv.Addr = ":" + s.cfg.Port
	s.logger.Info("starting HTTP server", zap.String("addr", s.srv.Addr))

	go func() {
		if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Fatal("HTTP server error", zap.Error(err))
		}
	}()

	return nil
}

func (s *Server) startHTTPS() error {
	m := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(s.cfg.LetsEncryptDomains...),
		Cache:      autocert.DirCache(s.cfg.LetsEncryptCacheDir),
		Email:      s.cfg.LetsEncryptEmail,
	}

	// HTTP server for ACME challenges and redirects
	go func() {
		s.logger.Info("starting HTTP server for challenges/redirects", zap.String("addr", ":80"))
		if err := http.ListenAndServe(":80", m.HTTPHandler(nil)); err != nil {
			s.logger.Error("HTTP server error", zap.Error(err))
		}
	}()

	s.srv.Addr = ":443"
	s.srv.TLSConfig = &tls.Config{GetCertificate: m.GetCertificate}
	s.logger.Info("starting HTTPS server", zap.String("addr", s.srv.Addr), zap.Strings("domains", s.cfg.LetsEncryptDomains))

	go func() {
		if err := s.srv.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Fatal("HTTPS server error", zap.Error(err))
		}
	}()

	return nil
}

// WaitForShutdown waits for interrupt signal and gracefully shuts down the server
func (s *Server) WaitForShutdown() error {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	s.logger.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.srv.Shutdown(ctx); err != nil {
		return err
	}

	s.logger.Info("server stopped")
	return nil
}
