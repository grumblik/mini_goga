package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	version  = "dev"
	cfgPath  string
	port     string
	serverHost string
	interval time.Duration
	timeout  time.Duration
	metricsAuth string
	allowedPorts string
	maxResponseSize int64

	httpClient *http.Client

	targetUp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "mini_goga_target_up", Help: "Whether the target is up (1) or down (0)."},
		[]string{"url"},
	)
	targetRespMS = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "mini_goga_target_response_ms", Help: "Response time in milliseconds."},
		[]string{"url"},
	)
	targetStatusCode = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "mini_goga_target_status_code", Help: "Status code one-hot. Label 'code' holds the HTTP status."},
		[]string{"url", "code"},
	)
	scrapeErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "mini_goga_scrape_errors_total", Help: "Total scrape errors per target."},
		[]string{"url"},
	)
	lastSuccessTS = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "mini_goga_last_success_timestamp", Help: "Unix timestamp of last successful check."},
		[]string{"url"},
	)
)

func init() {
	flag.StringVar(&cfgPath, "config", getenv("CONFIG", "config.cfg"), "path to targets file")
	flag.StringVar(&serverHost, "server-host", getenv("SERVER_HOST", "0.0.0.0"), "server host to listen on (default: 0.0.0.0)")
	flag.StringVar(&port, "port", getenv("SERVER_PORT", "9100"), "listen port")
	flag.DurationVar(&interval, "interval", getdur("INTERVAL", 15*time.Second), "probe interval")
	flag.DurationVar(&timeout, "timeout", getdur("TIMEOUT", 15*time.Second), "request timeout")
	flag.StringVar(&metricsAuth, "metrics-auth", getenv("METRICS_AUTH", ""), "basic auth for metrics endpoint (user:pass)")
	flag.StringVar(&allowedPorts, "allowed-ports", getenv("ALLOWED_PORTS", "80;443"), "allowed ports separated by semicolon (default: 80;443)")
	flag.Int64Var(&maxResponseSize, "max-response-size", getint64("MAX_RESPONSE_SIZE", 2*1024), "maximum response body size in bytes (default: 2KB)")
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
func getdur(env string, def time.Duration) time.Duration {
	if v := os.Getenv(env); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func getint64(env string, def int64) int64 {
	if v := os.Getenv(env); v != "" {
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i
		}
	}
	return def
}

// validateURL checks if URL is safe for monitoring
func validateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}

	// Only allow HTTP and HTTPS schemes
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}

	// Block private IP ranges and localhost
	host := u.Hostname()
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return fmt.Errorf("localhost access not allowed")
	}

	// Block private IP ranges
	ip := net.ParseIP(host)
	if ip != nil {
		if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			return fmt.Errorf("private IP access not allowed: %s", host)
		}
	}

	// Block common metadata endpoints
	if strings.Contains(host, "169.254.169.254") || strings.Contains(host, "metadata.google.internal") {
		return fmt.Errorf("metadata endpoint access not allowed")
	}

	// Allow only whitelisted ports - use environment variable or default list
	port := u.Port()
	if port != "" {
		var allowedPortsList []string
		
		if allowedPorts != "" {
			// Use custom allowed ports from environment variable
			allowedPortsList = strings.Split(allowedPorts, ";")
		} else {
			// Use default allowed ports (HTTP and HTTPS only)
			allowedPortsList = []string{"80", "443"}
		}
		
		// Check if port is in allowed list
		portAllowed := false
		for _, allowedPort := range allowedPortsList {
			allowedPort = strings.TrimSpace(allowedPort)
			if allowedPort != "" && port == allowedPort {
				portAllowed = true
				break
			}
		}
		
		if !portAllowed {
			return fmt.Errorf("port %s not in allowed list. Allowed ports: %s", port, allowedPorts)
		}
	}

	return nil
}

// authMiddleware provides basic authentication for metrics endpoint
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if metricsAuth == "" {
			next.ServeHTTP(w, r)
			return
		}
		
		user, pass, ok := r.BasicAuth()
		expectedUser, expectedPass, _ := strings.Cut(metricsAuth, ":")
		
		if !ok || user != expectedUser || pass != expectedPass {
			w.Header().Set("WWW-Authenticate", `Basic realm="mini_goga"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

func buildHTTPClient(timeout time.Duration) *http.Client {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		MaxIdleConns:          200,
		MaxIdleConnsPerHost:   20,
		IdleConnTimeout:       90 * time.Second,
		ForceAttemptHTTP2:     true,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
}

func readTargets(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []string
	sc := bufio.NewScanner(f)
	lineNum := 0
	for sc.Scan() {
		lineNum++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		// Validate URL for security
		if err := validateURL(line); err != nil {
			log.Printf("WARNING: Skipping invalid URL on line %d: %s - %v", lineNum, line, err)
			continue
		}
		
		out = append(out, line)
	}
	return out, sc.Err()
}

func probeOne(ctx context.Context, url string) (status int, ms int64, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, 0, err
	}

	// Add security headers
	req.Header.Set("User-Agent", "mini_goga/1.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	start := time.Now()
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()
	
	// Limit response body size to prevent DoS
	limitedReader := io.LimitReader(resp.Body, maxResponseSize)
	_, _ = io.Copy(io.Discard, limitedReader)

	elapsed := time.Since(start).Milliseconds()
	return resp.StatusCode, elapsed, nil
}

func setOneHotStatus(url string, code int) {
	for _, c := range []int{200, 301, 302, 400, 401, 403, 404, 500, 502, 503} {
		targetStatusCode.WithLabelValues(url, strconv.Itoa(c)).Set(0)
	}
	targetStatusCode.WithLabelValues(url, strconv.Itoa(code)).Set(1)
}

func runProbes(ctx context.Context, targets []string) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	worker := func(url string) {
		cctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		code, ms, err := probeOne(cctx, url)
		if err != nil {
			targetUp.WithLabelValues(url).Set(0)
			scrapeErrors.WithLabelValues(url).Inc()
			return
		}
		targetUp.WithLabelValues(url).Set(1)
		targetRespMS.WithLabelValues(url).Set(float64(ms))
		setOneHotStatus(url, code)
		if code >= 200 && code < 400 {
			lastSuccessTS.WithLabelValues(url).Set(float64(time.Now().Unix()))
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			var wg sync.WaitGroup
			wg.Add(len(targets))
			for _, url := range targets {
				u := url
				go func() {
					defer wg.Done()
					worker(u)
				}()
			}
			wg.Wait()
		}
	}
}

func main() {
	flag.Parse()

	httpClient = buildHTTPClient(timeout)

	targets, err := readTargets(cfgPath)
	if err != nil {
		log.Fatalf("read targets: %v", err)
	}
	if len(targets) == 0 {
		log.Printf("no targets found in %s", cfgPath)
	}

	prometheus.MustRegister(targetUp, targetRespMS, targetStatusCode, scrapeErrors, lastSuccessTS)

	mux := http.NewServeMux()
	mux.Handle("/metrics", authMiddleware(promhttp.Handler()))
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	srv := &http.Server{
		Addr:              serverHost + ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	idleConns := make(chan struct{})
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)
		<-ch
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
		close(idleConns)
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go runProbes(ctx, targets)

	log.Printf("mini_goga listening on :%s, scraping %d targets every %s, version=%s", port, len(targets), interval, version)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("http server: %v", err)
	}

	<-idleConns
	log.Println("shutdown complete")
}
