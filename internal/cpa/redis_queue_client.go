package cpa

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

var ErrRedisQueueAuth = errors.New("redis queue auth failed")

type redisQueueSyncMode string

type redisQueueEffectState uint8

const (
	redisQueueSyncModeRedis redisQueueSyncMode = "redis"
	redisQueueSyncModeHTTP  redisQueueSyncMode = "http"

	redisQueueEffectNotStarted redisQueueEffectState = iota
	redisQueueEffectMayHaveStarted
)

type redisQueuePopResult struct {
	messages    []string
	err         error
	effectState redisQueueEffectState
}

type RedisQueueClient struct {
	address       string
	managementKey string
	timeout       time.Duration
	queueKey      string
	batchSize     int
	httpClient    *Client
	mu            sync.Mutex
	syncMode      redisQueueSyncMode
	dial          func(ctx context.Context, network, addr string) (net.Conn, error)
}

type RedisQueueOptions struct {
	BaseURL       string
	RedisAddr     string
	ManagementKey string
	Timeout       time.Duration
	QueueKey      string
	BatchSize     int
	TLS           bool
	TLSSkipVerify bool
}

func NewRedisQueueClientWithOptions(opts RedisQueueOptions) *RedisQueueClient {
	addr, useTLS := redisQueueAddress(opts.BaseURL, opts.RedisAddr)
	if opts.TLS {
		useTLS = true
	}
	netDialer := &net.Dialer{Timeout: opts.Timeout}
	dial := netDialer.DialContext
	if useTLS {
		tlsDialer := &tls.Dialer{NetDialer: netDialer, Config: &tls.Config{InsecureSkipVerify: opts.TLSSkipVerify}}
		dial = func(ctx context.Context, network, addr string) (net.Conn, error) {
			if opts.Timeout > 0 {
				deadline := time.Now().Add(opts.Timeout)
				if existing, ok := ctx.Deadline(); !ok || deadline.Before(existing) {
					var cancel context.CancelFunc
					ctx, cancel = context.WithDeadline(ctx, deadline)
					defer cancel()
				}
			}
			return tlsDialer.DialContext(ctx, network, addr)
		}
	}
	return &RedisQueueClient{
		address:       addr,
		managementKey: strings.TrimSpace(opts.ManagementKey),
		timeout:       opts.Timeout,
		queueKey:      strings.TrimSpace(opts.QueueKey),
		batchSize:     opts.BatchSize,
		httpClient:    NewClient(opts.BaseURL, opts.ManagementKey, opts.Timeout, opts.TLSSkipVerify),
		dial:          dial,
	}
}

