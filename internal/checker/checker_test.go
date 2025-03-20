package checker

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/grafov/m3u8"
	"github.com/iudanet/hls_exporter/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock structures
type MockHTTPClient struct {
	mock.Mock
}

func (m *MockHTTPClient) GetPlaylist(ctx context.Context, url string) (*models.PlaylistResponse, error) {
	args := m.Called(ctx, url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	resp := args.Get(0).(*models.PlaylistResponse)
	return resp, args.Error(1)
}

func (m *MockHTTPClient) GetSegment(ctx context.Context, url string, validate bool) (*models.SegmentResponse, error) {
	args := m.Called(ctx, url, validate)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	resp := args.Get(0).(*models.SegmentResponse)
	return resp, args.Error(1)
}

func (m *MockHTTPClient) SetTimeout(timeout time.Duration) {
	m.Called(timeout)
}

func (m *MockHTTPClient) Close() error {
	args := m.Called()
	return args.Error(0)
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

func (m *MockMetricsCollector) RecordSegmentCheck(name string, success bool) {
	m.Called(name, success)
}

func (m *MockMetricsCollector) SetStreamBitrate(name string, bitrate float64) {
	m.Called(name, bitrate)
}

func (m *MockMetricsCollector) SetSegmentsCount(name string, count int) {
	m.Called(name, count)
}

func (m *MockMetricsCollector) RecordError(name, errorType string) {
	m.Called(name, errorType)
}

func (m *MockMetricsCollector) SetLastCheckTime(name string, timestamp time.Time) {
	m.Called(name, timestamp)
}

func (m *MockMetricsCollector) SetActiveChecks(count int) {
	m.Called(count)
}

// Test cases
func TestStreamChecker_Check(t *testing.T) {
	tests := []struct {
		name    string
		config  models.StreamConfig
		setup   func(*MockHTTPClient, *MockValidator, *MockMetricsCollector)
		want    *models.CheckResult
		wantErr bool
	}{
		{
			name: "successful check",
			config: models.StreamConfig{
				Name:            "test_stream",
				URL:             "http://example.com/master.m3u8",
				CheckMode:       models.CheckModeFirstLast,
				ValidateContent: true,
				MediaValidation: &models.MediaValidation{
					ContainerType:  []string{"TS"},
					MinSegmentSize: 1024,
					CheckAudio:     true,
					CheckVideo:     true,
				},
			},
			setup: func(httpClient *MockHTTPClient, validator *MockValidator, metrics *MockMetricsCollector) {
				// Setup mock для master playlist
				masterPlaylist := &models.PlaylistResponse{
					Body:       []byte("#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1280000\nstream.m3u8"),
					StatusCode: http.StatusOK,
					Headers:    http.Header{},
				}
				httpClient.On("GetPlaylist", mock.Anything, "http://example.com/master.m3u8").Return(masterPlaylist, nil)

				// Setup mock для variant playlist
				variantPlaylist := &models.PlaylistResponse{
					Body:       []byte("#EXTM3U\n#EXTINF:2.0,\nsegment1.ts\n#EXTINF:2.0,\nsegment2.ts"),
					StatusCode: http.StatusOK,
					Headers:    http.Header{},
				}
				httpClient.On("GetPlaylist", mock.Anything, "stream.m3u8").Return(variantPlaylist, nil)

				// Setup mock для сегментов
				segmentResponse := &models.SegmentResponse{
					MediaInfo: models.MediaInfo{
						Container: "TS",
						HasVideo:  true,
						HasAudio:  true,
					},
					StatusCode: http.StatusOK,
					Size:       1024 * 10,
					Duration:   time.Second,
				}
				httpClient.On("GetSegment", mock.Anything, mock.AnythingOfType("string"), true).Return(segmentResponse, nil)

				// Setup validator mocks
				validator.On("ValidateMaster", mock.Anything).Return(nil)
				validator.On("ValidateMedia", mock.Anything).Return(nil)
				validator.On("ValidateSegment", mock.Anything, mock.Anything).Return(nil)

				// Setup metrics mocks
				setupBaseMetrics(metrics, "test_stream", true) // меняем false на true для успешного кейса
			},
			want: &models.CheckResult{
				Success: true,
				StreamStatus: models.StreamStatus{
					IsLive:        true,
					VariantsCount: 1,
					SegmentsCount: 2,
				},
				Segments: models.SegmentResults{
					Checked: 2,
					Failed:  0,
				},
			},
			wantErr: false,
		},
		// Добавьте тест-кейс для ошибки
		{
			name: "master playlist error",
			config: models.StreamConfig{
				Name: "test_stream",
				URL:  "http://example.com/master.m3u8",
			},
			setup: func(httpClient *MockHTTPClient, validator *MockValidator, metrics *MockMetricsCollector) {
				// Порядок вызовов важен
				httpClient.On("GetPlaylist", mock.Anything, "http://example.com/master.m3u8").
					Return(nil, fmt.Errorf("network error"))

				// Метрики вызываются в handleError
				setupBaseMetrics(metrics, "test_stream", false)
				metrics.On("RecordError", "test_stream", string(models.ErrPlaylistDownload)).Return()
			},
			want: &models.CheckResult{
				Success: false,
				Error: &models.CheckError{
					Type:    models.ErrPlaylistDownload,
					Message: "network error",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid master playlist",
			config: models.StreamConfig{
				Name: "test_stream",
				URL:  "http://example.com/master.m3u8",
			},
			setup: func(httpClient *MockHTTPClient, validator *MockValidator, metrics *MockMetricsCollector) {
				// Возвращаем некорректный плейлист
				masterPlaylist := &models.PlaylistResponse{
					Body:       []byte("invalid playlist"),
					StatusCode: http.StatusOK,
				}
				httpClient.On("GetPlaylist", mock.Anything, "http://example.com/master.m3u8").
					Return(masterPlaylist, nil)
				setupBaseMetrics(metrics, "test_stream", false)
				metrics.On("RecordError", "test_stream", string(models.ErrPlaylistParse)).Return()
			},
			want: &models.CheckResult{
				Success: false,
				Error: &models.CheckError{
					Type: models.ErrPlaylistParse,
				},
			},
			wantErr: true,
		},
		{
			name: "segment_validation_error",
			config: models.StreamConfig{
				Name:            "test_stream",
				URL:             "http://example.com/master.m3u8",
				ValidateContent: true,
				MediaValidation: &models.MediaValidation{
					ContainerType:  []string{"TS"},
					MinSegmentSize: 1024,
				},
			},
			setup: func(httpClient *MockHTTPClient, validator *MockValidator, metrics *MockMetricsCollector) {
				// Setup успешную загрузку плейлистов
				masterPlaylist := &models.PlaylistResponse{
					Body:       []byte("#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1280000\nstream.m3u8"),
					StatusCode: http.StatusOK,
					Headers:    http.Header{},
				}
				httpClient.On("GetPlaylist", mock.Anything, "http://example.com/master.m3u8").
					Return(masterPlaylist, nil)

				variantPlaylist := &models.PlaylistResponse{
					Body:       []byte("#EXTM3U\n#EXTINF:2.0,\nsegment.ts"),
					StatusCode: http.StatusOK,
					Headers:    http.Header{},
				}
				httpClient.On("GetPlaylist", mock.Anything, "stream.m3u8").
					Return(variantPlaylist, nil)

				segmentResponse := &models.SegmentResponse{
					MediaInfo: models.MediaInfo{
						Container: "MP4",
						HasVideo:  true,
						HasAudio:  true,
					},
					StatusCode: http.StatusOK,
					Size:       1024,
					Duration:   time.Second,
				}
				httpClient.On("GetSegment", mock.Anything, mock.AnythingOfType("string"), true).
					Return(segmentResponse, nil)

				// Setup validator mocks
				validator.On("ValidateMaster", mock.Anything).Return(nil)
				validator.On("ValidateMedia", mock.Anything).Return(nil)
				validator.On("ValidateSegment", mock.Anything, mock.Anything).
					Return(fmt.Errorf("invalid container type"))

				// Setup metrics - все вызовы через helper
				setupBaseMetrics(metrics, "test_stream", false)
				metrics.On("RecordError", "test_stream", string(models.ErrSegmentValidate)).Return()
			},
			want: &models.CheckResult{
				Success:    false,
				StreamName: "test_stream",
				Error: &models.CheckError{
					Type:    models.ErrSegmentValidate,
					Message: "1 of 1 segments failed validation",
				},
				StreamStatus: models.StreamStatus{
					IsLive:        true,
					VariantsCount: 1,
					SegmentsCount: 1,
				},
				Segments: models.SegmentResults{
					Checked: 1,
					Failed:  1,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize mocks
			httpClient := new(MockHTTPClient)
			validator := new(MockValidator)
			metrics := new(MockMetricsCollector)

			// Setup mocks
			if tt.setup != nil {
				tt.setup(httpClient, validator, metrics)
			}

			// Create checker
			c := NewStreamChecker(httpClient, validator, metrics, 1)

			// Run test
			got, err := c.Check(context.Background(), tt.config)

			// Assert results
			if tt.wantErr {
				assert.Error(t, err)
				if tt.want.Error != nil {
					assert.Equal(t, tt.want.Error.Type, got.Error.Type)
					assert.Contains(t, got.Error.Message, tt.want.Error.Message)
				}
				assert.False(t, got.Success)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want.Success, got.Success)
				if tt.want.StreamStatus.VariantsCount > 0 {
					assert.Equal(t, tt.want.StreamStatus.VariantsCount, got.StreamStatus.VariantsCount)
				}
				assert.Equal(t, tt.want.Segments.Checked, got.Segments.Checked)
				assert.Equal(t, tt.want.Segments.Failed, got.Segments.Failed)
			}

			// Всегда проверяем базовые поля
			assert.NotZero(t, got.Timestamp)
			assert.NotZero(t, got.Duration)
			assert.Equal(t, tt.config.Name, got.StreamName)

			// Verify mock expectations
			httpClient.AssertExpectations(t)
			validator.AssertExpectations(t)
			metrics.AssertExpectations(t)
		})
	}
}

func TestStreamChecker_Lifecycle(t *testing.T) {
	httpClient := new(MockHTTPClient)
	validator := new(MockValidator)
	metrics := new(MockMetricsCollector)

	checker := NewStreamChecker(httpClient, validator, metrics, 2)

	// Test Start
	err := checker.Start()
	assert.NoError(t, err)

	// Test Stop
	err = checker.Stop()
	assert.NoError(t, err)
}

// Add validator tests
func TestHLSValidator_ValidateMaster(t *testing.T) {
	validator := NewHLSValidator()

	tests := []struct {
		name     string
		playlist *m3u8.MasterPlaylist
		wantErr  bool
	}{
		{
			name:     "nil playlist",
			playlist: nil,
			wantErr:  true,
		},
		// Add more test cases
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateMaster(tt.playlist)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
func setupBaseMetrics(metrics *MockMetricsCollector, streamName string, success bool) {
	metrics.On("SetLastCheckTime", streamName, mock.AnythingOfType("time.Time")).Return()
	metrics.On("SetStreamUp", streamName, success).Return() // Важно: передаем правильное значение success
	metrics.On("RecordResponseTime", streamName, mock.AnythingOfType("float64")).Return()
	metrics.On("SetSegmentsCount", streamName, mock.AnythingOfType("int")).Return()
}
