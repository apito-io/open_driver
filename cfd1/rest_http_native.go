//go:build !js

package cfd1

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"time"
)

func restHTTPDoPlatform(ctx context.Context, method, url, authToken string, body []byte) (int, []byte, error) {
	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return 0, nil, err
	}
	if token := strings.TrimSpace(authToken); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, raw, nil
}
