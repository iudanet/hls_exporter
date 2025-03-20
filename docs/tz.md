# Техническое задание на разработку hls_exporter

## 1. Общие положения

### 1.1 Назначение
Prometheus exporter для мониторинга доступности HLS-потоков через HTTP/HTTPS протоколы.

### 1.2 Технологический стек
- Go 1.20+
- github.com/prometheus/client_golang
- gorilla/mux (опционально, для HTTP роутинга)

## 2. Архитектура приложения

### 2.1 Основные компоненты
```go
type HLSExporter struct {
    checker   *StreamChecker
    metrics   *MetricsCollector
    config    *Config
    workers   *WorkerPool
}

type StreamChecker struct {
    client    *http.Client
    validator *HLSValidator
}

type MetricsCollector struct {
    streamUp          *prometheus.GaugeVec
    responseTime      *prometheus.HistogramVec
    errorsTotal      *prometheus.CounterVec
    lastCheckTime    *prometheus.GaugeVec
    activeChecks     prometheus.Gauge
}
```

### 2.2 Конфигурация
```yaml
server:
  port: 9090
  metrics_path: "/metrics"

checks:
  interval: 30s
  timeout: 10s
  workers: 5

streams:
  - name: "stream1"
    url: "https://example.com/master.m3u8"
    check_mode: "first_last"  # all, first_last, random

  - name: "stream2"
    url: "https://example.com/stream.m3u8"
    check_mode: "all"

http_client:
  timeout: 5s
  keep_alive: true
  max_idle_conns: 10
  tls_verify: true
```

## 3. Функциональные требования

### 3.1 Проверка потоков
```go
type StreamChecker interface {
    // Основной метод проверки потока
    Check(ctx context.Context, stream StreamConfig) (*CheckResult, error)

    // Остановка всех проверок
    Stop() error
}

type CheckResult struct {
    Success      bool
    Error        error
    ResponseTime time.Duration
    CheckedAt    time.Time
}
```

### 3.2 Метрики Prometheus
```go
// Основные метрики
const (
    MetricStreamUp          = "hls_stream_up"
    MetricResponseTime      = "hls_response_time_seconds"
    MetricErrorsTotal       = "hls_errors_total"
    MetricLastCheck         = "hls_last_check_timestamp"
    MetricActiveChecks      = "hls_active_checks"
)

// Лейблы
const (
    LabelStream     = "stream"
    LabelErrorType  = "error_type"
)
```

## 4. HTTP Endpoints

### 4.1 Основные endpoints
```go
// GET /metrics - Prometheus метрики
// GET /health - Health check
// GET /-/reload - Перезагрузка конфигурации (опционально)
```

## 5. Компоненты системы

### 5.1 HTTP Client
```go
type HTTPClient struct {
    client    *http.Client
    config    HTTPConfig
}

type HTTPConfig struct {
    Timeout      time.Duration
    KeepAlive    bool
    MaxIdleConns int
    TLSVerify    bool
}
```

### 5.2 HLS Validator
```go
type HLSValidator struct {
    // Валидация master playlist
    ValidateMaster(playlist *m3u8.MasterPlaylist) error

    // Валидация media playlist
    ValidateMedia(playlist *m3u8.MediaPlaylist) error
}
```

### 5.3 Worker Pool
```go
type WorkerPool struct {
    workers  int
    jobs     chan CheckJob
    results  chan CheckResult
    stop     chan struct{}
}

type CheckJob struct {
    Stream  StreamConfig
    Context context.Context
}
```

## 6. Обработка ошибок

### 6.1 Типы ошибок
```go
const (
    ErrPlaylistParse   = "playlist_parse_error"
    ErrHTTPStatus      = "http_status_error"
    ErrTimeout         = "timeout_error"
    ErrValidation      = "validation_error"
    ErrNetwork         = "network_error"
)
```

## 7. Логирование

### 7.1 Формат логов
```go
type LogEntry struct {
    Time     time.Time `json:"time"`
    Level    string    `json:"level"`
    Message  string    `json:"message"`
    Stream   string    `json:"stream,omitempty"`
    Error    string    `json:"error,omitempty"`
}
```

## 8. Тесты

