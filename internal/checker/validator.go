package checker

import (
	"errors"
	"fmt"
	"time"

	"math/rand"

	"github.com/grafov/m3u8"
	"github.com/iudanet/hls_exporter/pkg/models"
)

type HLSValidator struct {
	segmentValidator models.SegmentValidator // встраиваем интерфейс
}

func NewHLSValidator() *HLSValidator {
	return &HLSValidator{
		segmentValidator: NewSegmentValidator(), // создаем конкретную реализацию
	}
}
func (v *HLSValidator) ValidateSegment(
	segment *models.SegmentData,
	validation *models.MediaValidation,
) error {
	// Базовая валидация (всегда)
	if err := v.segmentValidator.ValidateBasic(segment); err != nil {
		return err
	}

	// Опциональная валидация медиаконтейнера
	if validation != nil {
		if err := v.segmentValidator.ValidateMedia(segment, validation); err != nil {
			return err
		}
	}

	return nil
}
func (v *HLSValidator) ValidateMaster(playlist *m3u8.MasterPlaylist) error {
	if playlist == nil {
		return errors.New("empty master playlist")
	}

	if len(playlist.Variants) == 0 {
		return errors.New("no variants in master playlist")
	}

	for i, variant := range playlist.Variants {
		if variant.URI == "" {
			return fmt.Errorf("empty URI in variant %d", i)
		}
	}

	return nil
}

func (v *HLSValidator) ValidateMedia(playlist *m3u8.MediaPlaylist) error {
	if playlist == nil {
		return errors.New("empty media playlist")
	}

	if playlist.Count() == 0 {
		return errors.New("no segments in media playlist")
	}

	// Проверка последовательности сегментов
	var prevSeq uint64
	for _, seg := range playlist.Segments {
		if seg == nil {
			continue
		}
		if seg.SeqId < prevSeq {
			return errors.New("invalid segment sequence")
		}
		prevSeq = seg.SeqId
	}

	return nil
}

func (v *HLSValidator) validateBasic(segment *models.SegmentData) error {
	if segment.Size == 0 {
		return &models.ValidationError{
			Type:    models.ErrSegmentSize,
			Message: "empty segment",
		}
	}

	return nil
}

func (v *HLSValidator) validateMedia(
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

func (c *StreamChecker) selectSegments(playlist *m3u8.MediaPlaylist, mode string) []*m3u8.MediaSegment {
	var segments []*m3u8.MediaSegment

	switch mode {
	case models.CheckModeAll:
		for _, seg := range playlist.Segments {
			if seg != nil {
				segments = append(segments, seg)
			}
		}

	case models.CheckModeFirstLast:
		if playlist.Count() > 0 {
			segments = make([]*m3u8.MediaSegment, 0, 2)
			first := playlist.Segments[0]
			last := playlist.Segments[playlist.Count()-1]

			if first != nil {
				segments = append(segments, first)
			}
			if last != nil && last != first {
				segments = append(segments, last)
			}
		}

	case models.CheckModeRandom:
		if playlist.Count() > 0 {
			// Инициализируем генератор случайных чисел
			r := rand.New(rand.NewSource(time.Now().UnixNano()))

			// Выбираем случайные сегменты
			count := min(3, int(playlist.Count()))
			seen := make(map[int]bool)

			for len(segments) < count {
				idx := r.Intn(int(playlist.Count()))
				if !seen[idx] && playlist.Segments[idx] != nil {
					segments = append(segments, playlist.Segments[idx])
					seen[idx] = true
				}
			}
		}
	}

	return segments
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
