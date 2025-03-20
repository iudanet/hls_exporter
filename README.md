# HLS Exporter

Prometheus exporter для мониторинга доступности HLS-потоков через HTTP/HTTPS протоколы.

## Возможности

- Мониторинг master/variant плейлистов
- Проверка доступности сегментов
- Опциональная валидация медиаконтейнеров
- Настраиваемые режимы проверки (all/first_last/random)
- Prometheus метрики с детальной статистикой
- Поддержка нескольких потоков с разными параметрами
- Graceful shutdown

## Установка

```bash
go install github.com/iudanet/hls_exporter/cmd/hls_exporter@latest
```

или сборка из исходников:

```bash
git clone https://github.com/iudanet/hls_exporter
cd hls_exporter
make build
```

## Конфигурация

Пример конфигурации (`config.yaml`):

```yaml
server:
  port: 9090
  metrics_path: "/metrics"
  health_path: "/health"

checks:
  workers: 5
  retry_attempts: 3
  retry_delay: "1s"
  segment_sample: 3  # для random режима

logging:
  level: "debug"  # debug, info, warn, error
  encoding: "json"  # json или console
  development: true  # включает режим разработки с более подробными

http_client:
  timeout: "5s"
  keep_alive: true
  max_idle_conns: 10
  tls_verify: true
  user_agent: "hls_exporter/1.0"

streams:
  - name: "stream_1"
    url: "https://example.com/master.m3u8"
    check_mode: "first_last"  # all, first_last, random
    interval: "30s"
    timeout: "10s"
    validate_content: false  # отключена проверка медиаконтейнера

  - name: "stream_2"
    url: "https://example.com/stream.m3u8"
    check_mode: "all"
    interval: "1m"
    timeout: "15s"
    validate_content: true   # включена проверка медиаконтейнера
    media_validation:        # настройки валидации медиа
      container_type: ["TS", "fMP4"]
      min_segment_size: 10240
      check_audio: true
      check_video: true
```

## Запуск

```bash
hls_exporter -config config.yaml
```

## Метрики

Основные метрики:

```
# Доступность HLS потока (1 = доступен, 0 = недоступен)
hls_stream_up{name="stream_1"} 1

# Время ответа в секундах
hls_response_time_seconds{name="stream_1",type="playlist"} 0.245

# Количество ошибок
hls_errors_total{name="stream_1",error_type="segment_download"} 2

# Количество проверенных сегментов
hls_segments_checked_total{name="stream_1",status="success"} 42

# Timestamp последней проверки
hls_last_check_timestamp{name="stream_1"} 1645372800
```

## Docker

```bash
docker run -p 9090:9090 -v $(pwd)/config.yaml:/config.yaml iudanet/hls_exporter:latest
```

## Мониторинг

Пример правила для Prometheus:

```yaml
groups:
- name: hls_alerts
  rules:
  - alert: HLSStreamDown
    expr: hls_stream_up == 0
    for: 5m
    labels:
      severity: critical
    annotations:
      summary: "HLS stream {{ $labels.name }} is down"
```

## Разработка

Требования:
- Go 1.20+
- Make

Команды:

```bash
make test      # Запуск тестов
make lint      # Проверка линтером
make build     # Сборка бинарного файла
make docker    # Сборка Docker образа
```

## Лицензия

MIT

## Автор

[Chudakov Alexander](https://github.com/iudanet)
