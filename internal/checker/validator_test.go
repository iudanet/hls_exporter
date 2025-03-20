package checker

import (
	"fmt"
	"testing"

	"github.com/grafov/m3u8"
	"github.com/iudanet/hls_exporter/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestHLSValidator_ValidateSegment(t *testing.T) {
	validator := NewHLSValidator()

	tests := []struct {
		name       string
		segment    *models.SegmentData
		validation *models.MediaValidation
		wantErr    bool
		errType    models.ValidationType
	}{
		{
			name: "valid segment without media validation",
			segment: &models.SegmentData{
				Size:     1024,
				Duration: 2.0,
			},
			validation: nil,
			wantErr:    false,
		},
		{
			name: "valid segment with media validation",
			segment: &models.SegmentData{
				Size:     2048,
				Duration: 2.0,
				MediaInfo: models.MediaInfo{
					Container: "TS",
					HasVideo:  true,
					HasAudio:  true,
				},
			},
			validation: &models.MediaValidation{
				ContainerType:  []string{"TS"},
				MinSegmentSize: 1024,
				CheckVideo:     true,
				CheckAudio:     true,
			},
			wantErr: false,
		},
		{
			name: "invalid container type",
			segment: &models.SegmentData{
				Size:     2048,
				Duration: 2.0,
				MediaInfo: models.MediaInfo{
					Container: "MP4",
					HasVideo:  true,
					HasAudio:  true,
				},
			},
			validation: &models.MediaValidation{
				ContainerType:  []string{"TS"},
				MinSegmentSize: 1024,
				CheckVideo:     true,
				CheckAudio:     true,
			},
			wantErr: true,
			errType: models.ErrContainer,
		},
		{
			name: "segment too small",
			segment: &models.SegmentData{
				Size:     512,
				Duration: 2.0,
				MediaInfo: models.MediaInfo{
					Container: "TS",
					HasVideo:  true,
					HasAudio:  true,
				},
			},
			validation: &models.MediaValidation{
				ContainerType:  []string{"TS"},
				MinSegmentSize: 1024,
				CheckVideo:     true,
				CheckAudio:     true,
			},
			wantErr: true,
			errType: models.ErrSegmentSize,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateSegment(tt.segment, tt.validation)
			if tt.wantErr {
				assert.Error(t, err)
				if validationErr, ok := err.(*models.ValidationError); ok {
					assert.Equal(t, tt.errType, validationErr.Type)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetRandomIndex(t *testing.T) {
	tests := []struct {
		name    string
		limit   int64
		wantErr bool
	}{
		{
			name:    "valid limit",
			limit:   10,
			wantErr: false,
		},
		{
			name:    "zero limit",
			limit:   0,
			wantErr: true,
		},
		{
			name:    "negative limit",
			limit:   -1,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx, err := getRandomIndex(tt.limit)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.GreaterOrEqual(t, idx, 0)
				assert.Less(t, idx, int(tt.limit))
			}
		})
	}
}

func createPlaylist(count int) *m3u8.MediaPlaylist {
	p, _ := m3u8.NewMediaPlaylist(uint(count), uint(count))
	for i := 0; i < count; i++ {
		if err := p.Append(fmt.Sprintf("segment_%d.ts", i), 2.0, ""); err != nil {
			// In a test helper, we might want to panic on error since this shouldn't fail
			panic(fmt.Sprintf("failed to append segment to playlist: %v", err))
		}
	}
	return p
}

func TestSelectSegments(t *testing.T) {
	checker := &StreamChecker{}

	tests := []struct {
		name          string
		playlist      *m3u8.MediaPlaylist
		mode          string
		expectedCount int
		checkFirst    bool
		checkLast     bool
	}{
		{
			name:          "all segments",
			playlist:      createPlaylist(5),
			mode:          models.CheckModeAll,
			expectedCount: 5,
		},
		{
			name:          "first and last",
			playlist:      createPlaylist(5),
			mode:          models.CheckModeFirstLast,
			expectedCount: 2,
			checkFirst:    true,
			checkLast:     true,
		},
		{
			name:          "random segments",
			playlist:      createPlaylist(10),
			mode:          models.CheckModeRandom,
			expectedCount: 3, // default random sample size
		},
		{
			name:          "empty playlist",
			playlist:      createPlaylist(0),
			mode:          models.CheckModeAll,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segments := checker.selectSegments(tt.playlist, tt.mode)
			assert.Len(t, segments, tt.expectedCount)

			if tt.checkFirst {
				assert.Contains(t, segments[0].URI, "segment_0")
			}
			if tt.checkLast && tt.expectedCount > 0 {
				assert.Contains(t, segments[len(segments)-1].URI, fmt.Sprintf("segment_%d", tt.playlist.Count()-1))
			}
		})
	}
}

func TestSafeConversions(t *testing.T) {
	tests := []struct {
		name     string
		input    uint
		expected int
	}{
		{
			name:     "small number",
			input:    10,
			expected: 10,
		},
		{
			name:     "max uint",
			input:    ^uint(0),
			expected: MaxInt,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := safeCount(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
