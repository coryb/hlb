package solver

import (
	"context"

	"golang.org/x/sync/semaphore"
)

type concurrencyLimiterKey struct{}
type multiwriterKey struct{}

func WithConcurrencyLimiter(ctx context.Context, limiter *semaphore.Weighted) context.Context {
	return context.WithValue(ctx, concurrencyLimiterKey{}, limiter)
}

func ConcurrencyLimiter(ctx context.Context) *semaphore.Weighted {
	limiter, _ := ctx.Value(concurrencyLimiterKey{}).(*semaphore.Weighted)
	return limiter
}

func WithMultiWriter(ctx context.Context, mw *MultiWriter) context.Context {
	return context.WithValue(ctx, multiwriterKey{}, mw)
}

func LoadMultiWriter(ctx context.Context) *MultiWriter {
	mw, _ := ctx.Value(multiwriterKey{}).(*MultiWriter)
	return mw
}
