package api

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"cpa-usage/internal/poller"
	"cpa-usage/internal/quota"
	repodto "cpa-usage/internal/repository/dto"
	"cpa-usage/internal/service"
	"cpa-usage/internal/version"
	"github.com/gin-gonic/gin"
)

const appBasePathPlaceholder = "__APP_BASE_PATH__"
const manualSyncRateLimitWindow = time.Second

type syncLimiter struct {
	mu       sync.Mutex
	window   time.Duration
	lastSync time.Time
}

func (l *syncLimiter) allow(now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.lastSync.IsZero() && now.Sub(l.lastSync) < l.window {
		return false
	}
	l.lastSync = now
	return true
}

type StatusProvider interface {
	Status() poller.Status
}

type SyncRunner interface {
	SyncNow(ctx context.Context) error
}

type QuotaProvider interface {
	Check(context.Context, quota.CheckRequest) (quota.CheckResponse, error)
	GetCachedQuota(context.Context, quota.CacheRequest) (quota.CacheResponse, error)
	Refresh(context.Context, quota.RefreshRequest) (quota.RefreshResponse, error)
	GetRefreshTask(context.Context, string) (quota.RefreshTaskResponse, error)
}

type OptionalProviders struct {
	Analytics      service.AnalyticsProvider
	UsageIdentity  service.UsageIdentityProvider
	KeyAlias       service.KeyAliasProvider
	Quota          QuotaProvider
	RollupBackfill service.RollupBackfillStatusProvider
}

type syncUserMessageError interface {
	UserMessage() string
}

func NewRouter(
	staticFS fs.FS,
	statusProvider StatusProvider,
	usageProvider service.UsageProvider,
	pricingProvider service.PricingProvider,
	authConfig AuthConfig,
	authHandler *authHandler,
	basePath string,
	optionalProviders ...OptionalProviders,
) *gin.Engine {
	router := gin.New()
	if err := router.SetTrustedProxies(authConfig.TrustedProxies); err != nil {
		panic(err)
	}
	router.Use(gin.Recovery())

	appGroup := router.Group(basePath)
	registerHealthRoutes(appGroup)

	apiV1 := appGroup.Group("/api/v1")
	apiV1.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	authGroup := apiV1.Group("/auth")
	if authHandler == nil {
		authHandler = NewAuthHandler(authConfig, nil)
	}
	authHandler.registerRoutes(authGroup)

	var usageIdentityProvider service.UsageIdentityProvider
	var analyticsProvider service.AnalyticsProvider
	var keyAliasProvider service.KeyAliasProvider
	var quotaProvider QuotaProvider
	var rollupBackfillProvider service.RollupBackfillStatusProvider
	if len(optionalProviders) > 0 {
		analyticsProvider = optionalProviders[0].Analytics
		usageIdentityProvider = optionalProviders[0].UsageIdentity
		keyAliasProvider = optionalProviders[0].KeyAlias
		quotaProvider = optionalProviders[0].Quota
		rollupBackfillProvider = optionalProviders[0].RollupBackfill
	}

	protected := apiV1.Group("")
	protected.Use(authHandler.middleware())
	registerStatusRoutes(protected, statusProvider, rollupBackfillProvider)
	registerUpdateRoutes(protected, nil)
	registerSyncRoutes(protected, statusProvider, &syncLimiter{window: manualSyncRateLimitWindow})
	registerUsageOverviewRoute(protected, usageProvider)
	registerUsageAnalysisRoute(protected, usageProvider)
	registerAnalyticsRoutes(protected, analyticsProvider)
	registerUsageEventsRoute(protected, usageProvider, usageIdentityProvider, keyAliasProvider)
	registerUsageIdentityRoutes(protected, usageIdentityProvider, keyAliasProvider)
	registerPricingRoutes(protected, pricingProvider)
	registerQuotaRoutes(protected, quotaProvider)

	if staticFS != nil {
		if indexFile, err := staticFS.Open("index.html"); err == nil {
			_ = indexFile.Close()
			httpFS := http.FS(staticFS)
			serveIndex := func(c *gin.Context) {
				indexHTML, err := renderIndexHTML(staticFS, basePath)
				if err != nil {
					c.Status(http.StatusNotFound)
					return
				}
				setHTMLCacheHeaders(c)
				c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
			}
			serveAsset := func(c *gin.Context) {
				assetPath := "assets/" + strings.TrimPrefix(c.Param("filepath"), "/")
				if assetFile, err := staticFS.Open(assetPath); err == nil {
					_ = assetFile.Close()
					setStaticAssetCacheHeaders(c)
					c.FileFromFS(assetPath, httpFS)
					return
				}
				c.Status(http.StatusNotFound)
			}

			redirectBasePath := func(c *gin.Context) {
				target := basePath + "/"
				if c.Request.URL.RawQuery != "" {
					target += "?" + c.Request.URL.RawQuery
				}
				c.Redirect(http.StatusPermanentRedirect, target)
			}
			if basePath != "" {
				appGroup.GET("", redirectBasePath)
				appGroup.HEAD("", redirectBasePath)
			}
			appGroup.GET("/", serveIndex)
			appGroup.GET("/index.html", serveIndex)
			appGroup.HEAD("/index.html", serveIndex)
			appGroup.GET("/assets/*filepath", serveAsset)
			appGroup.HEAD("/assets/*filepath", serveAsset)
			router.NoRoute(func(c *gin.Context) {
				requestPath, ok := stripBasePath(basePath, c.Request.URL.Path)
				if !ok {
					c.Status(http.StatusNotFound)
					return
				}
				if strings.HasPrefix(requestPath, "/api/") {
					c.Status(http.StatusNotFound)
					return
				}

				if assetPath, ok := staticAssetPath(requestPath); ok {
					if assetFile, err := staticFS.Open(assetPath); err == nil {
						_ = assetFile.Close()
						setStaticAssetCacheHeaders(c)
						c.FileFromFS(assetPath, httpFS)
						return
					}
				}

				serveIndex(c)
			})
		}
	}

	return router
}