### 8.1 Обязательные тесты
```go
// unit tests
func TestStreamChecker_Check(t *testing.T)
func TestHLSValidator_ValidatePlaylist(t *testing.T)
func TestMetricsCollector_Record(t *testing.T)

// integration tests
func TestExporter_Integration(t *testing.T)
```

## 9. Структура проекта
```
/cmd
    /hls_exporter
        main.go
/internal
    /checker
        checker.go
        validator.go
    /config
        config.go
    /metrics
        collector.go
    /http
        client.go
    /worker
        pool.go
/pkg
    /models
        types.go
```

## 10. Требования к реализации

### 10.1 Производительность
- Эффективное использование горутин
- Переиспользование HTTP соединений
- Минимизация аллокаций памяти

### 10.2 Отказоустойчивость
- Graceful shutdown
- Корректная обработка context
- Retry механизм для HTTP запросов

### 10.3 Мониторинг
- Prometheus метрики
- Health check endpoint
- Логирование критических ошибок

## 11. Результаты

### 11.1 Ожидаемые артефакты
- Исходный код
- Dockerfile
- README.md с описанием и примерами
- Конфигурационные файлы

### 11.2 Критерии приемки
- Успешное прохождение тестов
- Покрытие кода тестами >80%
- Корректная работа метрик
- Стабильная работа под нагрузкой

## 12. Сборка и запуск

### 12.1 Локальная сборка
```bash
go build -o hls_exporter cmd/hls_exporter/main.go
```

### 12.2 Docker
```dockerfile
FROM golang:1.20-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o hls_exporter cmd/hls_exporter/main.go

FROM alpine:3.14
COPY --from=builder /app/hls_exporter /usr/local/bin/
EXPOSE 9090
ENTRYPOINT ["hls_exporter"]
```

## 13. Дополнительные требования

### 13.1 Безопасность
- Validate входящих URL
- Ограничение максимального размера ответа
- Таймауты для всех операций

### 13.2 Конфигурация
- Поддержка переменных окружения
- Валидация конфигурации при старте
- Логирование изменений конфигурации

нужно добавить валидацию сегментов

ошпиши только публичные интерфейсы в качестве примеров. так же должна быть описана вся бизнеслогика в начале.

# Спецификация публичных интерфейсов hls_exporter

## 1. Бизнес-логика проверки HLS потока

### 1.1 Последовательность проверки
1. Загрузка master playlist
2. Парсинг и валидация master playlist
3. Для каждого варианта:
   - Загрузка variant playlist
   - Парсинг и валидация variant playlist
   - Проверка сегментов согласно режиму (all/first_last/random)
   - Валидация медиа-контента сегментов
4. Обновление метрик

### 1.2 Режимы проверки сегментов
- `all`: проверка всех сегментов
- `first_last`: только первый и последний сегмент
- `random`: случайная выборка N сегментов

### 1.3 Правила валидации
1. Master playlist:
   - Наличие обязательных тегов
   - Корректность URI
   - Минимум один вариант

2. Variant playlist:
   - Последовательность сегментов
   - Актуальность (для live)
   - Длительность сегментов

3. Сегменты:
   - HTTP статус
   - Content-Type
   - Размер файла
   - Валидация медиа-контейнера
   - Проверка битрейта

## 2. Основные интерфейсы

### 2.1 Checker Interface
```go
type Checker interface {
    // Основной метод проверки
    Check(ctx context.Context, stream StreamConfig) (*CheckResult, error)

    // Управление жизненным циклом
    Start() error
    Stop() error
}

type CheckResult struct {
    Success      bool
    StreamStatus StreamStatus
    Segments     SegmentResults
    Duration     time.Duration
    Timestamp    time.Time
    Error        *CheckError
}

type StreamStatus struct {
    IsLive          bool
    VariantsCount   int
    SegmentsCount   int
    TotalDuration   float64
    LastModified    time.Time
}

type SegmentResults struct {
    Checked     int
    Failed      int
    Details     []SegmentCheck
}
```

