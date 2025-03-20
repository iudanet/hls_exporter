package checker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestStreamChecker_Lifecycle(t *testing.T) {
	mockClient := new(MockHTTPClient)
	mockValidator := new(MockValidator)
	mockMetrics := new(MockMetricsCollector)

	// Добавляем ожидания для новых методов
	mockClient.On("SetTimeout", mock.Anything).Return()
	mockMetrics.On("RecordSegmentCheck", mock.Anything, mock.Anything).Return()
	mockMetrics.On("SetActiveChecks", mock.Anything).Return()
	mockMetrics.On("SetStreamBitrate", mock.Anything, mock.Anything).Return()
	mockClient.On("Close").Return(nil)
	tests := []struct {
		name        string
		workers     int
		startDelay  time.Duration
		stopDelay   time.Duration
		description string
	}{
		{
			name:        "normal shutdown",
			workers:     2,
			startDelay:  100 * time.Millisecond,
			stopDelay:   100 * time.Millisecond,
			description: "should start and stop normally",
		},
		{
			name:        "immediate shutdown",
			workers:     1,
			startDelay:  0,
			stopDelay:   0,
			description: "should handle immediate shutdown",
		},
		{
			name:        "multiple workers",
			workers:     5,
			startDelay:  50 * time.Millisecond,
			stopDelay:   50 * time.Millisecond,
			description: "should handle multiple workers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewStreamChecker(mockClient, mockValidator, mockMetrics, tt.workers)

			// Start
			err := checker.Start()
			assert.NoError(t, err, "Start should not return error")
			time.Sleep(tt.startDelay)

			// Stop
			err = checker.Stop()
			assert.NoError(t, err, "Stop should not return error")
			time.Sleep(tt.stopDelay)

			// Verify no goroutine leaks (this is approximate)
			// In real world, you might want to use more sophisticated leak detection
		})
	}
}

func TestStreamChecker_MultipleStartStop(t *testing.T) {
	mockClient := new(MockHTTPClient)
	mockValidator := new(MockValidator)
	mockMetrics := new(MockMetricsCollector)

	// Добавляем ожидания для новых методов
	mockClient.On("SetTimeout", mock.Anything).Return()
	mockMetrics.On("RecordSegmentCheck", mock.Anything, mock.Anything).Return()
	mockMetrics.On("SetActiveChecks", mock.Anything).Return()
	mockMetrics.On("SetStreamBitrate", mock.Anything, mock.Anything).Return()

	mockClient.On("Close").Return(nil)

	// Добавляем ожидание вызова Close
	mockClient.On("Close").Return(nil)

	checker := NewStreamChecker(mockClient, mockValidator, mockMetrics, 2)

	// First cycle
	assert.NoError(t, checker.Start(), "First start should succeed")
	time.Sleep(50 * time.Millisecond)
	assert.NoError(t, checker.Stop(), "First stop should succeed")

	// Second cycle
	assert.NoError(t, checker.Start(), "Second start should succeed")
	time.Sleep(50 * time.Millisecond)
	assert.NoError(t, checker.Stop(), "Second stop should succeed")
}
