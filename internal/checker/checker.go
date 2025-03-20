package checker

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/grafov/m3u8"
	"github.com/iudanet/hls_exporter/pkg/models"
	"go.uber.org/zap"
)

var (
	_ models.Checker          = (*StreamChecker)(nil)
	_ models.Validator        = (*HLSValidator)(nil)
	_ models.SegmentValidator = (*BasicSegmentValidator)(nil)
)

type StreamChecker struct {
	client    models.HTTPClient
	validator models.Validator
	metrics   models.MetricsCollector
	workers   int
	wg        sync.WaitGroup
	logger    *zap.Logger
	stopCh    chan struct{}
}

func NewStreamChecker(
	client models.HTTPClient,
	validator models.Validator,
	metrics models.MetricsCollector,
	workers int,
) *StreamChecker {
	logger, _ := zap.NewProduction() // Можно передавать logger как параметр
	return &StreamChecker{
		client:    client,
		validator: validator,
		metrics:   metrics,
		workers:   workers,
		logger:    logger,
		stopCh:    make(chan struct{}),
	}
}
func (c *StreamChecker) StopCh() <-chan struct{} {
	return c.stopCh
}
func (c *StreamChecker) Start() error {
	c.client.SetTimeout(10 * time.Second) // Set timeout when starting the checker
	for i := 0; i < c.workers; i++ {
		c.wg.Add(1)
		go c.worker()
	}
	return nil
}

func (c *StreamChecker) handleError(
	result *models.CheckResult,
	err error,
	errType models.ErrorType,
) error {
	result.Success = false
	result.Error = &models.CheckError{
		Type:    errType,
		Message: err.Error(),
	}
	return err
}

func (c *StreamChecker) Stop() error {
	select {
	case <-c.stopCh:
		// Channel is already closed
		return nil
	default:
		close(c.stopCh)
	}
	c.wg.Wait()
	return nil
}
func (c *StreamChecker) Check(ctx context.Context, stream models.StreamConfig) (*models.CheckResult, error) {
	result := c.initResult(stream)
	start := result.Timestamp

	// Обработка мастер-плейлиста
	masterPlaylist, masterResp, err := c.checkMasterPlaylist(ctx, stream.URL, result)
	if err != nil {
		result.Duration = time.Since(start)
		// Обновляем метрики после установки всех полей
		c.updateMetrics(stream.Name, result)
		return result, err
	}

	// Проверка сегментов
	segResults := c.checkVariants(ctx, masterPlaylist, stream)
	result = c.updateResultStatus(result, masterPlaylist, masterResp, segResults)
	result.Duration = time.Since(start)

	// Устанавливаем статус до обновления метрик
	if segResults.Failed > 0 {
		result.Success = false
		errMsg := fmt.Sprintf("%d of %d segments failed validation", segResults.Failed, segResults.Total)
		result.Error = &models.CheckError{
			Type:    models.ErrSegmentValidate,
			Message: errMsg,
		}
		c.updateMetrics(stream.Name, result)
		return result, fmt.Errorf("segment validation failed: %s", errMsg)
	}

	// Успешное завершение
	result.Success = true
	c.updateMetrics(stream.Name, result)
	return result, nil
}

func (c *StreamChecker) initResult(stream models.StreamConfig) *models.CheckResult {
	return &models.CheckResult{
		Timestamp:  time.Now(),
		StreamName: stream.Name,
		Success:    false,
	}
}

func (c *StreamChecker) checkMasterPlaylist(ctx context.Context, url string, result *models.CheckResult) (*m3u8.MasterPlaylist, *models.PlaylistResponse, error) {
	masterResp, err := c.client.GetPlaylist(ctx, url)
	if err != nil {
		return nil, nil, c.handleError(result, err, models.ErrPlaylistDownload)
	}

	masterPlaylist, err := parseMasterPlaylist(masterResp.Body)
	if err != nil {
		return nil, nil, c.handleError(result, err, models.ErrPlaylistParse)
	}

	if err := c.validator.ValidateMaster(masterPlaylist); err != nil {
		return nil, nil, c.handleError(result, err, models.ErrPlaylistParse)
	}

	return masterPlaylist, masterResp, nil
}

func (c *StreamChecker) updateResultStatus(result *models.CheckResult, masterPlaylist *m3u8.MasterPlaylist, masterResp *models.PlaylistResponse, segResults models.SegmentResults) *models.CheckResult {
	var lastModified time.Time
	if lm := masterResp.Headers.Get("Last-Modified"); lm != "" {
		if t, err := time.Parse(time.RFC1123, lm); err == nil {
			lastModified = t
		}
	}

	result.Segments = segResults
	result.StreamStatus = models.StreamStatus{
		IsLive:        true,
		VariantsCount: len(masterPlaylist.Variants),
		SegmentsCount: segResults.Checked,
		LastModified:  lastModified,
	}

	return result
}

