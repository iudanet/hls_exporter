package checker

import (
	"errors"
	"fmt"
	"math/big"

	"crypto/rand"

	"github.com/grafov/m3u8"
	"github.com/iudanet/hls_exporter/pkg/models"
)

// Заменяем генерацию случайных чисел
func getRandomIndex(limit int64) (int, error) {
	if limit <= 0 {
		return 0, fmt.Errorf("invalid limit: %d", limit)
	}
	n, err := rand.Int(rand.Reader, big.NewInt(limit))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()), nil
}

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
			count := minInt(3, safeCount(playlist.Count()))
			seen := make(map[int]bool)

			playlistCount := safeInt64(playlist.Count())
			for len(segments) < count {
				idx, err := getRandomIndex(playlistCount)
				if err != nil {
					continue
				}
				if !seen[idx] && playlist.Segments[idx] != nil {
					segments = append(segments, playlist.Segments[idx])
					seen[idx] = true
				}
			}
		}
	}

	return segments
}

const (
	MaxInt   = int(^uint(0) >> 1)
	MaxInt64 = int64(^uint64(0) >> 1)
)

func safeCount(count uint) int {
	if count > uint(MaxInt) {
		return MaxInt
	}
	return int(count)
}
func safeInt64(n uint) int64 {
	if n > uint(MaxInt64) {
		return MaxInt64
	}
	return int64(n)
}
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
