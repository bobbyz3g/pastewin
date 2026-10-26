// Package updatecheck checks the project's public stable release without downloading updates.
package updatecheck

import (
	"context"
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

const endpoint = "https://api.github.com/repos/bobbyz3g/pastewin/releases/latest"
const maxResponse = 1 << 20

var (
	ErrNetwork   = errors.New("无法检查更新，请检查网络连接")
	ErrRateLimit = errors.New("请求过多，请稍后再试")
	ErrNoRelease = errors.New("暂无正式版本")
	ErrServer    = errors.New("更新服务暂不可用，请稍后重试")
	ErrVersion   = errors.New("无法识别版本信息")
)

type Result struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	Tag     string `json:"tag"`
}

func version(tag string) ([3]uint64, error) {
	var result [3]uint64
	parts := strings.Split(strings.TrimPrefix(tag, "v"), ".")
	if len(parts) != 3 {
		return result, ErrVersion
	}
	for i, part := range parts {
		if part == "" || len(part) > 1 && part[0] == '0' {
			return result, ErrVersion
		}
		for _, c := range part {
			if c < '0' || c > '9' {
				return result, ErrVersion
			}
		}
		n, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return result, ErrVersion
		}
		result[i] = n
	}
	return result, nil
}

// ReleaseURL constructs a link only within this project's releases.
func ReleaseURL(tag string) (string, error) {
	if _, err := version(tag); err != nil {
		return "", err
	}
	return "https://github.com/bobbyz3g/pastewin/releases/tag/" + tag, nil
}

// Check queries the latest stable release with a bounded request lifetime.
func Check(ctx context.Context, current string) (Result, error) {
	return check(ctx, &http.Client{Timeout: 10 * time.Second}, current)
}

func check(ctx context.Context, client *http.Client, current string) (Result, error) {
	local, err := version(current)
	if err != nil {
		return Result{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Result{}, ErrNetwork
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "PasteWin-update-check")
	resp, err := client.Do(req)
	if err != nil {
		return Result{}, ErrNetwork
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusTooManyRequests,
		resp.StatusCode == http.StatusForbidden && (resp.Header.Get("X-RateLimit-Remaining") == "0" || resp.Header.Get("Retry-After") != ""):
		return Result{}, ErrRateLimit
	case resp.StatusCode == http.StatusNotFound:
		return Result{}, ErrNoRelease
	case resp.StatusCode != http.StatusOK:
		return Result{}, ErrServer
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
	if err != nil {
		return Result{}, ErrNetwork
	}
	if len(data) > maxResponse {
		return Result{}, ErrServer
	}
	var release struct {
		Tag        string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}
	if err := json.Unmarshal(data, &release); err != nil {
		return Result{}, ErrServer
	}
	if release.Draft || release.Prerelease {
		return Result{}, ErrNoRelease
	}
	remote, err := version(release.Tag)
	if err != nil {
		return Result{}, err
	}
	status := "current"
	switch slices.Compare(remote[:], local[:]) {
	case 1:
		status = "available"
	case -1:
		status = "ahead"
	}
	return Result{Status: status, Version: strings.TrimPrefix(release.Tag, "v"), Tag: release.Tag}, nil
}
