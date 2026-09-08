package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// MetricsProvider 是运行时快照的 HTTP 层入口 seam；
// 实现由 app 组装各后台 runner 与 repository 读模型后提供。
type MetricsProvider interface {
	MetricsSnapshot(context.Context) (map[string]any, error)
}

// HTTPRequestObservation contains only bounded HTTP facts. Route is the Gin
// route template (for example /api/v1/usage/events/:id), never the raw URL.
type HTTPRequestObservation struct {
	Method string
	Route  string
	// Status is the final HTTP response status. The collector counts status
	// codes >=400 as errors, independently from request cancellation.
	Status   int
	Duration time.Duration
	Canceled bool
}

type httpRequestMetricsObserver interface {
	ObserveHTTPRequest(HTTPRequestObservation)
}

func registerMetricsRoute(router gin.IRoutes, provider MetricsProvider) {
	if provider == nil {
		return
	}
	if observer, ok := provider.(httpRequestMetricsObserver); ok {
		router.Use(observeHTTPRequests(observer))
	}
	router.GET("/metrics", func(c *gin.Context) {
		snapshot, err := provider.MetricsSnapshot(c.Request.Context())
		if err != nil {
			writeInternalError(c, "metrics snapshot is unavailable", err)
			return
		}
		c.JSON(http.StatusOK, snapshot)
	})
}

func observeHTTPRequests(observer httpRequestMetricsObserver) gin.HandlerFunc {
	return func(c *gin.Context) {
		startedAt := time.Now()
		observe := func(status int) {
			route := c.FullPath()
			if route == "" {
				// Gin has no bounded route template for unmatched requests. Skipping
				// them prevents raw paths from becoming an unbounded metrics label.
				return
			}
			observer.ObserveHTTPRequest(HTTPRequestObservation{
				Method:   c.Request.Method,
				Route:    route,
				Status:   status,
				Duration: time.Since(startedAt),
				Canceled: c.Request.Context().Err() != nil,
			})
		}
		defer func() {
			if recovered := recover(); recovered != nil {
				observe(http.StatusInternalServerError)
				panic(recovered)
			}
			observe(c.Writer.Status())
		}()
		c.Next()
	}
}