func setHTMLCacheHeaders(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
}

func setStaticAssetCacheHeaders(c *gin.Context) {
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
}

func renderIndexHTML(staticFS fs.FS, basePath string) ([]byte, error) {
	indexFile, err := staticFS.Open("index.html")
	if err != nil {
		return nil, err
	}
	defer indexFile.Close()
	indexHTML, err := io.ReadAll(indexFile)
	if err != nil {
		return nil, err
	}

	return bytes.ReplaceAll(
		indexHTML,
		[]byte(strconv.Quote(appBasePathPlaceholder)),
		[]byte(strconv.Quote(basePath)),
	), nil
}

func cleanURLPath(requestPath string) string {
	cleaned := path.Clean(requestPath)
	if cleaned == "." {
		return "/"
	}
	if !strings.HasPrefix(cleaned, "/") {
		return "/" + cleaned
	}
	return cleaned
}

func staticAssetPath(requestPath string) (string, bool) {
	cleaned := cleanURLPath(requestPath)
	if strings.Contains(cleaned, "\\") {
		return "", false
	}
	relPath := strings.TrimPrefix(cleaned, "/")
	if relPath == "" {
		return "", false
	}
	return relPath, true
}

func stripBasePath(basePath, requestPath string) (string, bool) {
	cleaned := cleanURLPath(requestPath)
	if basePath == "" {
		return cleaned, true
	}
	if cleaned == basePath {
		return "/", true
	}
	if !strings.HasPrefix(cleaned, basePath+"/") {
		return "", false
	}
	trimmed := strings.TrimPrefix(cleaned, basePath)
	if trimmed == "" {
		return "/", true
	}
	return trimmed, true
}

type statusResponse struct {
	Running        bool                         `json:"running"`
	SyncRunning    bool                         `json:"sync_running"`
	Timezone       string                       `json:"timezone"`
	Version        string                       `json:"version"`
	RollupBackfill rollupBackfillStatusResponse `json:"rollup_backfill"`
	LastRunAt      *time.Time                   `json:"last_run_at,omitempty"`
	LastError      string                       `json:"last_error,omitempty"`
	LastWarning    string                       `json:"last_warning,omitempty"`
	LastStatus     string                       `json:"last_status,omitempty"`
}

type syncStatusResponse struct {
	Running     bool       `json:"running"`
	SyncRunning bool       `json:"sync_running"`
	Timezone    string     `json:"timezone"`
	Version     string     `json:"version"`
	LastRunAt   *time.Time `json:"last_run_at,omitempty"`
	LastError   string     `json:"last_error,omitempty"`
	LastWarning string     `json:"last_warning,omitempty"`
	LastStatus  string     `json:"last_status,omitempty"`
}

type rollupBackfillStatusResponse struct {
	Status             string     `json:"status"`
	TargetBucketStart  *time.Time `json:"target_bucket_start,omitempty"`
	CoveredBucketStart *time.Time `json:"covered_bucket_start,omitempty"`
	StartedAt          *time.Time `json:"started_at,omitempty"`
	CompletedAt        *time.Time `json:"completed_at,omitempty"`
	FailedAt           *time.Time `json:"failed_at,omitempty"`
	LastError          string     `json:"last_error,omitempty"`
}