func (c *RedisQueueClient) PopUsage(ctx context.Context) ([]string, error) {
	if c == nil {
		return nil, fmt.Errorf("redis queue client is nil")
	}
	if c.queueKey == "" {
		return nil, fmt.Errorf("redis queue key is required")
	}
	if c.batchSize <= 0 {
		return nil, fmt.Errorf("redis queue batch size must be positive")
	}
	if c.timeout <= 0 {
		return nil, fmt.Errorf("redis queue timeout must be positive")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	switch c.syncMode {
	case redisQueueSyncModeRedis:
		result := c.popUsageOverRedis(ctx)
		return result.messages, result.err
	case redisQueueSyncModeHTTP:
		return c.popUsageOverHTTP(ctx)
	}

	result := c.popUsageOverRedis(ctx)
	if result.err == nil {
		c.syncMode = redisQueueSyncModeRedis
		slog.Info("usage queue sync used redis protocol", "message_count", len(result.messages))
		return result.messages, nil
	}
	slog.Error("usage queue sync failed to use redis protocol", "redis_error", result.err.Error())
	if errors.Is(result.err, context.Canceled) || errors.Is(result.err, context.DeadlineExceeded) {
		return nil, fmt.Errorf("usage queue sync stopped: %w", result.err)
	}
	if result.effectState == redisQueueEffectMayHaveStarted {
		return nil, fmt.Errorf("usage queue sync failed after redis queue pop may have started: %w", result.err)
	}
	if !c.canFallbackToHTTP() {
		return nil, fmt.Errorf("usage queue sync failed: %w; http usage queue fallback not possible", result.err)
	}

	messages, fallbackErr := c.popUsageOverHTTP(ctx)
	if fallbackErr != nil {
		return nil, fmt.Errorf("usage queue sync failed: %w; http usage queue fallback failed: %w", result.err, fallbackErr)
	}
	c.syncMode = redisQueueSyncModeHTTP
	slog.Info("usage queue sync used http protocol", "message_count", len(messages))
	return messages, nil
}

func (c *RedisQueueClient) popUsageOverRedis(ctx context.Context) redisQueuePopResult {
	operationCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	conn, reader, stopCancellation, err := c.openAuthenticatedConnection(operationCtx)
	if err != nil {
		return redisQueuePopResult{err: err}
	}
	defer stopCancellation()
	defer conn.Close()

	result := redisQueuePopResult{effectState: redisQueueEffectMayHaveStarted}
	if err := writeRESPCommand(conn, cpaManagementRedisPopCommand, c.queueKey, strconv.Itoa(c.batchSize)); err != nil {
		result.err = fmt.Errorf("write redis queue pop command: %w", connectionContextError(operationCtx, err))
		return result
	}
	popResponse, err := readRESPValue(reader)
	if err != nil {
		result.err = fmt.Errorf("read redis queue pop response: %w", connectionContextError(operationCtx, err))
		return result
	}
	if popResponse.err != "" {
		result.err = fmt.Errorf("redis queue pop failed: %s", popResponse.err)
		return result
	}
	result.messages = popResponse.strings()
	return result
}

func (c *RedisQueueClient) canFallbackToHTTP() bool {
	return c != nil && c.httpClient != nil && strings.TrimSpace(c.httpClient.baseURL) != ""
}

func (c *RedisQueueClient) popUsageOverHTTP(ctx context.Context) ([]string, error) {
	if c == nil || c.httpClient == nil {
		return nil, fmt.Errorf("redis queue http client is nil")
	}
	result, err := c.httpClient.FetchUsageQueue(ctx, c.batchSize)
	if err != nil {
		return nil, fmt.Errorf("fetch usage queue over http: %w", err)
	}
	messages := make([]string, 0, len(result.Payload))
	for _, item := range result.Payload {
		trimmed := strings.TrimSpace(string(item))
		if trimmed == "" || trimmed == "null" {
			continue
		}
		messages = append(messages, trimmed)
	}
	return messages, nil
}

func (c *RedisQueueClient) openAuthenticatedConnection(ctx context.Context) (net.Conn, *bufio.Reader, func(), error) {
	if c == nil {
		return nil, nil, nil, fmt.Errorf("redis queue client is nil")
	}
	if c.address == "" {
		return nil, nil, nil, fmt.Errorf("redis queue address is required")
	}
	if c.managementKey == "" {
		return nil, nil, nil, fmt.Errorf("redis queue management key is required")
	}

	conn, err := c.dial(ctx, cpaManagementRedisNetwork, c.address)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("connect redis queue: %w", connectionContextError(ctx, err))
	}
	stopContextWatch := context.AfterFunc(ctx, func() {
		_ = conn.SetDeadline(time.Now())
	})
	stopCancellation := func() {
		stopContextWatch()
	}
	closeConnection := func() {
		stopCancellation()
		_ = conn.Close()
	}

	reader := bufio.NewReader(conn)
	if err := writeRESPCommand(conn, cpaManagementRedisAuthCommand, c.managementKey); err != nil {
		closeConnection()
		return nil, nil, nil, fmt.Errorf("write redis queue auth command: %w", connectionContextError(ctx, err))
	}
	authResponse, err := readRESPValue(reader)
	if err != nil {
		closeConnection()
		return nil, nil, nil, fmt.Errorf("read redis queue auth response: %w", connectionContextError(ctx, err))
	}
	if authResponse.err != "" {
		closeConnection()
		return nil, nil, nil, fmt.Errorf("%w: %s", ErrRedisQueueAuth, authResponse.err)
	}
	return conn, reader, stopCancellation, nil
}

