package checker

import (
	"testing"

	"github.com/iudanet/hls_exporter/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestBasicSegmentValidator_ValidateBasic(t *testing.T) {
	validator := NewSegmentValidator()

	tests := []struct {
		name    string
		segment *models.SegmentData
		wantErr bool
	}{
		{
			name: "valid segment",
			segment: &models.SegmentData{
				Size:     1024,
				Duration: 2.0,
			},
			wantErr: false,
		},
		{
			name: "empty segment",
			segment: &models.SegmentData{
				Size:     0,
				Duration: 2.0,
			},
			wantErr: true,
		},
		{
			name: "invalid duration",
			segment: &models.SegmentData{
				Size:     1024,
				Duration: 0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateBasic(tt.segment)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBasicSegmentValidator_ValidateMedia(t *testing.T) {
	validator := NewSegmentValidator()

	tests := []struct {
		name       string
		segment    *models.SegmentData
		validation *models.MediaValidation
		wantErr    bool
	}{
		{
			name: "valid TS segment",
			segment: &models.SegmentData{
				Size: 2048,
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
		// Добавьте больше тест-кейсов
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateMedia(tt.segment, tt.validation)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
