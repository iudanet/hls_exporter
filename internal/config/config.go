package config

import (
	"fmt"
	"strings"

	"github.com/iudanet/hls_exporter/pkg/models"

	"github.com/spf13/viper"
)

// Проверка имплементации интерфейсов на этапе компиляции
var (
	_ models.ConfigLoader    = (*Manager)(nil)
	_ models.ConfigValidator = (*Validator)(nil)
)

type Manager struct {
	viper     *viper.Viper
	validator models.ConfigValidator
}

func NewConfigManager() models.ConfigLoader {
	return &Manager{
		viper:     viper.New(),
		validator: NewValidator(),
	}
}

// LoadConfig имплементация интерфейса ConfigLoader
func (cm *Manager) LoadConfig(path string) (*models.Config, error) {
	// Настройка Viper
	cm.viper.SetConfigFile(path)
	cm.viper.SetEnvPrefix("HLS")
	cm.viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	cm.viper.AutomaticEnv()

	// Установка дефолтных значений
	cm.viper.SetConfigType("yaml")
	cm.setDefaults()

	// Чтение конфига
	err := cm.viper.ReadInConfig()
	if err != nil {
		return nil, fmt.Errorf("error reading config: %w", err)
	}

	var config models.Config
	if err := cm.viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	validator := NewValidator()
	if err := validator.Validate(&config); err != nil {
		return nil, fmt.Errorf("config validation error: %w", err)
	}

	return &config, nil
}

// ConfigValidator имплементация интерфейса ConfigValidator
type Validator struct{}

func NewValidator() models.ConfigValidator {
	return &Validator{}
}

func (cv *Validator) Validate(cfg *models.Config) error {
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", cfg.Server.Port)
	}

	if cfg.Checks.Workers <= 0 {
		return fmt.Errorf("workers must be greater than 0")
	}

	if cfg.Checks.RetryAttempts < 0 {
		return fmt.Errorf("retry_attempts cannot be negative")
	}

	if len(cfg.Streams) == 0 {
		return fmt.Errorf("no streams configured")
	}

	for i, stream := range cfg.Streams {
		if err := cv.ValidateStream(&stream, i); err != nil {
			return err
		}
	}

	return nil
}

// setDefaults устанавливает значения по умолчанию
func (cm *Manager) setDefaults() {
	cm.viper.SetDefault("server.port", 9090)
	cm.viper.SetDefault("server.metrics_path", "/metrics")
	cm.viper.SetDefault("server.health_path", "/health")

	cm.viper.SetDefault("checks.workers", 5)
	cm.viper.SetDefault("checks.retry_attempts", 3)
	cm.viper.SetDefault("checks.retry_delay", "1s")
	cm.viper.SetDefault("checks.segment_sample", 3)

	cm.viper.SetDefault("http_client.timeout", "5s")
	cm.viper.SetDefault("http_client.keep_alive", true)
	cm.viper.SetDefault("http_client.max_idle_conns", 10)
	cm.viper.SetDefault("http_client.tls_verify", true)
	cm.viper.SetDefault("http_client.user_agent", "hls_exporter/1.0")
}

// validateStream проверяет конфигурацию отдельного стрима
func (cv *Validator) ValidateStream(stream *models.StreamConfig, index int) error {

	if stream.Name == "" {
		return fmt.Errorf("stream[%d]: name cannot be empty", index)
	}

	if stream.URL == "" {
		return fmt.Errorf("stream[%d]: url cannot be empty", index)
	}

	// Проверка CheckMode
	validModes := map[string]bool{
		models.CheckModeAll:       true,
		models.CheckModeFirstLast: true,
		models.CheckModeRandom:    true,
	}
	if !validModes[stream.CheckMode] {
		return fmt.Errorf("stream[%d]: invalid check_mode: %s", index, stream.CheckMode)
	}

	// Проверка интервалов
	if stream.Interval <= 0 {
		return fmt.Errorf("stream[%d]: interval must be greater than 0", index)
	}

	if stream.Timeout <= 0 {
		return fmt.Errorf("stream[%d]: timeout must be greater than 0", index)
	}

	if stream.Timeout >= stream.Interval {
		return fmt.Errorf("stream[%d]: timeout must be less than interval", index)
	}

	// Проверка MediaValidation если включена валидация контента
	if stream.ValidateContent && stream.MediaValidation != nil {
		if err := cv.ValidateMediaValidation(stream.MediaValidation, index); err != nil {
			return err
		}
	}

	return nil
}

// validateMediaValidation проверяет настройки валидации медиа
func (cv *Validator) ValidateMediaValidation(mv *models.MediaValidation, streamIndex int) error {
	if len(mv.ContainerType) == 0 {
		return fmt.Errorf("stream[%d]: media_validation: container_type cannot be empty", streamIndex)
	}

	validContainers := map[string]bool{"TS": true, "fMP4": true}
	for _, ct := range mv.ContainerType {
		if !validContainers[ct] {
			return fmt.Errorf("stream[%d]: media_validation: invalid container_type: %s", streamIndex, ct)
		}
	}

	if mv.MinSegmentSize < 0 {
		return fmt.Errorf("stream[%d]: media_validation: min_segment_size cannot be negative", streamIndex)
	}

	return nil
}
