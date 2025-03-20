package metrics

import (
	"testing"
	"time"

	"github.com/iudanet/hls_exporter/pkg/models"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
)

func TestCollector(t *testing.T) {
	// Создаем тестовый регистр для каждого теста
	t.Run("Metrics Registration", func(t *testing.T) {
		reg := prometheus.NewRegistry()
		collector := NewCollector(reg)
		assert.NotNil(t, collector)

		// Проверяем, что все метрики зарегистрированы
		metrics, err := reg.Gather()
		assert.NoError(t, err)
		assert.NotEmpty(t, metrics)
	})

	// Для каждого теста создаем новый регистр
	tests := []struct {
		name string
		test func(t *testing.T, reg *prometheus.Registry, collector models.MetricsCollector)
	}{
		{"SetStreamUp", testSetStreamUp},
		{"RecordError", testRecordError},
		{"SetLastCheckTime", testSetLastCheckTime},
		{"RecordSegmentCheck", testRecordSegmentCheck},
		{"RecordResponseTime", testRecordResponseTime},
		{"SetActiveChecks", testSetActiveChecks},
		{"SetSegmentsCount", testSetSegmentsCount},
		{"SetStreamBitrate", testSetStreamBitrate},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := prometheus.NewRegistry()
			collector := NewCollector(reg)
			tt.test(t, reg, collector)
		})
	}
}

func testSetStreamUp(t *testing.T, reg *prometheus.Registry, collector models.MetricsCollector) {
	collector.SetStreamUp("test_stream", true)
	value := collector.(*Collector).GetStreamUp("test_stream")
	assert.Equal(t, float64(1), value)

	collector.SetStreamUp("test_stream", false)
	value = collector.(*Collector).GetStreamUp("test_stream")
	assert.Equal(t, float64(0), value)
}
// Тест для RecordError
func testRecordError(t *testing.T, reg *prometheus.Registry, collector models.MetricsCollector) {
    collector.RecordError("test_stream", "test_error")
    value := collector.(*Collector).GetErrorsTotal("test_stream", "test_error")
    assert.Equal(t, float64(1), value)

    collector.RecordError("test_stream", "test_error")
    value = collector.(*Collector).GetErrorsTotal("test_stream", "test_error")
    assert.Equal(t, float64(2), value)
}

// Тест для SetLastCheckTime
func testSetLastCheckTime(t *testing.T, reg *prometheus.Registry, collector models.MetricsCollector) {
    now := time.Now()
    collector.SetLastCheckTime("test_stream", now)

    metrics, err := reg.Gather()
    assert.NoError(t, err)

    found := false
    for _, m := range metrics {
        if *m.Name == MetricLastCheck {
            for _, metric := range m.Metric {
                for _, label := range metric.Label {
                    if *label.Name == "name" && *label.Value == "test_stream" {
                        found = true
                        assert.Equal(t, float64(now.Unix()), *metric.Gauge.Value)
                    }
                }
            }
        }
    }
    assert.True(t, found, "LastCheck metric should be found")
}

// Тест для RecordSegmentCheck
func testRecordSegmentCheck(t *testing.T, reg *prometheus.Registry, collector models.MetricsCollector) {
    collector.RecordSegmentCheck("test_stream", true)
    collector.RecordSegmentCheck("test_stream", false)

    metrics, err := reg.Gather()
    assert.NoError(t, err)

    var successCount, failedCount float64
    for _, m := range metrics {
        if *m.Name == MetricSegmentsChecked {
            for _, metric := range m.Metric {
                for _, label := range metric.Label {
                    if *label.Name == "name" && *label.Value == "test_stream" {
                        if hasLabelValue(metric, "status", "success") {
                            successCount = *metric.Counter.Value
                        }
                        if hasLabelValue(metric, "status", "failed") {
                            failedCount = *metric.Counter.Value
                        }
                    }
                }
            }
        }
    }
    assert.Equal(t, float64(1), successCount, "Should have one successful check")
    assert.Equal(t, float64(1), failedCount, "Should have one failed check")
}

// Тест для RecordResponseTime
func testRecordResponseTime(t *testing.T, reg *prometheus.Registry, collector models.MetricsCollector) {
    collector.RecordResponseTime("test_stream", 0.5)

    metrics, err := reg.Gather()
    assert.NoError(t, err)

    found := false
    for _, m := range metrics {
        if *m.Name == MetricResponseTime {
            for _, metric := range m.Metric {
                if hasLabelValue(metric, "name", "test_stream") {
                    found = true
                    assert.Equal(t, uint64(1), *metric.Histogram.SampleCount)
                    assert.Equal(t, 0.5, *metric.Histogram.SampleSum)
                }
            }
        }
    }
    assert.True(t, found, "ResponseTime metric should be found")
}

// Тест для SetActiveChecks
func testSetActiveChecks(t *testing.T, reg *prometheus.Registry, collector models.MetricsCollector) {
    collector.(*Collector).SetActiveChecks(5)

    metrics, err := reg.Gather()
    assert.NoError(t, err)

    found := false
    for _, m := range metrics {
        if *m.Name == namespace+"_active_checks" {
            found = true
            assert.Equal(t, float64(5), *m.Metric[0].Gauge.Value)
        }
    }
    assert.True(t, found, "ActiveChecks metric should be found")
}

// Тест для SetSegmentsCount
func testSetSegmentsCount(t *testing.T, reg *prometheus.Registry, collector models.MetricsCollector) {
    collector.(*Collector).SetSegmentsCount("test_stream", 10)

    metrics, err := reg.Gather()
    assert.NoError(t, err)

    found := false
    for _, m := range metrics {
        if *m.Name == namespace+"_segments_count" {
            for _, metric := range m.Metric {
                if hasLabelValue(metric, "name", "test_stream") {
                    found = true
                    assert.Equal(t, float64(10), *metric.Gauge.Value)
                }
            }
        }
    }
    assert.True(t, found, "SegmentsCount metric should be found")
}

// Тест для SetStreamBitrate
func testSetStreamBitrate(t *testing.T, reg *prometheus.Registry, collector models.MetricsCollector) {
    collector.(*Collector).SetStreamBitrate("test_stream", 1500000)

    metrics, err := reg.Gather()
    assert.NoError(t, err)

    found := false
    for _, m := range metrics {
        if *m.Name == namespace+"_stream_bitrate_bytes" {
            for _, metric := range m.Metric {
                if hasLabelValue(metric, "name", "test_stream") {
                    found = true
                    assert.Equal(t, float64(1500000), *metric.Gauge.Value)
                }
            }
        }
    }
    assert.True(t, found, "StreamBitrate metric should be found")
}

// Вспомогательная функция для проверки значения метки
func hasLabelValue(metric *dto.Metric, labelName, labelValue string) bool {
    for _, label := range metric.Label {
        if *label.Name == labelName && *label.Value == labelValue {
            return true
        }
    }
    return false
}
func TestNewCollectorWithNilRegistry(t *testing.T) {
	// Сохраняем оригинальный регистр
	origReg := prometheus.DefaultRegisterer
	// Создаем новый регистр для теста
	prometheus.DefaultRegisterer = prometheus.NewRegistry()
	// Восстанавливаем оригинальный регистр после теста
	defer func() {
		prometheus.DefaultRegisterer = origReg
	}()

	collector := NewCollector(nil)
	assert.NotNil(t, collector)
}
