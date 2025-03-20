package checker

import (
	"fmt"

	"github.com/iudanet/hls_exporter/pkg/models"
)

type BasicSegmentValidator struct{}

func NewSegmentValidator() *BasicSegmentValidator {
    return &BasicSegmentValidator{}
}

// Реализация интерфейса models.SegmentValidator
func (v *BasicSegmentValidator) ValidateBasic(segment *models.SegmentData) error {
    if segment.Size == 0 {
        return &models.ValidationError{
            Type:    models.ErrSegmentSize,
            Message: "empty segment",
        }
    }

    if segment.Duration <= 0 {
        return &models.ValidationError{
            Type:    models.ErrSegmentSize,
            Message: "invalid segment duration",
        }
    }

    return nil
}

func (v *BasicSegmentValidator) ValidateMedia(
    segment *models.SegmentData,
    validation *models.MediaValidation,
) error {
    // Проверка типа контейнера
    validContainer := false
    for _, ct := range validation.ContainerType {
        if segment.MediaInfo.Container == ct {
            validContainer = true
            break
        }
    }
    if !validContainer {
        return &models.ValidationError{
            Type:    models.ErrContainer,
            Message: fmt.Sprintf("invalid container type: %s", segment.MediaInfo.Container),
        }
    }

    // Проверка минимального размера
    if segment.Size < validation.MinSegmentSize {
        return &models.ValidationError{
            Type:    models.ErrSegmentSize,
            Message: fmt.Sprintf("segment size %d less than minimum %d", segment.Size, validation.MinSegmentSize),
        }
    }

    // Проверка наличия видео/аудио
    if validation.CheckVideo && !segment.MediaInfo.HasVideo {
        return &models.ValidationError{
            Type:    models.ErrNoVideo,
            Message: "no video track found",
        }
    }

    if validation.CheckAudio && !segment.MediaInfo.HasAudio {
        return &models.ValidationError{
            Type:    models.ErrNoAudio,
            Message: "no audio track found",
        }
    }

    return nil
}
