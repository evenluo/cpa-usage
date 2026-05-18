package api

import (
	"crypto/subtle"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"cpa-usage/internal/auth"
	"github.com/gin-gonic/gin"
)

const defaultSessionCookieName = "cpa_usage_session"

const maxFailedLoginAttempts = 5
const failedLoginAttemptWindow = 15 * time.Minute

type AuthConfig struct {
	Enabled             bool
	LoginPassword       string
	SharedBearerToken   string
	SessionTTL          time.Duration
	BasePath            string
	SessionCookieName   string
	SessionCookieDomain string
	SessionCookiePath   string
	TrustedProxies      []string
}

type authHandler struct {
	config   AuthConfig
	sessions *auth.SessionManager

	mu             sync.Mutex
	failedAttempts map[string]failedLoginAttempt
	now            func() time.Time
}

type failedLoginAttempt struct {
	count        int
	lastFailedAt time.Time
}

type loginRequest struct {
	Password string `json:"password"`
}

type sessionResponse struct {
	Authenticated bool `json:"authenticated"`
}

func NewAuthHandler(config AuthConfig, sessions *auth.SessionManager) *authHandler {
	return &authHandler{config: config, sessions: sessions, failedAttempts: make(map[string]failedLoginAttempt), now: time.Now}
}

func (h *authHandler) registerRoutes(router gin.IRoutes) {
	router.GET("/session", h.getSession)
	router.POST("/login", h.login)
	router.POST("/logout", h.logout)
}

func (h *authHandler) middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h == nil || !h.config.Enabled {
			c.Next()
			return
		}
		if h.sessions == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		if !h.validateRequest(c) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		c.Next()
	}
}

func (h *authHandler) getSession(c *gin.Context) {
	if h == nil || !h.config.Enabled {
		c.JSON(http.StatusOK, sessionResponse{Authenticated: true})
		return
	}
	if h.sessions == nil {
		c.JSON(http.StatusOK, sessionResponse{Authenticated: false})
		return
	}

	c.JSON(http.StatusOK, sessionResponse{Authenticated: h.validateRequest(c)})
}

func (h *authHandler) login(c *gin.Context) {
	if h == nil || !h.config.Enabled {
		c.Status(http.StatusNoContent)
		return
	}
	if h.sessions == nil {
		writeInternalError(c, "session manager is not configured", nil)
		return
	}

	var request loginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	clientKey := loginClientKey(c)
	passwordMatches := h.passwordMatches(request.Password)
	if h.tooManyFailedAttempts(clientKey) && !passwordMatches {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many failed login attempts"})
		return
	}

	if !passwordMatches {
		h.recordFailedAttempt(clientKey)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid password"})
		return
	}
	h.clearFailedAttempts(clientKey)

	token, expiresAt, err := h.sessions.Create()
	if err != nil {
		writeInternalError(c, "create auth session failed", err)
		return
	}

	secure := c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     h.sessionCookieName(),
		Value:    token,
		Path:     h.sessionCookiePath(),
		Domain:   h.config.SessionCookieDomain,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
	})
	c.Status(http.StatusNoContent)
}

func (h *authHandler) logout(c *gin.Context) {
	if h == nil || !h.config.Enabled {
		c.Status(http.StatusNoContent)
		return
	}
	if h.sessions != nil {
		if token, err := c.Cookie(h.sessionCookieName()); err == nil {
			h.sessions.Delete(token)
		}
	}
	clearSessionCookie(c, h.sessionCookieName(), h.sessionCookiePath(), h.config.SessionCookieDomain)
	c.Status(http.StatusNoContent)
}

func (h *authHandler) tooManyFailedAttempts(key string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := h.now()
	h.pruneExpiredFailedAttemptsLocked(now)
	return h.failedAttempts[key].count >= maxFailedLoginAttempts
}

func (h *authHandler) recordFailedAttempt(key string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := h.now()
	h.pruneExpiredFailedAttemptsLocked(now)
	attempt := h.failedAttempts[key]
	attempt.count++
	attempt.lastFailedAt = now
	h.failedAttempts[key] = attempt
}

func (h *authHandler) clearFailedAttempts(key string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.failedAttempts, key)
}

func (h *authHandler) pruneExpiredFailedAttemptsLocked(now time.Time) {
	for key, attempt := range h.failedAttempts {
		if now.Sub(attempt.lastFailedAt) > failedLoginAttemptWindow {
			delete(h.failedAttempts, key)
		}
	}
}

func loginClientKey(c *gin.Context) string {
	clientIP := c.ClientIP()
	if clientIP != "" {
		return clientIP
	}
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return c.Request.RemoteAddr
}

func (h *authHandler) validateRequest(c *gin.Context) bool {
	if h == nil {
		return false
	}
	if h.validateBearerToken(c.GetHeader("Authorization")) {
		return true
	}
	if h.sessions == nil {
		return false
	}
	token, err := c.Cookie(h.sessionCookieName())
	return err == nil && h.sessions.Validate(token)
}

func (h *authHandler) passwordMatches(password string) bool {
	if h == nil {
		return false
	}
	if h.config.LoginPassword != "" && subtle.ConstantTimeCompare([]byte(password), []byte(h.config.LoginPassword)) == 1 {
		return true
	}
	return h.config.SharedBearerToken != "" && subtle.ConstantTimeCompare([]byte(password), []byte(h.config.SharedBearerToken)) == 1
}

func (h *authHandler) validateBearerToken(header string) bool {
	if h == nil || h.config.SharedBearerToken == "" {
		return false
	}
	token := bearerToken(header)
	return token != "" && subtle.ConstantTimeCompare([]byte(token), []byte(h.config.SharedBearerToken)) == 1
}

func bearerToken(header string) string {
	const prefix = "Bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}

func (h *authHandler) sessionCookieName() string {
	if h == nil || h.config.SessionCookieName == "" {
		return defaultSessionCookieName
	}
	return h.config.SessionCookieName
}

func (h *authHandler) sessionCookiePath() string {
	if h == nil {
		return "/"
	}
	if h.config.SessionCookiePath != "" {
		return h.config.SessionCookiePath
	}
	if h.config.BasePath != "" {
		return h.config.BasePath
	}
	return "/"
}

func clearSessionCookie(c *gin.Context, name, cookiePath, domain string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     cookiePath,
		Domain:   domain,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
}
