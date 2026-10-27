// 本文件提供带重试和响应体上限的 HTTP 请求基础设施。
package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	// maxResponseBytes 单个响应的最大字节数，防止异常服务端耗尽内存。
	maxResponseBytes = 4 << 20
	// maxRetriesLimit 用户配置的最大重试次数上限。
	maxRetriesLimit = 10
	// baseRetryDelay 指数退避的初始等待时间。
	baseRetryDelay = 200 * time.Millisecond
	// maxRetryDelay 单次退避等待的上限。
	maxRetryDelay = 5 * time.Second
)

// getWithRetry 发送 GET 请求，对网络错误和 429/5xx 做有上限的指数退避重试。
// 最终响应（包括 4xx/5xx）会返回给调用方，由调用方生成具体错误信息。
func getWithRetry(ctx context.Context, client *http.Client, rawURL string, headers map[string]string, maxRetries int) (*http.Response, error) {
	if maxRetries < 0 {
		maxRetries = 0
	}
	if maxRetries > maxRetriesLimit {
		maxRetries = maxRetriesLimit
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "iskey")
		for key, value := range headers {
			req.Header.Set(key, value)
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			if attempt == maxRetries || !sleepBeforeRetry(ctx, attempt, 0) {
				return nil, err
			}
			continue
		}

		if !retryableStatus(resp.StatusCode) || attempt == maxRetries {
			return resp, nil
		}

		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
		lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
		if !sleepBeforeRetry(ctx, attempt, retryAfter) {
			return nil, ctx.Err()
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("request failed")
}

// retryableStatus 判断状态码是否适合重试；4xx 中仅 429 可重试。
func retryableStatus(code int) bool {
	return code == http.StatusTooManyRequests || code >= 500
}

// parseRetryAfter 解析 Retry-After 的秒数或 HTTP 日期格式。
func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil {
		if delay := time.Until(when); delay > 0 {
			return delay
		}
	}
	return 0
}

// sleepBeforeRetry 执行受限的指数退避，并在上下文取消时提前返回。
func sleepBeforeRetry(ctx context.Context, attempt int, retryAfter time.Duration) bool {
	delay := retryAfter
	if delay <= 0 {
		delay = baseRetryDelay << attempt
	}
	if delay > maxRetryDelay {
		delay = maxRetryDelay
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// readLimited 读取响应体，超过 maxResponseBytes 时返回错误而不是静默截断。
func readLimited(resp *http.Response) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxResponseBytes {
		return nil, fmt.Errorf("response body exceeds %d bytes", maxResponseBytes)
	}
	return body, nil
}
