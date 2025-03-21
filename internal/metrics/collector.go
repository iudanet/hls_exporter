package metrics

import (
	"time"

	"github.com/iudanet/hls_exporter/pkg/models"
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
	streamBitrate   *prometheus.GaugeVec // Добавляем
	segmentsCount   *prometheus.GaugeVec // Добавляем
	activeChecks    prometheus.Gauge     // Добавляем
}

var _ models.MetricsCollector = (*Collector)(nil)

// NewCollector создает и регистрирует все метрики
func NewCollector(reg prometheus.Registerer) models.MetricsCollector {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}

	factory := promauto.With(reg)

	c := &Collector{
		streamUp: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: MetricStreamUp,
				Help: "Shows if the HLS stream is available",
			},
			[]string{"name"},
		),

		responseTime: factory.NewHistogramVec( // Заменили promauto на factory
			prometheus.HistogramOpts{
				Name:    MetricResponseTime,
				Help:    "Response time in seconds",
				Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			},
			[]string{"name", "type"},
		),

		errorsTotal: factory.NewCounterVec( // Заменили promauto на factory
			prometheus.CounterOpts{
				Name: MetricErrorsTotal,
				Help: "Total number of errors",
			},
			[]string{"name", "error_type"},
		),

		lastCheck: factory.NewGaugeVec( // Заменили promauto на factory
			prometheus.GaugeOpts{
				Name: MetricLastCheck,
				Help: "Timestamp of last check",
			},
			[]string{"name"},
		),

		segmentsChecked: factory.NewCounterVec( // Заменили promauto на factory
			prometheus.CounterOpts{
				Name: MetricSegmentsChecked,
				Help: "Number of segments checked",
			},
			[]string{"name", "status"},
		),

		streamBitrate: factory.NewGaugeVec( // Заменили promauto на factory
			prometheus.GaugeOpts{
				Name: namespace + "_stream_bitrate_bytes",
				Help: "Stream bitrate in bytes per second",
			},
			[]string{"name"},
		),

		segmentsCount: factory.NewGaugeVec( // Заменили promauto на factory
			prometheus.GaugeOpts{
				Name: namespace + "_segments_count",
				Help: "Number of segments in playlist",
			},
			[]string{"name"},
		),

		activeChecks: factory.NewGauge( // Заменили promauto на factory
			prometheus.GaugeOpts{
				Name: namespace + "_active_checks",
				Help: "Number of active checks",
			},
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
func (c *Collector) RecordResponseTime(name string, duration float64) {
	c.responseTime.WithLabelValues(name, "total").Observe(duration)
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

func (c *Collector) SetStreamBitrate(name string, bitrate float64) {
	c.streamBitrate.WithLabelValues(name).Set(bitrate)
}

func (c *Collector) SetSegmentsCount(name string, count int) {
	c.segmentsCount.WithLabelValues(name).Set(float64(count))
}

func (c *Collector) SetActiveChecks(count int) {
	c.activeChecks.Set(float64(count))
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
