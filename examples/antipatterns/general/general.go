// Package general contrasts language-level mistakes with their fixes.
// Functions ending in Bad are intentionally wrong; never copy them.
package general

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// TotalBad uses binary floating point for money: 0.1 + 0.2 != 0.3.
func TotalBad(prices []float64) float64 {
	var total float64
	for _, p := range prices {
		total += p
	}
	return total
}

// TotalGood sums exact integer minor units.
func TotalGood(pricesMinor []int64) int64 {
	var total int64
	for _, p := range pricesMinor {
		total += p
	}
	return total
}

type userIDKey struct{}

// GreetBad hides a business parameter in the context.
func GreetBad(ctx context.Context) string {
	id, _ := ctx.Value(userIDKey{}).(string)
	return "hello " + id
}

// GreetGood takes the parameter explicitly.
func GreetGood(_ context.Context, userID string) string { return "hello " + userID }

var ErrNotFound = errors.New("not found")

// IsNotFoundBad classifies by message text, which breaks on rewording and
// matches unrelated errors.
func IsNotFoundBad(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not found")
}

// IsNotFoundGood classifies by identity and supports wrapped errors. It returns
// false when err is nil.
func IsNotFoundGood(err error) bool { return errors.Is(err, ErrNotFound) }

// ProcessAllBad starts one unbounded, unowned goroutine per item and drops
// every error.
func ProcessAllBad(items []string, process func(string) error) {
	for _, item := range items {
		go func() { _ = process(item) }()
	}
}

// ProcessAllGood bounds concurrency, honors cancellation, waits for every
// goroutine, and returns all failures.
func ProcessAllGood(ctx context.Context, items []string, limit int, process func(context.Context, string) error) error {
	if limit <= 0 {
		return errors.New("limit must be positive")
	}
	sem := make(chan struct{}, limit)
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)
	for _, item := range items {
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			wg.Wait()
			return errors.Join(append(errs, ctx.Err())...)
		}
		wg.Go(func() {
			defer func() { <-sem }()
			if err := process(ctx, item); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("item %q: %w", item, err))
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	return errors.Join(errs...)
}