### 2.2 Validator Interface
```go
type Validator interface {
    // Валидация Master Playlist
    ValidateMaster(playlist *m3u8.MasterPlaylist) error

    // Валидация Media Playlist
    ValidateMedia(playlist *m3u8.MediaPlaylist) error

    // Валидация сегмента
    ValidateSegment(segment *SegmentData) error
}

type SegmentData struct {
    URI         string
    Duration    float64
    Size        int64
    MediaInfo   MediaInfo
    Headers     http.Header
}

type MediaInfo struct {
    Container   string  // TS/fMP4
    Bitrate     int
    HasVideo    bool
    HasAudio    bool
    IsComplete  bool
}
```

### 2.3 HTTP Client Interface
```go
type HTTPClient interface {
    // Загрузка плейлиста
    GetPlaylist(ctx context.Context, url string) (*PlaylistResponse, error)

    // Загрузка и валидация сегмента
    GetSegment(ctx context.Context, url string, validate bool) (*SegmentResponse, error)

    // Конфигурация клиента
    SetTimeout(timeout time.Duration)
    Close() error
}

type PlaylistResponse struct {
    Body        []byte
    StatusCode  int
    Headers     http.Header
    Duration    time.Duration
}

type SegmentResponse struct {
    MediaInfo   MediaInfo
    StatusCode  int
    Size        int64
    Duration    time.Duration
}
```

### 2.4 Metrics Interface
```go
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
```

### 2.5 Configuration Types
```go
type Config struct {
    Streams     []StreamConfig
    HTTP        HTTPConfig
    Checks      CheckConfig
    Metrics     MetricsConfig
}

type StreamConfig struct {
    Name            string
    URL             string
    CheckMode       string  // all/first_last/random
    Interval        time.Duration
    Timeout         time.Duration
    ValidateContent bool
}

type CheckConfig struct {
    Workers         int
    RetryAttempts   int
    RetryDelay      time.Duration
    SegmentSample   int     // для random mode
}
```

### 2.6 Error Types
```go
type CheckError struct {
    Type        ErrorType
    Message     string
    StatusCode  int
    Retryable   bool
}

type ErrorType string

const (
    ErrPlaylistDownload  ErrorType = "playlist_download"
    ErrPlaylistParse     ErrorType = "playlist_parse"
    ErrSegmentDownload   ErrorType = "segment_download"
    ErrSegmentValidate   ErrorType = "segment_validate"
    ErrMediaContainer    ErrorType = "media_container"
)
```

## 3. Пример использования

```go
func main() {
    // Инициализация
    cfg := LoadConfig()
    metrics := NewMetricsCollector()
    validator := NewHLSValidator()
    client := NewHTTPClient(cfg.HTTP)

    checker := NewStreamChecker(client, validator, metrics)

    // Запуск проверок
    if err := checker.Start(); err != nil {
        log.Fatal(err)
    }

    // HTTP сервер для метрик
    http.Handle("/metrics", promhttp.Handler())
    http.ListenAndServe(":9090", nil)
}
```

Эта спецификация описывает все ключевые интерфейсы для реализации системы проверки HLS потоков с фокусом на валидацию сегментов и сбор метрик. Все компоненты слабо связаны и могут быть протестированы независимо.

 отлично добавь пример сонфигурации и прверку медиаконтенера сделать опциональной

# Дополнение к спецификации hls_exporter

## 1. Расширенная конфигурация

### 1.1 Пример конфигурации (config.yaml)
```yaml
server:
  port: 9090
  metrics_path: "/metrics"
  health_path: "/health"

checks:
  workers: 5
  retry_attempts: 3
  retry_delay: "1s"
  segment_sample: 3  # количество сегментов для random режима

http_client:
  timeout: "5s"
  keep_alive: true
  max_idle_conns: 10
  tls_verify: true
  user_agent: "hls_exporter/1.0"

streams:
  - name: "stream_1"
    url: "https://example.com/master.m3u8"
    check_mode: "first_last"
    interval: "30s"
    timeout: "10s"
    validate_content: false  # отключена проверка медиаконтейнера

  - name: "stream_2"
    url: "https://example.com/stream.m3u8"
    check_mode: "all"
    interval: "1m"
    timeout: "15s"
    validate_content: true   # включена проверка медиаконтейнера
    media_validation:        # опциональные настройки валидации медиа
      container_type: ["TS", "fMP4"]
      min_segment_size: 10240
      check_audio: true
      check_video: true

  - name: "stream_3"
    url: "https://example.com/backup.m3u8"
    check_mode: "random"
    interval: "45s"
    timeout: "10s"
    validate_content: true
    media_validation:
      container_type: ["TS"]
      min_segment_size: 1024
      check_audio: false     # проверяем только наличие видео
      check_video: true
```

