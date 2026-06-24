package cfd1

import (
	"context"
	"fmt"
	"strings"
)

// restHTTPPost sends a JSON POST to the Cloudflare API and returns status + body.
func restHTTPPost(ctx context.Context, url, authToken string, body []byte) (int, []byte, error) {
	url = strings.TrimSpace(url)
	if url == "" {
		return 0, nil, fmt.Errorf("cfd1: url required")
	}
	return restHTTPDo(ctx, "POST", url, authToken, body)
}

func restHTTPDelete(ctx context.Context, url, authToken string) (int, []byte, error) {
	return restHTTPDo(ctx, "DELETE", url, authToken, nil)
}

func restHTTPDo(ctx context.Context, method, url, authToken string, body []byte) (int, []byte, error) {
	if strings.TrimSpace(method) == "" {
		method = "POST"
	}
	return restHTTPDoPlatform(ctx, method, url, authToken, body)
}
