package checker

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/iudanet/hls_exporter/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestStreamChecker_Check_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := models.NewMockHTTPClient(ctrl)
	mockValidator := models.NewMockValidator(ctrl)
	mockMetrics := models.NewMockMetricsCollector(ctrl)

	checker := NewStreamChecker(mockClient, mockValidator, mockMetrics, 1)

	// Настройка моков
	masterURL := "http://test.com/master.m3u8"
	variantURL := "http://test.com/stream.m3u8"
	segmentURL := "http://test.com/segment1.ts"

	mockClient.EXPECT().GetPlaylist(gomock.Any(), masterURL).
		Return(&models.PlaylistResponse{
			Body: []byte("#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1000000\nstream.m3u8"),
		}, nil)

	mockClient.EXPECT().GetPlaylist(gomock.Any(), variantURL).
		Return(&models.PlaylistResponse{
			Body: []byte("#EXTM3U\n#EXTINF:10.0,\nsegment1.ts"),
		}, nil)

	mockClient.EXPECT().GetSegment(gomock.Any(), segmentURL, false).
		Return(&models.SegmentResponse{Size: 1024}, nil)

	mockValidator.EXPECT().ValidateMaster(gomock.Any()).Return(nil)
	mockValidator.EXPECT().ValidateMedia(gomock.Any()).Return(nil)

	// Ожидаем все вызовы метрик в любом порядке
	mockMetrics.EXPECT().SetStreamUp("test_stream", true).AnyTimes()
	mockMetrics.EXPECT().RecordResponseTime("test_stream", gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().SetLastCheckTime("test_stream", gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().SetSegmentsCount("test_stream", gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordSegmentCheck("test_stream", true).AnyTimes()
	mockMetrics.EXPECT().SetStreamBitrate("test_stream", gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().SetActiveChecks(gomock.Any()).AnyTimes()

	result, err := checker.Check(context.Background(), models.StreamConfig{
		Name:      "test_stream",
		URL:       masterURL,
		CheckMode: models.CheckModeAll,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
}

func TestStreamChecker_Check_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := models.NewMockHTTPClient(ctrl)
	mockValidator := models.NewMockValidator(ctrl)
	mockMetrics := models.NewMockMetricsCollector(ctrl)

	checker := NewStreamChecker(mockClient, mockValidator, mockMetrics, 1)

	mockClient.EXPECT().GetPlaylist(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("network error"))

	// Ожидаем все возможные вызовы метрик при ошибке
	mockMetrics.EXPECT().SetStreamUp(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordResponseTime(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().SetLastCheckTime(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().SetSegmentsCount(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordSegmentCheck(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().SetStreamBitrate(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().SetActiveChecks(gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().RecordError("test_stream", string(models.ErrPlaylistDownload)).AnyTimes()

	result, err := checker.Check(context.Background(), models.StreamConfig{
		Name: "test_stream",
		URL:  "http://test.com/master.m3u8",
	})

	assert.Error(t, err)
	assert.False(t, result.Success)
}