### 1.2 Обновленные типы конфигурации
```go
type Config struct {
    Server      ServerConfig     `yaml:"server"`
    Checks      CheckConfig      `yaml:"checks"`
    HTTPClient  HTTPConfig       `yaml:"http_client"`
    Streams     []StreamConfig   `yaml:"streams"`
}

type StreamConfig struct {
    Name             string           `yaml:"name"`
    URL              string           `yaml:"url"`
    CheckMode        string           `yaml:"check_mode"`
    Interval         Duration         `yaml:"interval"`
    Timeout          Duration         `yaml:"timeout"`
    ValidateContent  bool             `yaml:"validate_content"`
    MediaValidation  *MediaValidation `yaml:"media_validation,omitempty"`
}

type MediaValidation struct {
    ContainerType   []string `yaml:"container_type"`
    MinSegmentSize  int64    `yaml:"min_segment_size"`
    CheckAudio      bool     `yaml:"check_audio"`
    CheckVideo      bool     `yaml:"check_video"`
}
```

### 1.3 Обновленный интерфейс валидатора
```go
type Validator interface {
    // Валидация Master Playlist
    ValidateMaster(playlist *m3u8.MasterPlaylist) error

    // Валидация Media Playlist
    ValidateMedia(playlist *m3u8.MediaPlaylist) error

    // Валидация сегмента с опциональной проверкой медиаконтейнера
    ValidateSegment(segment *SegmentData, validation *MediaValidation) error
}

type SegmentValidator struct {
    // Базовая валидация (всегда выполняется)
    validateBasic(segment *SegmentData) error

    // Опциональная валидация медиаконтейнера
    validateMedia(segment *SegmentData, validation *MediaValidation) error
}
```

### 1.4 Пример метрик
```go
// Основные метрики
const (
    MetricStreamUp          = "hls_stream_up"
    MetricResponseTime      = "hls_response_time_seconds"
    MetricErrorsTotal       = "hls_errors_total"
    MetricLastCheck         = "hls_last_check_timestamp"
    MetricSegmentsChecked   = "hls_segments_checked_total"
    MetricSegmentSize       = "hls_segment_size_bytes"
)

// Пример вывода метрик
```
```text
# HELP hls_stream_up Показывает доступность HLS потока (1 = доступен, 0 = недоступен)
hls_stream_up{name="stream_1"} 1

# HELP hls_response_time_seconds Время ответа в секундах
hls_response_time_seconds{name="stream_1",type="playlist"} 0.245
hls_response_time_seconds{name="stream_1",type="segment"} 0.876

# HELP hls_segments_checked_total Количество проверенных сегментов
hls_segments_checked_total{name="stream_1",status="success"} 42
hls_segments_checked_total{name="stream_1",status="failed"} 0

# HELP hls_segment_size_bytes Размер сегментов в байтах
hls_segment_size_bytes{name="stream_1",quantile="0.5"} 2097152
hls_segment_size_bytes{name="stream_1",quantile="0.9"} 2359296
```

### 1.5 Пример использования валидатора
```go
func (c *Checker) checkSegment(ctx context.Context, url string, cfg *StreamConfig) error {
    segment, err := c.client.GetSegment(ctx, url, cfg.ValidateContent)
    if err != nil {
        return err
    }

    // Базовая валидация всегда
    if err := c.validator.validateBasic(segment); err != nil {
        return err
    }

    // Опциональная валидация медиаконтейнера
    if cfg.ValidateContent && cfg.MediaValidation != nil {
        if err := c.validator.validateMedia(segment, cfg.MediaValidation); err != nil {
            return err
        }
    }

    return nil
}
```

### 1.6 Обработка ошибок валидации
```go
type ValidationError struct {
    Type        ValidationType
    Message     string
    Details     map[string]interface{}
}

type ValidationType string

const (
    // Базовая валидация
    ErrSegmentSize    ValidationType = "segment_size"
    ErrSegmentStatus  ValidationType = "segment_status"

    // Медиа валидация
    ErrContainer      ValidationType = "container_type"
    ErrNoVideo        ValidationType = "no_video"
    ErrNoAudio        ValidationType = "no_audio"
    ErrCorrupted      ValidationType = "corrupted_media"
)
```