func (c *StreamChecker) checkVariants(
	ctx context.Context,
	master *m3u8.MasterPlaylist,
	cfg models.StreamConfig,
) models.SegmentResults {
	results := models.SegmentResults{}
	baseURL := cfg.URL

	var wg sync.WaitGroup
	resultCh := make(chan models.SegmentCheck, len(master.Variants)*10) // Буферизованный канал для результатов

	for _, variant := range master.Variants {
		if variant == nil {
			continue
		}

		variantURL := resolveURL(baseURL, variant.URI)
		wg.Add(1)
		go func(variantURL string) {
			defer wg.Done()
			variantResp, err := c.client.GetPlaylist(ctx, variantURL)
			if err != nil {
				c.logger.Error("Failed to get variant playlist",
					zap.String("uri", variant.URI),
					zap.String("url", variantURL),
					zap.Error(err))
				return
			}

			mediaPlaylist, err := parseMediaPlaylist(variantResp.Body)
			if err != nil {
				c.logger.Error("Failed to parse media playlist",
					zap.String("uri", variant.URI),
					zap.Error(err))
				return
			}

			if err := c.validator.ValidateMedia(mediaPlaylist); err != nil {
				c.logger.Error("Failed to validate media playlist",
					zap.String("uri", variant.URI),
					zap.Error(err))
				return
			}

			for _, seg := range mediaPlaylist.Segments {
				if seg != nil {
					seg.URI = resolveURL(variantURL, seg.URI)
				}
			}

			segments := c.selectSegments(mediaPlaylist, cfg.CheckMode)
			results.Total += len(segments)

			for _, seg := range segments {
				if seg == nil {
					continue
				}

				wg.Add(1)
				go func(seg *m3u8.MediaSegment) {
					defer wg.Done()
					segCheck := c.checkSegment(ctx, seg, cfg)
					resultCh <- segCheck
				}(seg)
			}
		}(variantURL)
	}

	// Закрываем канал после завершения всех горутин
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// Собираем результаты из канала
	for segCheck := range resultCh {
		results.Checked++
		results.Details = append(results.Details, segCheck)
		if !segCheck.Success {
			results.Failed++
		}
	}

	return results
}
func (c *StreamChecker) checkSegment(ctx context.Context, segment *m3u8.MediaSegment, cfg models.StreamConfig) models.SegmentCheck {
	check := models.SegmentCheck{
		URL:     segment.URI,
		Success: false,
	}

	resp, err := c.client.GetSegment(ctx, segment.URI, cfg.ValidateContent)
	if err != nil {
		c.logger.Debug("Segment download failed",
			zap.String("url", segment.URI),
			zap.Error(err))
		check.Error = &models.CheckError{
			Type:    models.ErrSegmentDownload,
			Message: err.Error(),
		}
		return check
	}

	// Add logging for successful download
	c.logger.Debug("Segment downloaded successfully",
		zap.String("url", segment.URI),
		zap.Int64("size", resp.Size))

	// Если валидация контента отключена, считаем сегмент успешным
	if !cfg.ValidateContent {
		check.Success = true
		check.Duration = resp.Duration
		return check
	}

	segData := &models.SegmentData{
		URI:       segment.URI,
		Duration:  segment.Duration,
		Size:      resp.Size,
		MediaInfo: resp.MediaInfo,
	}

	if err := c.validator.ValidateSegment(segData, cfg.MediaValidation); err != nil {
		if c.logger != nil {
			c.logger.Debug("Segment validation failed",
				zap.String("url", segment.URI),
				zap.Error(err))
		}
		check.Error = &models.CheckError{
			Type:    models.ErrSegmentValidate,
			Message: err.Error(),
		}
		return check
	}

	check.Success = true
	check.Duration = resp.Duration
	return check
}

func (c *StreamChecker) worker() {
	defer c.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			// Здесь можно добавить периодические проверки
		}
	}
}

func (c *StreamChecker) updateMetrics(stream string, result *models.CheckResult) {
	c.metrics.SetStreamUp(stream, result.Success)
	c.metrics.RecordResponseTime(stream, result.Duration.Seconds())
	c.metrics.SetLastCheckTime(stream, result.Timestamp)
	c.metrics.SetSegmentsCount(stream, result.Segments.Checked)
	c.metrics.SetActiveChecks(c.workers)
	c.metrics.RecordSegmentCheck(stream, result.Success)
	c.metrics.SetStreamBitrate(stream, 0.0) // Add proper bitrate calculation if needed

	if result.Error != nil {
		c.metrics.RecordError(stream, string(result.Error.Type))
	}
}

func parseMasterPlaylist(data []byte) (*m3u8.MasterPlaylist, error) {
	playlist, listType, err := m3u8.DecodeFrom(bytes.NewReader(data), false)
	if err != nil {
		return nil, err
	}

	if listType != m3u8.MASTER {
		return nil, fmt.Errorf("expected master playlist, got %v", listType)
	}

	return playlist.(*m3u8.MasterPlaylist), nil
}

func parseMediaPlaylist(data []byte) (*m3u8.MediaPlaylist, error) {
	playlist, listType, err := m3u8.DecodeFrom(bytes.NewReader(data), false)
	if err != nil {
		return nil, err
	}

	if listType != m3u8.MEDIA {
		return nil, fmt.Errorf("expected media playlist, got %v", listType)
	}

	return playlist.(*m3u8.MediaPlaylist), nil
}

func resolveURL(baseURL, relativePath string) string {
	base, err := url.Parse(baseURL)
	if err != nil {
		return relativePath
	}

	relative, err := url.Parse(relativePath)
	if err != nil {
		return relativePath
	}

	return base.ResolveReference(relative).String()
}
