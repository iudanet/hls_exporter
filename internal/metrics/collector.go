package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	dto "github.com/prometheus/client_model/go"
)

const (
	namespace = "hls"

	// Метрики
	MetricStreamUp        = namespace + "_stream_up"
	MetricResponseTime    = namespace + "_response_time_seconds"
	MetricErrorsTotal     = namespace + "_errors_total"
	MetricLastCheck       = namespace + "_last_check_timestamp"
	MetricSegmentsChecked = namespace + "_segments_checked_total"
)

// Collector реализует интерфейс MetricsCollector
type Collector struct {
	streamUp        *prometheus.GaugeVec
	responseTime    *prometheus.HistogramVec
	errorsTotal     *prometheus.CounterVec
	lastCheck       *prometheus.GaugeVec
	segmentsChecked *prometheus.CounterVec
}

// NewCollector создает и регистрирует все метрики
func NewCollector() *Collector {
	c := &Collector{
		streamUp: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: MetricStreamUp,
				Help: "Shows if the HLS stream is available",
			},
			[]string{"name"},
		),

		responseTime: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    MetricResponseTime,
				Help:    "Response time in seconds",
				Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			},
			[]string{"name", "type"},
		),

		errorsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: MetricErrorsTotal,
				Help: "Total number of errors",
			},
			[]string{"name", "error_type"},
		),

		lastCheck: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: MetricLastCheck,
				Help: "Timestamp of last check",
			},
			[]string{"name"},
		),

		segmentsChecked: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: MetricSegmentsChecked,
				Help: "Number of segments checked",
			},
			[]string{"name", "status"},
		),
	}

	return c
}

// SetStreamUp устанавливает доступность потока
func (c *Collector) SetStreamUp(name string, up bool) {
	value := 0.0
	if up {
		value = 1.0
	}
	c.streamUp.WithLabelValues(name).Set(value)
}

// RecordResponseTime записывает время ответа
func (c *Collector) RecordResponseTime(name string, duration float64, checkType string) {
	c.responseTime.WithLabelValues(name, checkType).Observe(duration)
}

// RecordError увеличивает счетчик ошибок
func (c *Collector) RecordError(name, errorType string) {
	c.errorsTotal.WithLabelValues(name, errorType).Inc()
}

// SetLastCheckTime устанавливает время последней проверки
func (c *Collector) SetLastCheckTime(name string, timestamp time.Time) {
	c.lastCheck.WithLabelValues(name).Set(float64(timestamp.Unix()))
}

// RecordSegmentCheck записывает результат проверки сегмента
func (c *Collector) RecordSegmentCheck(name string, success bool) {
	status := "success"
	if !success {
		status = "failed"
	}
	c.segmentsChecked.WithLabelValues(name, status).Inc()
}

// Reset сбрасывает все метрики для указанного потока
func (c *Collector) Reset(name string) {
	c.streamUp.DeleteLabelValues(name)
	// Для гистограмм и счетчиков сброс не требуется,
	// так как они автоматически очищаются Prometheus
}

// Close освобождает ресурсы (необязательно, так как promauto сам управляет регистрацией)
func (c *Collector) Close() error {
	return nil
}

// Вспомогательные функции для тестирования
func (c *Collector) GetStreamUp(name string) float64 {
	return getGaugeValue(c.streamUp.WithLabelValues(name))
}

func (c *Collector) GetErrorsTotal(name, errorType string) float64 {
	return getCounterValue(c.errorsTotal.WithLabelValues(name, errorType))
}

// Получение значения Gauge метрики
func getGaugeValue(gauge prometheus.Gauge) float64 {
	var metric dto.Metric
	if err := gauge.Write(&metric); err != nil {
		return 0
	}
	if metric.Gauge == nil {
		return 0
	}
	return metric.Gauge.GetValue()
}

// Получение значения Counter метрики
func getCounterValue(counter prometheus.Counter) float64 {
	var metric dto.Metric
	if err := counter.Write(&metric); err != nil {
		return 0
	}
	if metric.Counter == nil {
		return 0
	}
	return metric.Counter.GetValue()
}
