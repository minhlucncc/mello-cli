package mello

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// RefreshSession exchanges a refresh token for a fresh access token via
// POST {baseURL}/auth/refresh (internal API). The server rotates the refresh
// token, so the new one (from the Set-Cookie header) is returned and must be
// persisted by the caller. The refresh token is sent as the `refresh_token`
// cookie, matching the web client.
func RefreshSession(ctx context.Context, baseURL, refreshToken string, timeout time.Duration) (access, newRefresh string, err error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	base := strings.TrimRight(baseURL, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/auth/refresh", nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Accept", "application/json")
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: refreshToken})

	resp, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", newAPIError(http.MethodPost, "/auth/refresh", resp.StatusCode, data)
	}

	for _, ck := range resp.Cookies() {
		if ck.Name == "refresh_token" && ck.Value != "" {
			newRefresh = ck.Value
		}
	}
	var body map[string]any
	_ = json.Unmarshal(data, &body)
	access = pickStr(body, "access_token", "accessToken", "token", "jwt")
	// Some deployments nest it under a session/data object.
	if access == "" {
		for _, k := range []string{"session", "data", "result"} {
			if o, ok := body[k].(map[string]any); ok {
				if a := pickStr(o, "access_token", "accessToken", "token", "jwt"); a != "" {
					access = a
					break
				}
			}
		}
	}
	if access == "" {
		return "", "", fmt.Errorf("refresh succeeded but no access token found in the response")
	}
	if newRefresh == "" {
		newRefresh = refreshToken
	}
	return access, newRefresh, nil
}
