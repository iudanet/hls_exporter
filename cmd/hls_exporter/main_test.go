package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/iudanet/hls_exporter/internal/checker"
	"github.com/iudanet/hls_exporter/internal/config"
	client "github.com/iudanet/hls_exporter/internal/http"
	"github.com/iudanet/hls_exporter/internal/metrics"
	"github.com/iudanet/hls_exporter/pkg/models"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testSegmentPath = "/segment1.ts"
	testStreamPath  = "/stream.m3u8"
	testM3U8Path    = "/test.m3u8"
	baseContentTpl  = `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-STREAM-INF:BANDWIDTH=1000000
%s/stream.m3u8`
	mediaContentTpl = `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:10
#EXTINF:10.0,
%s/segment1.ts`
)

func getTestConfig() string {
	return `
server:
    port: 9091
    metrics_path: "/metrics"
    health_path: "/health"

logging:
    level: "debug"
    encoding: "json"
    development: false

streams:
    - name: "test_stream"
      url: "%s/test.m3u8"
      check_mode: "first_last"
      interval: "5s"
      timeout: "2s"
      validate_content: false
`
}

func TestRunStreamChecks(t *testing.T) {
	// Создаем отдельный регистр для тестов
	reg, testServerURL, cleanup := setupTest(t)
	defer cleanup()
	baseContent := `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-STREAM-INF:BANDWIDTH=1000000
%s/stream.m3u8`

	mediaContent := `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:10
#EXTINF:10.0,
%s/segment1.ts`

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case testM3U8Path:
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, baseContent, testServerURL)

		case testStreamPath:
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, mediaContent, testServerURL)

		case testSegmentPath:
			w.Header().Set("Content-Type", "video/MP2T")
			w.Header().Set("Content-Length", "1024")
			w.WriteHeader(http.StatusOK)
			_, err := w.Write(make([]byte, 1024))
			if err != nil {
				t.Errorf("Failed to write response: %v", err)
			}

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer testServer.Close()

	testServerURL = testServer.URL

	// Создаем временный конфиг
	configContent := fmt.Sprintf(`
server:
    port: 9091
    metrics_path: "/metrics"
    health_path: "/health"

checks:
    workers: 2
    retry_attempts: 1
    retry_delay: "1s"
    segment_sample: 2

http_client:
    timeout: "2s"
    keep_alive: true
    max_idle_conns: 5
    tls_verify: true
    user_agent: "hls_exporter_test/1.0"

streams:
    - name: "test_stream"
      url: "%s/test.m3u8"
      check_mode: "first_last"
      interval: "5s"
      timeout: "2s"
      validate_content: false
`, testServerURL)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test_config.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Загружаем конфигурацию
	configLoader := config.NewConfigManager()
	cfg, err := configLoader.LoadConfig(configPath)
	require.NoError(t, err)

	// Инициализируем компоненты
	metricsCollector := metrics.NewCollector(reg)
	httpClient := client.NewClient(cfg.HTTPClient)
	validator := checker.NewHLSValidator()

	streamChecker := checker.NewStreamChecker(
		httpClient,
		validator,
		metricsCollector,
		cfg.Checks.Workers,
	)

	// Запускаем чекер
	err = streamChecker.Start()
	require.NoError(t, err)
	defer func() {
		err := streamChecker.Stop()
		if err != nil {
			t.Errorf("Failed to stop stream checker: %v", err)
		}
	}()

	// Запускаем одну проверку
	streamCfg := cfg.Streams[0]
	result, err := streamChecker.Check(context.Background(), streamCfg)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Проверяем результат
	assert.True(t, result.Success, "Check should be successful")
	assert.Equal(t, 1, result.Segments.Checked, "Should check one segment")
	assert.Equal(t, 0, result.Segments.Failed, "Should have no failed segments")

	// Проверяем метрики
	collector := metricsCollector.(*metrics.Collector)
	streamUp := collector.GetStreamUp(streamCfg.Name)
	assert.Equal(t, float64(1), streamUp, "Stream should be up")

	errorsTotal := collector.GetErrorsTotal(streamCfg.Name, string(models.ErrSegmentValidate))
	assert.Equal(t, float64(0), errorsTotal, "Should have no validation errors")
}

// TestHealthCheckHandler тестирует обработчик health check
func TestMainIntegration(t *testing.T) {
	// Сохраняем оригинальный регистр
	originalReg := prometheus.DefaultRegisterer
	// Создаем новый регистр для теста
	testReg := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = testReg
	// Восстанавливаем оригинальный регистр после теста
	defer func() {
		prometheus.DefaultRegisterer = originalReg
	}()

	var testServerURL string
	testServer := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		// ... существующий код обработчика ...
	}))
	defer testServer.Close()

	testServerURL = testServer.URL

	// Создаем временный конфиг с настройками логгера
	configContent := fmt.Sprintf(getTestConfig(), testServerURL)

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Переопределяем аргументы командной строки
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"cmd", "-config", configPath}

	// Запускаем main в отдельной горутине
	done := make(chan struct{})
	go func() {
		main()
		close(done)
	}()

	// Ждем запуска сервиса
	time.Sleep(2 * time.Second)

	// Отправляем сигнал завершения
	p, err := os.FindProcess(os.Getpid())
	require.NoError(t, err)
	err = p.Signal(os.Interrupt)
	require.NoError(t, err)

	// Ждем завершения с таймаутом
	select {
	case <-done:
		// OK
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for shutdown")
	}
}

func setupTest(t *testing.T) (*prometheus.Registry, string, func()) {
	reg := prometheus.NewRegistry()

	// Создаем тестовый сервер
	baseContent := baseContentTpl

	mediaContent := mediaContentTpl

	var testServerURL string
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case testM3U8Path:
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, baseContent, testServerURL)
		case testStreamPath:
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, mediaContent, testServerURL)
		case testSegmentPath:
			w.Header().Set("Content-Type", "video/MP2T")
			w.Header().Set("Content-Length", "1024")
			w.WriteHeader(http.StatusOK)
			_, err := w.Write(make([]byte, 1024))
			if err != nil {
				t.Errorf("Failed to write response: %v", err)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	testServerURL = testServer.URL

	cleanup := func() {
		testServer.Close()
	}

	return reg, testServerURL, cleanup
}
