package lidarr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/marcatos/cuearr/internal/domain"
)

const responseSnippetLimit = 4 << 10

type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	PathMap    []domain.PathMapRule
}

func (c Client) Ping(ctx context.Context) error {
	return c.do(ctx, http.MethodGet, "/api/v1/system/status", nil, "lidarr_ping")
}

func (c Client) RequestImport(ctx context.Context, albumPath string) error {
	payload := struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}{
		Name: "DownloadedAlbumsScan",
		Path: domain.MapLidarrPath(albumPath, c.PathMap),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode Lidarr import request: %w", err)
	}
	return c.do(ctx, http.MethodPost, "/api/v1/command", body, "lidarr_request_import")
}

func (c Client) do(
	ctx context.Context,
	method string,
	endpoint string,
	body []byte,
	operation string,
) error {
	started := time.Now()
	slog.Info("Lidarr request started", "operation", operation)

	request, err := http.NewRequestWithContext(
		ctx,
		method,
		strings.TrimRight(c.BaseURL, "/")+endpoint,
		bytes.NewReader(body),
	)
	if err != nil {
		slog.Error("Lidarr request failed",
			"operation", operation,
			"duration_ms", time.Since(started).Milliseconds(),
			"error_type", "build_request",
		)
		return fmt.Errorf("build Lidarr request: %w", err)
	}
	request.Header.Set("X-Api-Key", c.APIKey)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := c.httpClient().Do(request)
	if err != nil {
		slog.Error("Lidarr request failed",
			"operation", operation,
			"duration_ms", time.Since(started).Milliseconds(),
			"error_type", "transport",
		)
		return fmt.Errorf("send Lidarr request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		snippet, readErr := io.ReadAll(io.LimitReader(response.Body, responseSnippetLimit))
		if readErr != nil {
			slog.Error("Lidarr request failed",
				"operation", operation,
				"duration_ms", time.Since(started).Milliseconds(),
				"status", response.StatusCode,
				"error_type", "read_response",
			)
			return fmt.Errorf("Lidarr returned HTTP %d; read response: %w", response.StatusCode, readErr)
		}

		slog.Warn("Lidarr request rejected",
			"operation", operation,
			"duration_ms", time.Since(started).Milliseconds(),
			"status", response.StatusCode,
		)
		return fmt.Errorf(
			"Lidarr returned HTTP %d: %s",
			response.StatusCode,
			c.sanitize(strings.TrimSpace(string(snippet))),
		)
	}

	slog.Info("Lidarr request completed",
		"operation", operation,
		"duration_ms", time.Since(started).Milliseconds(),
		"status", response.StatusCode,
	)
	return nil
}

func (c Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func (c Client) sanitize(value string) string {
	if c.APIKey == "" {
		return value
	}
	return strings.ReplaceAll(value, c.APIKey, "[REDACTED]")
}
