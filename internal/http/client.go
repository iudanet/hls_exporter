package http

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/iudanet/hls_exporter/pkg/models"
)

type Client struct {
	httpClient *http.Client
	userAgent  string
}

var _ models.HTTPClient = (*Client)(nil)

func NewClient(config models.HTTPConfig) models.HTTPClient {
	transport := &http.Transport{
		MaxIdleConns:    config.MaxIdleConns,
		IdleConnTimeout: 90 * time.Second,
		TLSClientConfig: nil, // TODO: add TLS config if needed
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   config.Timeout,
	}

	return &Client{
		httpClient: client,
		userAgent:  config.UserAgent,
	}
}

func (c *Client) GetPlaylist(ctx context.Context, url string) (*models.PlaylistResponse, error) {
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &models.PlaylistResponse{
			StatusCode: resp.StatusCode,
			Duration:   time.Since(start),
			Headers:    resp.Header,
		}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	return &models.PlaylistResponse{
		Body:       body,
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Duration:   time.Since(start),
	}, nil
}

func (c *Client) GetSegment(ctx context.Context, url string, validate bool) (*models.SegmentResponse, error) {
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}

	// Если не нужна валидация, проверяем только заголовки
	if !validate {
		req.Method = http.MethodHead
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &models.SegmentResponse{
			StatusCode: resp.StatusCode,
			Duration:   time.Since(start),
		}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	segmentResponse := &models.SegmentResponse{
		StatusCode: resp.StatusCode,
		Duration:   time.Since(start),
	}

	// Получаем размер сегмента
	if contentLength := resp.Header.Get("Content-Length"); contentLength != "" {
		if size, err := parseInt64(contentLength); err == nil {
			segmentResponse.Size = size
		}
	}

	// Если нужна валидация, читаем и анализируем тело
	if validate {
		mediaInfo, err := c.analyzeSegment(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("analyze segment: %w", err)
		}
		segmentResponse.MediaInfo = mediaInfo
	}

	return segmentResponse, nil
}

func (c *Client) SetTimeout(timeout time.Duration) {
	c.httpClient.Timeout = timeout
}

func (c *Client) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}

// analyzeSegment анализирует медиа-контейнер сегмента
func (c *Client) analyzeSegment(_ io.Reader) (models.MediaInfo, error) {
	// TODO: Implement actual media container analysis
	// This is a placeholder that should be replaced with actual media container parsing
	return models.MediaInfo{
		Container:  "TS",
		HasVideo:   true,
		HasAudio:   true,
		IsComplete: true,
	}, nil
}

func parseInt64(s string) (int64, error) {
	var n int64
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
