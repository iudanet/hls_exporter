package checker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/grafov/m3u8"
	"github.com/iudanet/hls_exporter/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock implementations
type MockHTTPClient struct {
	mock.Mock
}

func (m *MockHTTPClient) GetPlaylist(ctx context.Context, url string) (*models.PlaylistResponse, error) {
	args := m.Called(ctx, url)
	if resp := args.Get(0); resp != nil {
		return resp.(*models.PlaylistResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockHTTPClient) GetSegment(
	ctx context.Context,
	url string,
	validate bool,
) (*models.SegmentResponse, error) {
	args := m.Called(ctx, url, validate)
	if resp := args.Get(0); resp != nil {
		return resp.(*models.SegmentResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockHTTPClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockHTTPClient) SetTimeout(timeout time.Duration) {
	m.Called(timeout)
}

type MockValidator struct {
	mock.Mock
}

func (m *MockValidator) ValidateMaster(playlist *m3u8.MasterPlaylist) error {
	args := m.Called(playlist)
	return args.Error(0)
}

func (m *MockValidator) ValidateMedia(playlist *m3u8.MediaPlaylist) error {
	args := m.Called(playlist)
	return args.Error(0)
}

func (m *MockValidator) ValidateSegment(segment *models.SegmentData, validation *models.MediaValidation) error {
	args := m.Called(segment, validation)
	return args.Error(0)
}

type MockMetricsCollector struct {
	mock.Mock
}

func (m *MockMetricsCollector) SetStreamUp(name string, up bool) {
	m.Called(name, up)
}

func (m *MockMetricsCollector) RecordResponseTime(name string, duration float64) {
	m.Called(name, duration)
}

func (m *MockMetricsCollector) RecordError(name, errorType string) {
	m.Called(name, errorType)
}

func (m *MockMetricsCollector) SetLastCheckTime(name string, timestamp time.Time) {
	m.Called(name, timestamp)
}

func (m *MockMetricsCollector) SetSegmentsCount(name string, count int) {
	m.Called(name, count)
}

func (m *MockMetricsCollector) RecordSegmentCheck(name string, success bool) {
	m.Called(name, success)
}

func (m *MockMetricsCollector) SetActiveChecks(count int) {
	m.Called(count)
}
func (m *MockMetricsCollector) SetStreamBitrate(name string, bitrate float64) {
	m.Called(name, bitrate)
}

func TestStreamChecker_Check_Success(t *testing.T) {
	// Setup
	mockClient := new(MockHTTPClient)
	mockValidator := new(MockValidator)
	mockMetrics := new(MockMetricsCollector)

	checker := NewStreamChecker(mockClient, mockValidator, mockMetrics, 1)

	masterURL := "http://test.com/master.m3u8"

	// Setup master playlist expectations
	mockClient.On("GetPlaylist", mock.Anything, masterURL).Return(
		&models.PlaylistResponse{
			Body: []byte(`#EXTM3U
#EXT-X-VERSION:3
#EXT-X-STREAM-INF:BANDWIDTH=1000000
stream.m3u8`),
			StatusCode: 200,
		}, nil)

	// Setup variant playlist expectations
	mockClient.On("GetPlaylist", mock.Anything, "http://test.com/stream.m3u8").Return(
		&models.PlaylistResponse{
			Body: []byte(`#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:10
#EXTINF:10.0,
segment1.ts`),
			StatusCode: 200,
		}, nil)

	// Setup segment expectations
	mockClient.On("GetSegment", mock.Anything, "http://test.com/segment1.ts", false).Return(
		&models.SegmentResponse{
			Size:     1024,
			Duration: time.Second,
			MediaInfo: models.MediaInfo{
				Container: "TS",
				HasVideo:  true,
				HasAudio:  true,
			},
		}, nil)

	// Add validator expectations
	mockValidator.On("ValidateMaster", mock.AnythingOfType("*m3u8.MasterPlaylist")).Return(nil)
	mockValidator.On("ValidateMedia", mock.AnythingOfType("*m3u8.MediaPlaylist")).Return(nil)

	// Add metrics expectations
	mockMetrics.On("SetStreamUp", "test_stream", true).Return()
	mockMetrics.On("RecordResponseTime", "test_stream", mock.AnythingOfType("float64")).Return()
	mockMetrics.On("SetLastCheckTime", "test_stream", mock.AnythingOfType("time.Time")).Return()
	mockMetrics.On("SetSegmentsCount", "test_stream", mock.AnythingOfType("int")).Return()
	mockMetrics.On("SetActiveChecks", mock.AnythingOfType("int")).Return()
	mockMetrics.On("RecordSegmentCheck", "test_stream", true).Return()
	mockMetrics.On("SetStreamBitrate", "test_stream", mock.AnythingOfType("float64")).Return()

	// Execute
	result, err := checker.Check(context.Background(), models.StreamConfig{
		Name:            "test_stream",
		URL:             masterURL,
		CheckMode:       models.CheckModeAll,
		ValidateContent: false,
	})

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, 1, result.Segments.Checked)
	assert.Equal(t, 0, result.Segments.Failed)

	// Verify all expectations were met
	mockClient.AssertExpectations(t)
	mockValidator.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)
}

func TestStreamChecker_Check_MasterPlaylistError(t *testing.T) {
	// Setup
	ctx := context.Background()
	mockClient := new(MockHTTPClient)
	mockValidator := new(MockValidator)
	mockMetrics := new(MockMetricsCollector)

	checker := NewStreamChecker(mockClient, mockValidator, mockMetrics, 1)

	// Setup only the necessary expectations
	mockClient.On("GetPlaylist", ctx, "http://test.com/master.m3u8").Return(nil, errors.New("network error"))

	// Metric expectations that are actually called in updateMetrics
	mockMetrics.On("SetStreamUp", "test_stream", false).Return()
	mockMetrics.On("RecordResponseTime", "test_stream", mock.AnythingOfType("float64")).Return()
	mockMetrics.On("SetLastCheckTime", "test_stream", mock.AnythingOfType("time.Time")).Return()
	mockMetrics.On("SetSegmentsCount", "test_stream", mock.AnythingOfType("int")).Return()
	mockMetrics.On("SetActiveChecks", mock.AnythingOfType("int")).Return()
	mockMetrics.On("RecordSegmentCheck", "test_stream", false).Return()
	mockMetrics.On("SetStreamBitrate", "test_stream", mock.AnythingOfType("float64")).Return()
	mockMetrics.On("RecordError", "test_stream", string(models.ErrPlaylistDownload)).Return()

	// Execute
	result, err := checker.Check(ctx, models.StreamConfig{
		Name: "test_stream",
		URL:  "http://test.com/master.m3u8",
	})

	// Assert
	assert.Error(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Equal(t, models.ErrPlaylistDownload, result.Error.Type)

	// Verify expectations
	mockClient.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)
}

func TestResolveURL(t *testing.T) {
	tests := []struct {
		name         string
		baseURL      string
		relativePath string
		expected     string
	}{
		{
			name:         "absolute path",
			baseURL:      "http://test.com/master.m3u8",
			relativePath: "http://test.com/variant.m3u8",
			expected:     "http://test.com/variant.m3u8",
		},
		{
			name:         "relative path",
			baseURL:      "http://test.com/master.m3u8",
			relativePath: "variant.m3u8",
			expected:     "http://test.com/variant.m3u8",
		},
		{
			name:         "parent directory",
			baseURL:      "http://test.com/path/master.m3u8",
			relativePath: "../variant.m3u8",
			expected:     "http://test.com/variant.m3u8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveURL(tt.baseURL, tt.relativePath)
			assert.Equal(t, tt.expected, result)
		})
	}
}
