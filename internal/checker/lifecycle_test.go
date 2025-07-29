package checker

import (
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/iudanet/hls_exporter/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestStreamChecker_StartStop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := models.NewMockHTTPClient(ctrl)
	mockValidator := models.NewMockValidator(ctrl)
	mockMetrics := models.NewMockMetricsCollector(ctrl)

	// Ожидаем все возможные вызовы
	mockClient.EXPECT().SetTimeout(gomock.Any()).AnyTimes()
	mockClient.EXPECT().Close().Return(nil).AnyTimes()

	// Ожидаем любые вызовы метрик
	mockMetrics.EXPECT().RecordSegmentCheck(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().SetActiveChecks(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().SetStreamBitrate(gomock.Any(), gomock.Any()).AnyTimes()

	checker := NewStreamChecker(mockClient, mockValidator, mockMetrics, 2)

	assert.NoError(t, checker.Start())
	time.Sleep(50 * time.Millisecond)
	assert.NoError(t, checker.Stop())
}

func TestStreamChecker_DoubleStart(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := models.NewMockHTTPClient(ctrl)
	mockValidator := models.NewMockValidator(ctrl)
	mockMetrics := models.NewMockMetricsCollector(ctrl)

	// Ожидаем все возможные вызовы
	mockClient.EXPECT().SetTimeout(gomock.Any()).AnyTimes()
	mockClient.EXPECT().Close().Return(nil).AnyTimes()
	mockMetrics.EXPECT().RecordSegmentCheck(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().SetActiveChecks(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().SetStreamBitrate(gomock.Any(), gomock.Any()).AnyTimes()

	checker := NewStreamChecker(mockClient, mockValidator, mockMetrics, 1)

	assert.NoError(t, checker.Start())
	time.Sleep(10 * time.Millisecond)
	assert.NoError(t, checker.Stop())

	assert.NoError(t, checker.Start())
	time.Sleep(10 * time.Millisecond)
	assert.NoError(t, checker.Stop())
}
