package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"faviconsvc/internal/cache"
	"faviconsvc/internal/fetch"
	"faviconsvc/internal/handler"
	"faviconsvc/pkg/logger"
	"faviconsvc/pkg/metrics"
	"faviconsvc/pkg/ratelimit"
)

var (
	addrFlag        string
	portFlag        int
	cacheDir        string
	cacheTTL        time.Duration
	browserMaxAge   time.Duration
	cdnSMaxAge      time.Duration
	useETag         bool
	janitorInterval time.Duration
	maxCacheSize    int64
	showHelp        bool
	logLevel        string
	// Rate limiting
	rateLimit       int
	rateLimitBurst  int
	ipRateLimit     int
	ipRateLimitBurst int
)

func main() {
	parseFlags()

	if showHelp {
		flag.Usage()
		return
	}

	// Initialize logger
	initLogger()

	// Initialize fetch client
	fetch.InitHTTPClient()

	// Setup cache
	cacheManager := cache.New(cacheDir, cacheTTL)
	if err := cacheManager.EnsureDirs(); err != nil {
		logger.Error("Failed to create cache directories: %v", err)
		os.Exit(1)
	}

	// Resolve effective cache headers
	effectiveBrowserMaxAge := browserMaxAge
	if effectiveBrowserMaxAge <= 0 {
		effectiveBrowserMaxAge = cacheTTL
	}
	effectiveCDNSMaxAge := cdnSMaxAge
	if effectiveCDNSMaxAge <= 0 {
		effectiveCDNSMaxAge = effectiveBrowserMaxAge
	}

	// Setup rate limiter
	var rateLimiter *ratelimit.Limiter
	if rateLimit > 0 || ipRateLimit > 0 {
		// Set default burst values
		if rateLimitBurst == 0 && rateLimit > 0 {
			rateLimitBurst = rateLimit * 2
		}
		if ipRateLimitBurst == 0 && ipRateLimit > 0 {
			ipRateLimitBurst = ipRateLimit * 2
		}
		
		rateLimiter = ratelimit.NewLimiter(rateLimit, rateLimitBurst, ipRateLimit, ipRateLimitBurst)
		
		// Log rate limiting configuration
		if rateLimit > 0 && ipRateLimit > 0 {
			logger.Info("Rate limiting enabled: global=%d/s (burst=%d), ip=%d/s (burst=%d)",
				rateLimit, rateLimitBurst, ipRateLimit, ipRateLimitBurst)
		} else if rateLimit > 0 {
			logger.Info("Rate limiting enabled: global=%d/s (burst=%d), ip=unlimited",
				rateLimit, rateLimitBurst)
		} else {
			logger.Info("Rate limiting enabled: global=unlimited, ip=%d/s (burst=%d)",
				ipRateLimit, ipRateLimitBurst)
		}
	} else {
		logger.Info("Rate limiting disabled (unlimited requests)")
	}

	// Setup HTTP handler
	handlerCfg := handler.NewConfig(
		cacheManager,
		effectiveBrowserMaxAge,
		effectiveCDNSMaxAge,
		useETag,
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/favicons", handler.FaviconHandler(handlerCfg))
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/metrics", metrics.Get().Handler())

	addr := resolveListenAddr()

	// Build middleware chain: rate limit -> metrics -> logging
	var finalHandler http.Handler = mux
	if rateLimiter != nil {
		finalHandler = ratelimit.Middleware(rateLimiter)(finalHandler)
	}
	finalHandler = metrics.Middleware(finalHandler)
	finalHandler = logMiddleware(finalHandler)

	srv := &http.Server{
		Addr:              addr,
		Handler:           finalHandler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Start server
	go func() {
		printAddr := addr
		if strings.HasPrefix(addr, ":") {
			printAddr = "localhost" + addr
		}
		logger.Info("Starting favicon service on http://%s", printAddr)
		logger.Info("Cache directory: %s (TTL: %v)", cacheDir, cacheTTL)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("Server error: %v", err)
			os.Exit(1)
		}
	}()

	// Start janitor if enabled
	var janCtx context.Context
	var janCancel context.CancelFunc
	if janitorInterval > 0 {
		janCtx, janCancel = context.WithCancel(context.Background())
		go cache.RunJanitor(janCtx, janitorInterval, cacheDir, cacheTTL, maxCacheSize)
	}

	// Wait for shutdown signal
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	logger.Info("Shutting down gracefully...")

	if janCancel != nil {
		janCancel()
	}

	if rateLimiter != nil {
		rateLimiter.Stop()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)

	logger.Info("Server stopped")
}

func parseFlags() {
	flag.StringVar(&addrFlag, "addr", "", "listen address, e.g. ':9090' or '0.0.0.0:9090'")
	flag.IntVar(&portFlag, "port", 0, "port number (alternative to -addr)")
	flag.StringVar(&cacheDir, "cache-dir", "./cache", "directory for disk cache")
	flag.DurationVar(&cacheTTL, "cache-ttl", 24*time.Hour, "TTL for disk cache entries")
	flag.DurationVar(&browserMaxAge, "browser-max-age", 0, "Cache-Control: max-age (default=cache-ttl)")
	flag.DurationVar(&cdnSMaxAge, "cdn-smax-age", 0, "Cache-Control: s-maxage (default=browser-max-age)")
	flag.BoolVar(&useETag, "etag", true, "Enable ETag/If-None-Match")
	flag.DurationVar(&janitorInterval, "janitor-interval", 30*time.Minute, "Purge expired cache (0=disabled)")
	flag.Int64Var(&maxCacheSize, "max-cache-size-bytes", 0, "Max cache size in bytes (0=unlimited)")
	flag.StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	flag.IntVar(&rateLimit, "rate-limit", 0, "Global requests/second (0=unlimited)")
	flag.IntVar(&rateLimitBurst, "rate-limit-burst", 0, "Global burst capacity (0=auto: rate*2)")
	flag.IntVar(&ipRateLimit, "ip-rate-limit", 0, "Requests/second per IP (0=unlimited)")
	flag.IntVar(&ipRateLimitBurst, "ip-rate-limit-burst", 0, "Per-IP burst capacity (0=auto: rate*2)")
	flag.BoolVar(&showHelp, "help", false, "Show help and exit")
	flag.Parse()
}

func initLogger() {
	var level logger.Level
	switch strings.ToLower(logLevel) {
	case "debug":
		level = logger.DEBUG
	case "warn":
		level = logger.WARN
	case "error":
		level = logger.ERROR
	default:
		level = logger.INFO
	}
	logger.SetLevel(level)
	logger.Init()
}

func resolveListenAddr() string {
	if addrFlag != "" {
		return addrFlag
	}
	if portFlag != 0 {
		return ":" + strconv.Itoa(portFlag)
	}
	if p := os.Getenv("PORT"); p != "" {
		return ":" + p
	}
	return ":9090"
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rw, r)
		duration := time.Since(start)
		logger.Info("%s %s %d %v", r.Method, r.URL.String(), rw.status, duration)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (w *statusRecorder) WriteHeader(statusCode int) {
	w.status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}
