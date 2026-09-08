package app

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"cpa-usage/internal/api"
)

func TestHTTPRequestMetricsRecordsErrorsCancellationAndP95Bucket(t *testing.T) {
	var metrics httpRequestMetrics
	metrics.observe(api.HTTPRequestObservation{
		Method:   "GET",
		Route:    "/api/v1/usage/performance",
		Status:   200,
		Duration: 12 * time.Millisecond,
	})
	metrics.observe(api.HTTPRequestObservation{
		Method:   "GET",
		Route:    "/api/v1/usage/performance",
		Status:   503,
		Canceled: true,
		Duration: 6 * time.Second,
	})

	snapshot := metrics.snapshot()
	if snapshot.Status != "available" || len(snapshot.Routes) != 1 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	route := snapshot.Routes[0]
	if route.RequestsTotal != 2 || route.ErrorsTotal != 1 || route.CanceledTotal != 1 {
		t.Fatalf("unexpected request counters: %+v", route)
	}
	if route.DurationSecondsTotal != 6.012 {
		t.Fatalf("expected 6.012 duration seconds, got %v", route.DurationSecondsTotal)
	}
	if route.DurationP95Bucket.LowerBoundSeconds != 5 || route.DurationP95Bucket.UpperBoundSeconds == nil || *route.DurationP95Bucket.UpperBoundSeconds != 10 || route.DurationP95Bucket.Overflow {
		t.Fatalf("expected p95 in (5s, 10s] bucket, got %+v", route.DurationP95Bucket)
	}
	if got := route.DurationHistogram.Buckets[1].CumulativeCount; got != 1 {
		t.Fatalf("expected one request <=25ms, got %d", got)
	}
}

func TestHTTPRequestMetricsReportsOverflowP95WithoutFalseUpperBound(t *testing.T) {
	counts := [len(httpRequestDurationBounds) + 1]uint64{}
	counts[0] = 94
	counts[len(counts)-1] = 6

	bucket := durationPercentileBucket(counts, 100, 95)
	if !bucket.Overflow || bucket.UpperBoundSeconds != nil || bucket.LowerBoundSeconds != 10 {
		t.Fatalf("expected p95 overflow above 10s, got %+v", bucket)
	}
}

func TestHTTPRequestMetricsBoundsRouteSeries(t *testing.T) {
	var metrics httpRequestMetrics
	for index := 0; index <= maxHTTPRequestMetricSeries; index++ {
		metrics.observe(api.HTTPRequestObservation{
			Method: "GET",
			Route:  fmt.Sprintf("/route/%03d", index),
			Status: 200,
		})
	}

	snapshot := metrics.snapshot()
	if len(snapshot.Routes) != maxHTTPRequestMetricSeries || snapshot.SeriesLimit != maxHTTPRequestMetricSeries {
		t.Fatalf("expected %d retained route series, got %d", maxHTTPRequestMetricSeries, len(snapshot.Routes))
	}
	if snapshot.DroppedObservationsTotal != 1 {
		t.Fatalf("expected one dropped observation, got %d", snapshot.DroppedObservationsTotal)
	}
}

func TestHTTPRequestMetricsConcurrentObservationAndSnapshot(t *testing.T) {
	var metrics httpRequestMetrics
	const workers = 16
	const observationsPerWorker = 100

	var wait sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for observation := 0; observation < observationsPerWorker; observation++ {
				metrics.observe(api.HTTPRequestObservation{
					Method:   "POST",
					Route:    "/api/v1/sync",
					Status:   202,
					Duration: time.Millisecond,
				})
				_ = metrics.snapshot()
			}
		}()
	}
	wait.Wait()

	snapshot := metrics.snapshot()
	if len(snapshot.Routes) != 1 || snapshot.Routes[0].RequestsTotal != workers*observationsPerWorker {
		t.Fatalf("unexpected concurrent snapshot: %+v", snapshot)
	}
}
