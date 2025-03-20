# План разработки hls_exporter

## 1. Подготовка проекта

### 1.1 Структура проекта
```bash
hls_exporter/
├── cmd/
│   └── hls_exporter/
│       └── main.go
├── internal/
│   ├── config/
│   │   └── config.go
│   ├── checker/
│   │   ├── checker.go
│   │   └── validator.go
│   ├── http/
│   │   └── client.go
│   └── metrics/
│       └── collector.go
├── pkg/
│   └── models/
│       └── types.go
├── Dockerfile
├── go.mod
└── README.md
```

### 1.2 Основные зависимости
```go
require (
    github.com/prometheus/client_golang v1.x.x
    github.com/grafov/m3u8 v0.x.x
    github.com/spf13/viper
    go.uber.org/zap
)
```

## 2. Последовательность разработки

### 2.1 Базовые типы и интерфейсы (pkg/models)
```go
// 1. Определить основные структуры
type StreamConfig struct {...}
type CheckResult struct {...}
type SegmentData struct {...}

// 2. Определить интерфейсы
type Checker interface {...}
type Validator interface {...}
type HTTPClient interface {...}
```

### 2.2 Конфигурация (internal/config)
```go
// 1. Структуры конфигурации
// 2. Загрузка из YAML
// 3. Валидация параметров
```

### 2.3 HTTP клиент (internal/http)
```go
// 1. Реализация базового клиента
// 2. Добавление retry логики
// 3. Настройка таймаутов и пулов
```

### 2.4 Валидатор (internal/checker)
```go
// 1. Базовая валидация плейлистов
// 2. Парсинг m3u8
// 3. Валидация сегментов
// 4. Опциональная проверка медиаконтейнера
```

### 2.5 Метрики (internal/metrics)
```go
// 1. Определение метрик
// 2. Реализация коллектора
// 3. Регистрация в Prometheus
```

### 2.6 Основной чекер (internal/checker)
```go
// 1. Реализация воркер-пула
// 2. Логика проверки потоков
// 3. Обработка ошибок
```

### 2.7 Main пакет (cmd/hls_exporter)
```go
// 1. Инициализация компонентов
// 2. HTTP сервер
// 3. Graceful shutdown
```

## 3. Этапы тестирования

### 3.1 Unit тесты
```go
// 1. Тесты конфигурации
// 2. Тесты валидатора
// 3. Тесты HTTP клиента
// 4. Тесты метрик
```

### 3.2 Интеграционные тесты
```go
// 1. Тест полного цикла проверки
// 2. Тест с mock HTTP сервером
// 3. Тест метрик
```

## 4. Последовательность реализации

1. **День 1: Базовая структура**
   - Создание проекта
   - Базовые интерфейсы
   - Конфигурация

2. **День 2: HTTP и Валидация**
   - HTTP клиент
   - Парсинг m3u8
   - Базовая валидация

3. **День 3: Чекер и Метрики**
   - Воркер пул
   - Логика проверок
   - Prometheus метрики

4. **День 4: Тесты и Документация**
   - Unit тесты
   - Интеграционные тесты
   - README и примеры

5. **День 5: Финализация**
   - Dockerfile
   - Примеры конфигурации
   - CI/CD

## 5. Ключевые моменты реализации

### 5.1 Управление ресурсами
```go
// Контроль горутин
type WorkerPool struct {
    workers  int
    jobs     chan CheckJob
    stop     chan struct{}
}

// Graceful shutdown
func (c *Checker) Stop() error {
    close(c.stop)
    // Ожидание завершения
    return c.wg.Wait()
}
```

### 5.2 Обработка ошибок
```go
// Централизованная обработка
func (c *Checker) handleError(err error, stream string) {
    c.metrics.IncErrors(stream, err.Type())
    c.logger.Error("check failed",
        zap.String("stream", stream),
        zap.Error(err))
}
```

### 5.3 Метрики
```go
// Атомарное обновление
func (c *Checker) updateMetrics(result *CheckResult) {
    c.metrics.SetStreamUp(result.Stream, result.Success)
    c.metrics.RecordResponseTime(result.Stream, result.Duration)
}
```

## 6. Критерии готовности

1. **Функциональность**
   - Все типы проверок работают
   - Метрики корректно обновляются
   - Конфигурация валидируется

2. **Надежность**
   - Graceful shutdown
   - Корректная обработка ошибок
   - Отсутствие утечек памяти

3. **Тестирование**
   - Покрытие тестами >80%
   - Проходят нагрузочные тесты
   - Документированные тест-кейсы

4. **Документация**
   - README с примерами
   - Описание метрик
   - Примеры конфигурации

Этот план дает структурированный подход к разработке и позволяет последовательно реализовать все компоненты системы с учетом зависимостей между ними.