Эти дополнения позволяют:
1. Гибко настраивать проверку для каждого потока
2. Оптимизировать производительность, отключая ненужную валидацию
3. Получать детальные метрики по каждому типу проверки
4. Разделять ошибки базовой и медиа-валидации



нам не требуетя метрика
   MetricSegmentSize       = "hls_segment_size_bytes"
   так как я не понимаю как берется квантиль если оно хранит сколько то результатов и выдает их и высчитывает  quantile то окей. но нужно тогда ограничить хранение результатов так как память будет утикать

Правильное замечание. Давайте упростим метрики и оставим только необходимые, которые не требуют хранения исторических данных.

```go
// Основные метрики
const (
    // Gauge: 1 = доступен, 0 = недоступен
    MetricStreamUp = "hls_stream_up"

    // Histogram: время ответа с бакетами
    MetricResponseTime = "hls_response_time_seconds"

    // Counter: общее количество ошибок
    MetricErrorsTotal = "hls_errors_total"

    // Gauge: timestamp последней проверки
    MetricLastCheck = "hls_last_check_timestamp"

    // Counter: количество проверенных сегментов
    MetricSegmentsChecked = "hls_segments_checked_total"
)

// Метрики создаются один раз при старте
type MetricsCollector struct {
    streamUp *prometheus.GaugeVec
    responseTime *prometheus.HistogramVec
    errorsTotal *prometheus.CounterVec
    lastCheck *prometheus.GaugeVec
    segmentsChecked *prometheus.CounterVec
}

func NewMetricsCollector() *MetricsCollector {
    return &MetricsCollector{
        streamUp: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: MetricStreamUp,
                Help: "Shows if the HLS stream is available",
            },
            []string{"name"},
        ),

        responseTime: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name: MetricResponseTime,
                Help: "Response time in seconds",
                // Бакеты оптимизированы под типичные значения
                Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10},
            },
            []string{"name", "type"},
        ),

        errorsTotal: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: MetricErrorsTotal,
                Help: "Total number of errors",
            },
            []string{"name", "error_type"},
        ),

        lastCheck: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: MetricLastCheck,
                Help: "Timestamp of last check",
            },
            []string{"name"},
        ),

        segmentsChecked: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: MetricSegmentsChecked,
                Help: "Number of segments checked",
            },
            []string{"name", "status"},
        ),
    }
}

// Пример вывода метрик
```
```text
# HELP hls_stream_up Shows if the HLS stream is available
hls_stream_up{name="stream_1"} 1

# HELP hls_response_time_seconds Response time in seconds
hls_response_time_seconds_bucket{name="stream_1",type="playlist",le="0.1"} 10
hls_response_time_seconds_bucket{name="stream_1",type="playlist",le="0.25"} 25
hls_response_time_seconds_bucket{name="stream_1",type="playlist",le="0.5"} 45
...
hls_response_time_seconds_count{name="stream_1",type="playlist"} 100
hls_response_time_seconds_sum{name="stream_1",type="playlist"} 42.3

# HELP hls_errors_total Total number of errors
hls_errors_total{name="stream_1",error_type="playlist_download"} 2

# HELP hls_last_check_timestamp Timestamp of last check
hls_last_check_timestamp{name="stream_1"} 1645372800

# HELP hls_segments_checked_total Number of segments checked
hls_segments_checked_total{name="stream_1",status="success"} 42
hls_segments_checked_total{name="stream_1",status="failed"} 1
```

Преимущества такого подхода:
1. Нет хранения исторических данных в памяти
2. Метрики оптимизированы под Prometheus
3. Чёткое разделение типов метрик (Gauge/Counter/Histogram)
4. Минимальная кардинальность лейблов

Метрики позволяют отслеживать:
- Доступность потока
- Время ответа с распределением
- Количество и типы ошибок
- Актуальность данных (timestamp)
- Общую статистику проверок сегментов

При этом не требуется дополнительной памяти для хранения промежуточных значений.
