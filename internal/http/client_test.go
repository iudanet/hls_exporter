package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/iudanet/hls_exporter/pkg/models"
)

func TestClient_GetPlaylist(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse string
		statusCode     int
		wantErr        bool
	}{
		{
			name:           "successful response",
			serverResponse: "#EXTM3U\n#EXT-X-VERSION:3",
			statusCode:     http.StatusOK,
			wantErr:        false,
		},
		{
			name:           "not found error",
			serverResponse: "Not Found",
			statusCode:     http.StatusNotFound,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
				if _, err := w.Write([]byte(tt.serverResponse)); err != nil {
					t.Fatalf("Failed to write response: %v", err)
				}
			}))
			defer server.Close()

			client := NewClient(models.HTTPConfig{
				Timeout:   5 * time.Second,
				UserAgent: "test-agent",
			}).(*Client)

			resp, err := client.GetPlaylist(context.Background(), server.URL)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetPlaylist() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if resp.StatusCode != tt.statusCode {
					t.Errorf("GetPlaylist() statusCode = %v, want %v", resp.StatusCode, tt.statusCode)
				}
				if string(resp.Body) != tt.serverResponse {
					t.Errorf("GetPlaylist() body = %v, want %v", string(resp.Body), tt.serverResponse)
				}
			}
		})
	}
}

func TestClient_GetSegment(t *testing.T) {
	tests := []struct {
		name       string
		validate   bool
		statusCode int
		size       string
		wantErr    bool
	}{
		{
			name:       "head request only",
			validate:   false,
			statusCode: http.StatusOK,
			size:       "1024",
			wantErr:    false,
		},
		{
			name:       "full validation",
			validate:   true,
			statusCode: http.StatusOK,
			size:       "2048",
			wantErr:    false,
		},
		{
			name:       "error response",
			validate:   false,
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.size != "" {
					w.Header().Set("Content-Length", tt.size)
				}
				w.WriteHeader(tt.statusCode)
				if r.Method != http.MethodHead {
					if _, err := w.Write([]byte("fake segment data")); err != nil {
						t.Fatalf("Failed to write response: %v", err)
					}
				}
			}))
			defer server.Close()

			client := NewClient(models.HTTPConfig{
				Timeout:   5 * time.Second,
				UserAgent: "test-agent",
			})

			resp, err := client.GetSegment(context.Background(), server.URL, tt.validate)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSegment() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if resp.StatusCode != tt.statusCode {
					t.Errorf("GetSegment() statusCode = %v, want %v", resp.StatusCode, tt.statusCode)
				}
				if tt.size != "" {
					expectedSize, _ := parseInt64(tt.size)
					if resp.Size != expectedSize {
						t.Errorf("GetSegment() size = %v, want %v", resp.Size, expectedSize)
					}
				}
			}
		})
	}
}

func TestClient_SetTimeout(t *testing.T) {
	client := NewClient(models.HTTPConfig{
		Timeout: 5 * time.Second,
	}).(*Client)

	newTimeout := 10 * time.Second
	client.SetTimeout(newTimeout)

	if client.httpClient.Timeout != newTimeout {
		t.Errorf("SetTimeout() = %v, want %v", client.httpClient.Timeout, newTimeout)
	}
}

func TestClient_Context(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(models.HTTPConfig{
		Timeout: 5 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.GetPlaylist(ctx, server.URL)
	if err == nil {
		t.Error("GetPlaylist() should fail with context deadline exceeded")
	}
}
