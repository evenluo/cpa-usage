package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

// MetricsProvider 是运行时快照的 HTTP 层入口 seam；
// 实现由 app 组装各后台 runner 与 repository 读模型后提供。
type MetricsProvider interface {
	MetricsSnapshot(context.Context) (map[string]any, error)
}

func registerMetricsRoute(router gin.IRoutes, provider MetricsProvider) {
	if provider == nil {
		return
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
