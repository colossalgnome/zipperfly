package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/joho/godotenv"
	"go.uber.org/zap"

	"zipperfly/internal/auth"
	"zipperfly/internal/circuitbreaker"
	"zipperfly/internal/config"
	"zipperfly/internal/database"
	"zipperfly/internal/handlers"
	"zipperfly/internal/metrics"
	"zipperfly/internal/server"
	"zipperfly/internal/storage"
)

func main() {
	// Parse command-line flags
	configFile := flag.String("config", "", "Path to config file (overrides CONFIG_FILE env var)")
	flag.Parse()

	// Load environment variables from file
	loadEnvFile(*configFile)

	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatal("failed to init logger:", err)
	}
	defer logger.Sync()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	ctx := context.Background()

	// Initialize metrics
	m := metrics.New()
	m.StartRuntimeMetricsCollector()

	// Initialize circuit breakers
	storageBreaker := circuitbreaker.New("storage", cfg, m)
	logger.Info("initialized circuit breaker", zap.String("name", "storage"))

	// Initialize database
	db, err := database.New(ctx, cfg, m)
	if err != nil {
		logger.Fatal("failed to initialize database", zap.Error(err))
	}
	defer db.Close()
	logger.Info("initialized database", zap.String("engine", cfg.DBEngine))

	// Initialize storage provider
	storageProvider, err := storage.New(ctx, cfg, m, storageBreaker)
	if err != nil {
		logger.Fatal("failed to initialize storage provider", zap.Error(err))
	}
	logger.Info("initialized storage provider", zap.String("type", cfg.StorageType))

	// Initialize auth verifier
	verifier := auth.NewVerifier(cfg.SigningSecret, cfg.EnforceSigning, m)

	// Initialize download handler
	downloadHandler := handlers.NewHandler(
		logger,
		db,
		storageProvider,
		verifier,
		m,
		cfg.AppendYMD,
		cfg.SanitizeNames,
		cfg.IgnoreMissing,
		cfg.MaxConcurrent,
		cfg.CallbackMaxRetries,
		cfg.CallbackRetryDelay,
		cfg.AllowPasswordProtected,
		cfg.AllowedExtensions,
		cfg.BlockedExtensions,
		cfg.MaxActiveDownloads,
		cfg.MaxFilesPerRequest,
	)

	// Initialize health handler
	healthHandler := handlers.NewHealthHandler(logger, db, storageProvider, m)

	// Initialize and start server
	srv := server.New(logger, cfg, m, downloadHandler, healthHandler)
	if err := srv.Start(); err != nil {
		logger.Fatal("failed to start server", zap.Error(err))
	}

	// Wait for shutdown signal
	if err := srv.WaitForShutdown(); err != nil {
		logger.Error("shutdown error", zap.Error(err))
	}
}

// loadEnvFile loads environment variables from a file
// Priority: --config flag > CONFIG_FILE env var > .env file
// Silently continues if file doesn't exist (falls back to OS env vars)
func loadEnvFile(flagConfigFile string) {
	var configFile string

	// 1. Check --config flag
	if flagConfigFile != "" {
		configFile = flagConfigFile
	} else {
		// 2. Check CONFIG_FILE env var
		configFile = os.Getenv("CONFIG_FILE")
	}

	// 3. Try specified file or default to .env
	if configFile != "" {
		// User specified a file - fail if it doesn't exist
		if err := godotenv.Load(configFile); err != nil {
			log.Fatalf("failed to load config file %s: %v", configFile, err)
		}
		log.Printf("loaded config from: %s", configFile)
	} else {
		// Try .env but don't error if it doesn't exist
		if err := godotenv.Load(); err == nil {
			log.Println("loaded config from: .env")
		}
		// Silently continue if .env doesn't exist - will use OS env vars
	}
}
