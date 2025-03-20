package config

import (
	"os"
	"testing"
	"time"

	"github.com/iudanet/hls_exporter/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	configContent := `
server:
  port: 9090
  metrics_path: "/metrics"
  health_path: "/health"

checks:
  workers: 5
  retry_attempts: 3
  retry_delay: "1s"
  segment_sample: 3

http_client:
  timeout: "5s"
  keep_alive: true
  max_idle_conns: 10
  tls_verify: true
  user_agent: "hls_exporter/1.0"

streams:
  - name: "stream_1"
    url: "http://example.com/stream1.m3u8"
    check_mode: "first_last"
    interval: "30s"
    timeout: "10s"
    validate_content: false

  - name: "stream_2"
    url: "http://example.com/stream2.m3u8"
    check_mode: "all"
    interval: "1m"
    timeout: "15s"
    validate_content: true
    media_validation:
      container_type: ["TS"]
      min_segment_size: 1024
      check_audio: true
      check_video: true`

	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.Write([]byte(configContent))
	require.NoError(t, err)
	tmpfile.Close()

	t.Run("successful load", func(t *testing.T) {
		configLoader := NewConfigManager()
		t.Logf("Loaded config: %+v", tmpfile.Name())
		cfg, err := configLoader.LoadConfig(tmpfile.Name())
		require.NoError(t, err)
		require.NotNil(t, cfg)

		assert.Equal(t, 9090, cfg.Server.Port)
		assert.Equal(t, "/metrics", cfg.Server.MetricsPath)
		assert.Equal(t, 5, cfg.Checks.Workers)
		assert.Equal(t, 2, len(cfg.Streams))

		stream1 := cfg.Streams[0]
		assert.Equal(t, "stream_1", stream1.Name)
		assert.Equal(t, "first_last", stream1.CheckMode)
		assert.Equal(t, 30*time.Second, stream1.Interval)
		assert.False(t, stream1.ValidateContent)

		stream2 := cfg.Streams[1]
		assert.Equal(t, "stream_2", stream2.Name)
		assert.Equal(t, "all", stream2.CheckMode)
		assert.True(t, stream2.ValidateContent)
		require.NotNil(t, stream2.MediaValidation)
		assert.Equal(t, []string{"TS"}, stream2.MediaValidation.ContainerType)
	})
}
func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		configFile  string
		expectError string
	}{
		{
			name: "invalid port",
			configFile: `
server:
  port: 70000
streams:
  - name: "test"
    url: "http://example.com"
    check_mode: "all"
    interval: "30s"
    timeout: "10s"`,
			expectError: "invalid server port",
		},
		{
			name: "empty stream name",
			configFile: `
server:
  port: 9090
streams:
  - name: ""
    url: "http://example.com"
    check_mode: "all"
    interval: "30s"
    timeout: "10s"`,
			expectError: "name cannot be empty",
		},
		{
			name: "invalid check mode",
			configFile: `
server:
  port: 9090
streams:
  - name: "test"
    url: "http://example.com"
    check_mode: "invalid"
    interval: "30s"
    timeout: "10s"`,
			expectError: "invalid check_mode",
		},
		{
			name: "timeout greater than interval",
			configFile: `
server:
  port: 9090
streams:
  - name: "test"
    url: "http://example.com"
    check_mode: "all"
    interval: "10s"
    timeout: "30s"`,
			expectError: "timeout must be less than interval",
		},
		{
			name: "invalid media validation",
			configFile: `
server:
  port: 9090
streams:
  - name: "test"
    url: "http://example.com"
    check_mode: "all"
    interval: "30s"
    timeout: "10s"
    validate_content: true
    media_validation:
      container_type: ["invalid"]
      min_segment_size: 1024
      check_audio: true
      check_video: true`,
			expectError: "invalid container_type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpfile, err := os.CreateTemp("", "config-*.yaml")
			require.NoError(t, err)
			defer os.Remove(tmpfile.Name())

			_, err = tmpfile.Write([]byte(tt.configFile))
			require.NoError(t, err)
			tmpfile.Close()

			configLoader := NewConfigManager()
			_, err = configLoader.LoadConfig(tmpfile.Name())
			require.Error(t, err)
			t.Logf("Got error: %v", err) // добавим логирование для отладки
			assert.Contains(t, err.Error(), tt.expectError)
		})
	}
}

func TestEnvironmentOverrides(t *testing.T) {
	configContent := `
server:
  port: 9090
streams:
  - name: "test"
    url: "http://example.com"
    check_mode: "all"
    interval: "30s"
    timeout: "10s"` // Заменили табуляции на пробелы

	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.Write([]byte(configContent))
	require.NoError(t, err)
	tmpfile.Close()

	os.Setenv("HLS_SERVER_PORT", "8080")
	defer os.Unsetenv("HLS_SERVER_PORT")

	configLoader := NewConfigManager()
	cfg, err := configLoader.LoadConfig(tmpfile.Name())
	require.NoError(t, err)

	assert.Equal(t, 8080, cfg.Server.Port)
}

// Добавим тесты для валидатора отдельно
func TestConfigValidator(t *testing.T) {
	validator := NewValidator()

	t.Run("validate stream", func(t *testing.T) {
		stream := &models.StreamConfig{
			Name:      "test",
			URL:       "http://example.com",
			CheckMode: models.CheckModeAll,
			Interval:  30 * time.Second,
			Timeout:   10 * time.Second,
		}
		err := validator.ValidateStream(stream, 0)
		assert.NoError(t, err)
	})

	t.Run("validate media validation", func(t *testing.T) {
		mv := &models.MediaValidation{
			ContainerType:  []string{"TS"},
			MinSegmentSize: 1024,
			CheckAudio:     true,
			CheckVideo:     true,
		}
		err := validator.ValidateMediaValidation(mv, 0)
		assert.NoError(t, err)
	})
}