func connectionContextError(ctx context.Context, err error) error {
	if contextErr := ctx.Err(); contextErr != nil {
		return contextErr
	}
	return err
}

func redisQueueAddress(baseURL, redisQueueAddr string) (string, bool) {
	override := strings.TrimSpace(redisQueueAddr)
	if override != "" {
		if parsed, err := url.Parse(override); err == nil && parsed.Host != "" {
			return parsed.Host, parsed.Scheme == "rediss"
		}
		return override, false
	}
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		return "", false
	}
	parsed, err := url.Parse(trimmed)
	if err == nil && parsed.Host != "" {
		useTLS := parsed.Scheme == "https"
		if parsed.Port() != "" {
			return parsed.Host, useTLS
		}
		return net.JoinHostPort(parsed.Hostname(), ManagementRedisDefaultPort), useTLS
	}
	trimmed = strings.TrimPrefix(strings.TrimPrefix(trimmed, "http://"), "https://")
	if _, _, err := net.SplitHostPort(trimmed); err == nil {
		return trimmed, false
	}
	return net.JoinHostPort(trimmed, ManagementRedisDefaultPort), false
}

func writeRESPCommand(writer io.Writer, parts ...string) error {
	if _, err := fmt.Fprintf(writer, "*%d\r\n", len(parts)); err != nil {
		return err
	}
	for _, part := range parts {
		if _, err := fmt.Fprintf(writer, "$%d\r\n%s\r\n", len(part), part); err != nil {
			return err
		}
	}
	return nil
}

type respValue struct {
	simple string
	bulk   *string
	array  []respValue
	err    string
	nil    bool
}

func (v respValue) strings() []string {
	if v.nil {
		return nil
	}
	if v.bulk != nil {
		return []string{*v.bulk}
	}
	if len(v.array) == 0 {
		return nil
	}
	items := make([]string, 0, len(v.array))
	for _, item := range v.array {
		if item.bulk != nil {
			items = append(items, *item.bulk)
		}
	}
	return items
}

func readRESPValue(reader *bufio.Reader) (respValue, error) {
	prefix, err := reader.ReadByte()
	if err != nil {
		return respValue{}, err
	}
	switch prefix {
	case '+':
		line, err := readRESPLine(reader)
		return respValue{simple: line}, err
	case '-':
		line, err := readRESPLine(reader)
		return respValue{err: line}, err
	case '$':
		return readRESPBulk(reader)
	case '*':
		return readRESPArray(reader)
	default:
		return respValue{}, fmt.Errorf("unexpected RESP prefix %q", prefix)
	}
}

func readRESPBulk(reader *bufio.Reader) (respValue, error) {
	line, err := readRESPLine(reader)
	if err != nil {
		return respValue{}, err
	}
	size, err := strconv.Atoi(line)
	if err != nil {
		return respValue{}, fmt.Errorf("parse bulk size: %w", err)
	}
	if size < 0 {
		return respValue{nil: true}, nil
	}
	buf := make([]byte, size+2)
	if _, err := io.ReadFull(reader, buf); err != nil {
		return respValue{}, err
	}
	value := string(buf[:size])
	return respValue{bulk: &value}, nil
}

func readRESPArray(reader *bufio.Reader) (respValue, error) {
	line, err := readRESPLine(reader)
	if err != nil {
		return respValue{}, err
	}
	count, err := strconv.Atoi(line)
	if err != nil {
		return respValue{}, fmt.Errorf("parse array size: %w", err)
	}
	if count < 0 {
		return respValue{nil: true}, nil
	}
	items := make([]respValue, 0, count)
	for range count {
		item, err := readRESPValue(reader)
		if err != nil {
			return respValue{}, err
		}
		items = append(items, item)
	}
	return respValue{array: items}, nil
}

func readRESPLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r"), nil
}