func registerStatusRoutes(router gin.IRoutes, statusProvider StatusProvider, rollupBackfillProvider service.RollupBackfillStatusProvider) {
	router.GET("/status", func(c *gin.Context) {
		rollupBackfillStatus, err := loadRollupBackfillStatus(c.Request.Context(), rollupBackfillProvider)
		if err != nil {
			writeInternalError(c, "rollup backfill status is unavailable", err)
			return
		}
		if statusProvider == nil {
			c.JSON(http.StatusOK, buildStatusResponse(poller.Status{}, rollupBackfillStatus))
			return
		}

		c.JSON(http.StatusOK, buildStatusResponse(statusProvider.Status(), rollupBackfillStatus))
	})
}

func manualSyncErrorMessage(err error) string {
	var userMessage syncUserMessageError
	if errors.As(err, &userMessage) && userMessage.UserMessage() != "" {
		return userMessage.UserMessage()
	}
	return "manual sync failed"
}

func registerSyncRoutes(router gin.IRoutes, statusProvider StatusProvider, limiter *syncLimiter) {
	router.POST("/sync", func(c *gin.Context) {
		if limiter != nil && !limiter.allow(time.Now()) {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "sync rate limit exceeded"})
			return
		}

		syncRunner, ok := statusProvider.(SyncRunner)
		if !ok || syncRunner == nil {
			writeInternalError(c, "sync runner is not configured", nil)
			return
		}

		if err := syncRunner.SyncNow(c.Request.Context()); err != nil {
			if errors.Is(err, poller.ErrSyncAlreadyRunning) {
				c.JSON(http.StatusConflict, gin.H{"error": "sync already running"})
				return
			}
			slog.Error("manual sync failed", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": manualSyncErrorMessage(err)})
			return
		}

		if statusProvider, ok := syncRunner.(StatusProvider); ok {
			c.JSON(http.StatusOK, buildSyncStatusResponse(statusProvider.Status()))
			return
		}
		c.JSON(http.StatusOK, gin.H{"sync_running": false})
	})
}

func loadRollupBackfillStatus(ctx context.Context, provider service.RollupBackfillStatusProvider) (repodto.RollupBackfillStatus, error) {
	if provider == nil {
		return repodto.RollupBackfillStatus{Status: repodto.RollupBackfillStatusPending}, nil
	}
	status, err := provider.GetRollupBackfillStatus(ctx)
	if err != nil {
		return repodto.RollupBackfillStatus{}, err
	}
	return repodto.NormalizeRollupBackfillStatus(status), nil
}

func buildStatusResponse(status poller.Status, rollupBackfill repodto.RollupBackfillStatus) statusResponse {
	response := statusResponse{
		Running:        status.Running,
		SyncRunning:    status.SyncRunning,
		Timezone:       time.Local.String(),
		Version:        version.Version,
		RollupBackfill: buildRollupBackfillStatusResponse(rollupBackfill),
		LastError:      status.LastError,
		LastWarning:    status.LastWarning,
		LastStatus:     status.LastStatus,
	}
	if !status.LastRunAt.IsZero() {
		lastRunAt := status.LastRunAt.UTC()
		response.LastRunAt = &lastRunAt
	}
	return response
}

func buildSyncStatusResponse(status poller.Status) syncStatusResponse {
	response := syncStatusResponse{
		Running:     status.Running,
		SyncRunning: status.SyncRunning,
		Timezone:    time.Local.String(),
		Version:     version.Version,
		LastError:   status.LastError,
		LastWarning: status.LastWarning,
		LastStatus:  status.LastStatus,
	}
	if !status.LastRunAt.IsZero() {
		lastRunAt := status.LastRunAt.UTC()
		response.LastRunAt = &lastRunAt
	}
	return response
}

func buildRollupBackfillStatusResponse(status repodto.RollupBackfillStatus) rollupBackfillStatusResponse {
	status = repodto.NormalizeRollupBackfillStatus(status)
	return rollupBackfillStatusResponse{
		Status:             status.Status,
		TargetBucketStart:  status.TargetBucketStart,
		CoveredBucketStart: status.CoveredBucketStart,
		StartedAt:          status.StartedAt,
		CompletedAt:        status.CompletedAt,
		FailedAt:           status.FailedAt,
		LastError:          status.LastError,
	}
}
