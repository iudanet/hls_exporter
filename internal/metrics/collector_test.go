package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	testStreamName = "test_stream"
)

// createTestCollector создает коллектор с собственным регистром для тестов
func createTestCollector() (*Collector, *prometheus.Registry) {
	reg := prometheus.NewRegistry()
	c := &Collector{
		streamUp: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: MetricStreamUp,
				Help: "Shows if the HLS stream is available",
			},
			[]string{"name"},
		),

		responseTime: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    MetricResponseTime,
				Help:    "Response time in seconds",
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

	reg.MustRegister(c.streamUp)
	reg.MustRegister(c.responseTime)
	reg.MustRegister(c.errorsTotal)
	reg.MustRegister(c.lastCheck)
	reg.MustRegister(c.segmentsChecked)

	return c, reg
}

func TestCollector_StreamUp(t *testing.T) {
	c, _ := createTestCollector()

	tests := []struct {
		name     string
		streamUp bool
		want     float64
	}{
		{
			name:     "stream1",
			streamUp: true,
			want:     1.0,
		},
		{
			name:     "stream2",
			streamUp: false,
			want:     0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c.SetStreamUp(tt.name, tt.streamUp)
			if got := c.GetStreamUp(tt.name); got != tt.want {
				t.Errorf("SetStreamUp() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCollector_ErrorsTotal(t *testing.T) {
	c, _ := createTestCollector()

	tests := []struct {
		name      string
		errorType string
		count     int
		want      float64
	}{
		{
			name:      "stream1",
			errorType: "download",
			count:     2,
			want:      2.0,
		},
		{
			name:      "stream2",
			errorType: "parse",
			count:     1,
			want:      1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := 0; i < tt.count; i++ {
				c.RecordError(tt.name, tt.errorType)
			}
			if got := c.GetErrorsTotal(tt.name, tt.errorType); got != tt.want {
				t.Errorf("RecordError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCollector_SegmentCheck(t *testing.T) {
	c, reg := createTestCollector()
	streamName := testStreamName

	successCases := 3
	failureCases := 2

	for i := 0; i < successCases; i++ {
		c.RecordSegmentCheck(streamName, true)
	}
	for i := 0; i < failureCases; i++ {
		c.RecordSegmentCheck(streamName, false)
	}

	metrics, err := reg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	var found bool
	for _, m := range metrics {
		if m.GetName() == MetricSegmentsChecked {
			found = true
			break
		}
	}
	if !found {
		t.Error("Segments checked metric not registered")
	}
}

func TestCollector_ResponseTime(t *testing.T) {
	c, reg := createTestCollector()
	streamName := testStreamName

	times := []float64{0.1, 0.5, 2.0, 5.0}
	for _, duration := range times {
		c.RecordResponseTime(streamName, duration)
	}

	metrics, err := reg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	var found bool
	for _, m := range metrics {
		if m.GetName() == MetricResponseTime {
			found = true
			break
		}
	}
	if !found {
		t.Error("Response time metric not registered")
	}
}

func TestCollector_LastCheckTime(t *testing.T) {
	c, reg := createTestCollector()
	streamName := testStreamName

	now := time.Now()
	c.SetLastCheckTime(streamName, now)

	metrics, err := reg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	var found bool
	for _, m := range metrics {
		if m.GetName() == MetricLastCheck {
			found = true
			break
		}
	}
	if !found {
		t.Error("Last check time metric not registered")
	}
}

func TestCollector_Reset(t *testing.T) {
	c, _ := createTestCollector()
	streamName := testStreamName

	c.SetStreamUp(streamName, true)
	c.RecordError(streamName, "download")
	c.RecordSegmentCheck(streamName, true)

	c.Reset(streamName)

	if got := c.GetStreamUp(streamName); got != 0 {
		t.Errorf("After Reset() stream up = %v, want 0", got)
	}
}

func TestCollector_NewCollector(t *testing.T) {
	c := NewCollector()
	if c == nil {
		t.Error("NewCollector() returned nil")
	}
}
