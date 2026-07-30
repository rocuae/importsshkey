package utils

import (
	"context"
	"fmt"
	"math"
	"time"
)

// RetryWithBackoff 指数退避重试
// 参数：
//   - ctx: 上下文
//   - maxRetries: 最大重试次数
//   - baseDelay: 基础延迟时间
//   - fn: 要重试的函数
//
// 返回：
//   - error: 最后一次执行的错误（成功则为 nil）
func RetryWithBackoff(ctx context.Context, maxRetries int, baseDelay time.Duration, fn func() error) error {
	var lastErr error
	for i := 0; i <= maxRetries; i++ {
		if err := fn(); err != nil {
			lastErr = err
			if i < maxRetries {
				delay := time.Duration(math.Pow(2, float64(i))) * baseDelay
				select {
				case <-ctx.Done():
					return fmt.Errorf("context canceled: %w", ctx.Err())
				case <-time.After(delay):
					continue
				}
			}
		} else {
			return nil
		}
	}
	return fmt.Errorf("max retries exceeded: %w", lastErr)
}
