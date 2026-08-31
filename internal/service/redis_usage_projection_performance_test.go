package service

import (
	"fmt"
	"strings"
	"testing"
)

var redisUsageProjectionBenchmarkResult string

// BenchmarkReplaySafeRedisUsageMessage measures the CPU and allocation cost of
// removing fields that cannot be retained after CPA's destructive queue pop.
// The input shapes are deterministic and deliberately include both ordinary
// success records and a failed record with large excluded payload fields.
func BenchmarkReplaySafeRedisUsageMessage(b *testing.B) {
	largeExcludedBody := strings.Repeat("sensitive-body-", 256)
	cases := []struct {
		name    string
		message string
	}{
		{
			name:    "ordinary_attempt",
			message: `{"timestamp":"2026-08-31T08:00:00Z","provider":"claude","model":"sonnet","request_id":"attempt-001","tokens":{"input_tokens":123,"output_tokens":45,"total_tokens":168}}`,
		},
		{
			name:    "failed_attempt_with_excluded_payload",
			message: fmt.Sprintf(`{"timestamp":"2026-08-31T08:00:00Z","provider":"claude","model":"sonnet","request_id":"attempt-002","failed":true,"fail":{"status_code":429,"body":%q},"response_headers":{"set-cookie":%q},"tokens":{"input_tokens":123,"output_tokens":45,"total_tokens":168}}`, largeExcludedBody, largeExcludedBody),
		},
		{
			name:    "invalid_attempt",
			message: "{invalid CPA payload that must not be retained",
		},
	}

	for _, benchmarkCase := range cases {
		b.Run(benchmarkCase.name, func(b *testing.B) {
			// Warm encoding/json's reflection cache before collecting the per-message cost.
			projected, err := replaySafeRedisUsageMessage(benchmarkCase.message)
			if err != nil {
				b.Fatalf("warm replay-safe projection: %v", err)
			}
			redisUsageProjectionBenchmarkResult = projected
			b.SetBytes(int64(len(benchmarkCase.message)))
			b.ReportAllocs()
			b.ResetTimer()
			for benchmarkIndex := 0; benchmarkIndex < b.N; benchmarkIndex++ {
				projected, err := replaySafeRedisUsageMessage(benchmarkCase.message)
				if err != nil {
					b.Fatalf("project replay-safe message: %v", err)
				}
				redisUsageProjectionBenchmarkResult = projected
			}
		})
	}
}
