package app

import (
	"sort"
	"sync"
	"time"

	"cpa-usage/internal/api"
)

const maxHTTPRequestMetricSeries = 128

var httpRequestDurationBounds = [...]time.Duration{
	10 * time.Millisecond,
	25 * time.Millisecond,
	50 * time.Millisecond,
	100 * time.Millisecond,
	250 * time.Millisecond,
	500 * time.Millisecond,
	time.Second,
	2500 * time.Millisecond,
	5 * time.Second,
	10 * time.Second,
}

type httpRequestMetricKey struct {
	method string
	route  string
}

type httpRequestMetric struct {
	requestsTotal uint64
	errorsTotal   uint64
	canceledTotal uint64
	durationTotal time.Duration
	durationCount [len(httpRequestDurationBounds) + 1]uint64
}

type httpRequestMetrics struct {
	mu                       sync.RWMutex
	series                   map[httpRequestMetricKey]*httpRequestMetric
	droppedObservationsTotal uint64
}

type httpRequestMetricCopy struct {
	key    httpRequestMetricKey
	metric httpRequestMetric
}

type httpRequestMetricsSnapshot struct {
	Status                   string                           `json:"status"`
	SeriesLimit              int                              `json:"series_limit"`
	DroppedObservationsTotal uint64                           `json:"dropped_observations_total"`
	Routes                   []httpRequestMetricRouteSnapshot `json:"routes"`
}

type httpRequestMetricRouteSnapshot struct {
	Method               string                       `json:"method"`
	Route                string                       `json:"route"`
	RequestsTotal        uint64                       `json:"requests_total"`
	ErrorsTotal          uint64                       `json:"errors_total"`
	CanceledTotal        uint64                       `json:"canceled_total"`
	DurationSecondsTotal float64                      `json:"duration_seconds_total"`
	DurationHistogram    httpDurationHistogram        `json:"duration_histogram"`
	DurationP95Bucket    httpDurationPercentileBucket `json:"duration_p95_bucket"`
}

type httpDurationHistogram struct {
	Buckets       []httpDurationBucket `json:"buckets"`
	OverflowCount uint64               `json:"overflow_count"`
}

type httpDurationBucket struct {
	UpperBoundSeconds float64 `json:"upper_bound_seconds"`
	CumulativeCount   uint64  `json:"cumulative_count"`
}

type httpDurationPercentileBucket struct {
	LowerBoundSeconds float64  `json:"lower_bound_seconds"`
	UpperBoundSeconds *float64 `json:"upper_bound_seconds"`
	Overflow          bool     `json:"overflow"`
}

// ObserveHTTPRequest records one bounded route-template observation. The API
// adapter supplies only registered route templates, never raw URLs or queries.
func (a *App) ObserveHTTPRequest(observation api.HTTPRequestObservation) {
	a.httpMetrics.observe(observation)
}

func (m *httpRequestMetrics) observe(observation api.HTTPRequestObservation) {
	if observation.Method == "" || observation.Route == "" {
		return
	}
	key := httpRequestMetricKey{method: observation.Method, route: observation.Route}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.series == nil {
		m.series = make(map[httpRequestMetricKey]*httpRequestMetric)
	}
	metric := m.series[key]
	if metric == nil {
		if len(m.series) >= maxHTTPRequestMetricSeries {
			m.droppedObservationsTotal++
			return
		}
		metric = &httpRequestMetric{}
		m.series[key] = metric
	}
	metric.requestsTotal++
	if observation.Status >= 400 {
		metric.errorsTotal++
	}
	if observation.Canceled {
		metric.canceledTotal++
	}
	duration := observation.Duration
	if duration < 0 {
		duration = 0
	}
	metric.durationTotal += duration
	bucket := sort.Search(len(httpRequestDurationBounds), func(index int) bool {
		return duration <= httpRequestDurationBounds[index]
	})
	metric.durationCount[bucket]++
}

func (m *httpRequestMetrics) snapshot() httpRequestMetricsSnapshot {
	m.mu.RLock()
	copies := make([]httpRequestMetricCopy, 0, len(m.series))
	for key, metric := range m.series {
		copies = append(copies, httpRequestMetricCopy{key: key, metric: *metric})
	}
	droppedObservationsTotal := m.droppedObservationsTotal
	m.mu.RUnlock()

	snapshot := httpRequestMetricsSnapshot{
		Status:                   "available",
		SeriesLimit:              maxHTTPRequestMetricSeries,
		DroppedObservationsTotal: droppedObservationsTotal,
		Routes:                   make([]httpRequestMetricRouteSnapshot, 0, len(copies)),
	}
	for _, copied := range copies {
		snapshot.Routes = append(snapshot.Routes, buildHTTPRequestMetricRouteSnapshot(copied.key, copied.metric))
	}
	sort.Slice(snapshot.Routes, func(left, right int) bool {
		if snapshot.Routes[left].Route == snapshot.Routes[right].Route {
			return snapshot.Routes[left].Method < snapshot.Routes[right].Method
		}
		return snapshot.Routes[left].Route < snapshot.Routes[right].Route
	})
	return snapshot
}

func buildHTTPRequestMetricRouteSnapshot(key httpRequestMetricKey, metric httpRequestMetric) httpRequestMetricRouteSnapshot {
	buckets := make([]httpDurationBucket, 0, len(httpRequestDurationBounds))
	var cumulative uint64
	for index, upperBound := range httpRequestDurationBounds {
		cumulative += metric.durationCount[index]
		buckets = append(buckets, httpDurationBucket{
			UpperBoundSeconds: upperBound.Seconds(),
			CumulativeCount:   cumulative,
		})
	}
	return httpRequestMetricRouteSnapshot{
		Method:               key.method,
		Route:                key.route,
		RequestsTotal:        metric.requestsTotal,
		ErrorsTotal:          metric.errorsTotal,
		CanceledTotal:        metric.canceledTotal,
		DurationSecondsTotal: metric.durationTotal.Seconds(),
		DurationHistogram: httpDurationHistogram{
			Buckets:       buckets,
			OverflowCount: metric.durationCount[len(httpRequestDurationBounds)],
		},
		DurationP95Bucket: durationPercentileBucket(metric.durationCount, metric.requestsTotal, 95),
	}
}

func durationPercentileBucket(counts [len(httpRequestDurationBounds) + 1]uint64, total uint64, percentile uint64) httpDurationPercentileBucket {
	if total == 0 || percentile == 0 {
		return httpDurationPercentileBucket{}
	}
	threshold := (total*percentile + 99) / 100
	var cumulative uint64
	for index, count := range counts {
		cumulative += count
		if cumulative < threshold {
			continue
		}
		lowerBound := float64(0)
		if index > 0 {
			lowerBound = httpRequestDurationBounds[index-1].Seconds()
		}
		if index == len(httpRequestDurationBounds) {
			return httpDurationPercentileBucket{LowerBoundSeconds: lowerBound, Overflow: true}
		}
		upperBound := httpRequestDurationBounds[index].Seconds()
		return httpDurationPercentileBucket{
			LowerBoundSeconds: lowerBound,
			UpperBoundSeconds: &upperBound,
		}
	}
	return httpDurationPercentileBucket{}
}
