package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/iudanet/hls_exporter/internal/checker"
	"github.com/iudanet/hls_exporter/internal/config"
	client "github.com/iudanet/hls_exporter/internal/http"
	"github.com/iudanet/hls_exporter/internal/metrics"
	"github.com/iudanet/hls_exporter/pkg/models"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

var (
	configFile = flag.String("config", "config.yaml", "Path to configuration file")
)

func main() {
	flag.Parse()

	// Инициализация логгера
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		err := logger.Sync()
		if err != nil {
			fmt.Printf("Failed to sync logger: %v\n", err)
		}
	}()

	// Загрузка конфигурации
	configLoader := config.NewConfigManager()
	cfg, err := configLoader.LoadConfig(*configFile)
	if err != nil {
		logger.Fatal("Failed to load configuration",
			zap.String("file", *configFile),
			zap.Error(err))
	}

	// Инициализация компонентов
	metricsCollector := metrics.NewCollector(nil) // nil использует DefaultRegisterer

	httpClient := client.NewClient(cfg.HTTPClient)
	defer httpClient.Close()
	validator := checker.NewHLSValidator()

	// Инициализация чекера
	streamChecker := checker.NewStreamChecker(
		httpClient,
		validator,
		metricsCollector,
		cfg.Checks.Workers,
	)

	// Запуск чекера
	if err := streamChecker.Start(); err != nil {
		logger.Fatal("Failed to start stream checker", zap.Error(err))
	}

	// HTTP сервер для метрик
	mux := http.NewServeMux()
	mux.Handle(cfg.Server.MetricsPath, promhttp.Handler())
	mux.HandleFunc(cfg.Server.HealthPath, healthCheckHandler)

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second, // Защита от Slowloris атак
	}

	// Канал для graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Запуск HTTP сервера
	go func() {
		logger.Info("Starting HTTP server",
			zap.String("address", server.Addr),
			zap.String("metrics_path", cfg.Server.MetricsPath))

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start HTTP server", zap.Error(err))
		}
	}()

	// Запуск проверок стримов
	for _, streamCfg := range cfg.Streams {
		go runStreamChecks(context.Background(), streamChecker, streamCfg, logger)
	}

	// Ожидание сигнала завершения
	<-stop
	logger.Info("Shutting down...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Остановка компонентов
	if err := streamChecker.Stop(); err != nil {
		logger.Error("Error stopping stream checker", zap.Error(err))
	}

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Error shutting down HTTP server", zap.Error(err))
	}

	logger.Info("Shutdown complete")
}

// runStreamChecks запускает периодические проверки для стрима
func runStreamChecks(ctx context.Context, checker *checker.StreamChecker, cfg models.StreamConfig, logger *zap.Logger) {
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	for {
		checkCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
		result, err := checker.Check(checkCtx, cfg)
		cancel()

		if err != nil {
			logger.Error("Stream check failed",
				zap.String("stream", cfg.Name),
				zap.Error(err))
		} else {
			logger.Debug("Stream check completed",
				zap.String("stream", cfg.Name),
				zap.Bool("success", result.Success))
		}

		select {
		case <-ticker.C:
			continue
		case <-checker.StopCh():
			return
		case <-ctx.Done():
			return
		}
	}
}

// healthCheckHandler для endpoint /health
func healthCheckHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		log.Printf("Error writing response: %v", err)
	}
}
