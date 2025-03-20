package models

import (
	"context"
	"net/http"
	"time"

	"github.com/grafov/m3u8"
)

// Основные интерфейсы

type Checker interface {
	// Основной метод проверки
	Check(ctx context.Context, stream StreamConfig) (*CheckResult, error)
	// Управление жизненным циклом
	Start() error
	Stop() error
}

type Validator interface {
	// Валидация Master Playlist
	ValidateMaster(playlist *m3u8.MasterPlaylist) error
	// Валидация Media Playlist
	ValidateMedia(playlist *m3u8.MediaPlaylist) error
	// Валидация сегмента с опциональной проверкой медиаконтейнера
	ValidateSegment(segment *SegmentData, validation *MediaValidation) error
}

type HTTPClient interface {
	// Загрузка плейлиста
	GetPlaylist(ctx context.Context, url string) (*PlaylistResponse, error)
	// Загрузка и валидация сегмента
	GetSegment(ctx context.Context, url string, validate bool) (*SegmentResponse, error)
	// Конфигурация клиента
	SetTimeout(timeout time.Duration)
	Close() error
}

type MetricsCollector interface {
	// Основные метрики
	SetStreamUp(name string, up bool)
	RecordResponseTime(name string, duration float64)
	RecordSegmentCheck(name string, success bool)
	// Детальные метрики
	SetStreamBitrate(name string, bitrate float64)
	SetSegmentsCount(name string, count int)
	RecordError(name, errorType string)
	// Служебные метрики
	SetLastCheckTime(name string, timestamp time.Time)
	SetActiveChecks(count int)
}

// Конфигурационные структуры

type Config struct {
	Server     ServerConfig   `yaml:"server"`
	Checks     CheckConfig    `yaml:"checks"`
	HTTPClient HTTPConfig     `yaml:"http_client"`
	Streams    []StreamConfig `yaml:"streams"`
}

type ServerConfig struct {
	Port        int    `yaml:"port"`
	MetricsPath string `yaml:"metrics_path"`
	HealthPath  string `yaml:"health_path"`
}

type CheckConfig struct {
	Workers       int           `yaml:"workers"`
	RetryAttempts int           `yaml:"retry_attempts"`
	RetryDelay    time.Duration `yaml:"retry_delay"`
	SegmentSample int           `yaml:"segment_sample"`
}

type HTTPConfig struct {
	Timeout      time.Duration `yaml:"timeout"`
	KeepAlive    bool          `yaml:"keep_alive"`
	MaxIdleConns int           `yaml:"max_idle_conns"`
	TLSVerify    bool          `yaml:"tls_verify"`
	UserAgent    string        `yaml:"user_agent"`
}

type StreamConfig struct {
	Name            string           `yaml:"name"`
	URL             string           `yaml:"url"`
	CheckMode       string           `yaml:"check_mode"`
	Interval        time.Duration    `yaml:"interval"`
	Timeout         time.Duration    `yaml:"timeout"`
	ValidateContent bool             `yaml:"validate_content"`
	MediaValidation *MediaValidation `yaml:"media_validation,omitempty"`
}

type MediaValidation struct {
	ContainerType  []string `yaml:"container_type"`
	MinSegmentSize int64    `yaml:"min_segment_size"`
	CheckAudio     bool     `yaml:"check_audio"`
	CheckVideo     bool     `yaml:"check_video"`
}

// Структуры результатов

type CheckResult struct {
	Success      bool
	StreamStatus StreamStatus
	Segments     SegmentResults
	Duration     time.Duration
	Timestamp    time.Time
	Error        *CheckError
}

type StreamStatus struct {
	IsLive        bool
	VariantsCount int
	SegmentsCount int
	TotalDuration float64
	LastModified  time.Time
}

type SegmentResults struct {
	Checked int
	Failed  int
	Details []SegmentCheck
}

type SegmentCheck struct {
	URL      string
	Success  bool
	Duration time.Duration
	Error    *CheckError
}

type SegmentData struct {
	URI       string
	Duration  float64
	Size      int64
	MediaInfo MediaInfo
	Headers   http.Header
}

type MediaInfo struct {
	Container  string // TS/fMP4
	Bitrate    int
	HasVideo   bool
	HasAudio   bool
	IsComplete bool
}

// Структуры ответов

type PlaylistResponse struct {
	Body       []byte
	StatusCode int
	Headers    http.Header
	Duration   time.Duration
}

type SegmentResponse struct {
	MediaInfo  MediaInfo
	StatusCode int
	Size       int64
	Duration   time.Duration
}

// Структуры ошибок

type CheckError struct {
	Type       ErrorType
	Message    string
	StatusCode int
	Retryable  bool
}

type ErrorType string

const (
	ErrPlaylistDownload ErrorType = "playlist_download"
	ErrPlaylistParse    ErrorType = "playlist_parse"
	ErrSegmentDownload  ErrorType = "segment_download"
	ErrSegmentValidate  ErrorType = "segment_validate"
	ErrMediaContainer   ErrorType = "media_container"
)

type ValidationError struct {
	Type    ValidationType
	Message string
	Details map[string]interface{}
}

type ValidationType string

const (
	// Базовая валидация
	ErrSegmentSize   ValidationType = "segment_size"
	ErrSegmentStatus ValidationType = "segment_status"
	// Медиа валидация
	ErrContainer ValidationType = "container_type"
	ErrNoVideo   ValidationType = "no_video"
	ErrNoAudio   ValidationType = "no_audio"
	ErrCorrupted ValidationType = "corrupted_media"
)

// Константы для режимов проверки
const (
	CheckModeAll       = "all"
	CheckModeFirstLast = "first_last"
	CheckModeRandom    = "random"
)
