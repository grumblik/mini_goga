package main

import (
	"bufio"
	"context"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
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
        version = "dev"
	cfgPath  string
	port     string
	interval time.Duration
	timeout  time.Duration

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
	flag.StringVar(&port, "port", getenv("SERVER_PORT", "9100"), "listen port")
	flag.DurationVar(&interval, "interval", getdur("INTERVAL", 15*time.Second), "probe interval")
	flag.DurationVar(&timeout, "timeout", getdur("TIMEOUT", 15*time.Second), "request timeout")
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
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
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

	start := time.Now()
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

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
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	srv := &http.Server{
		Addr:              ":" + port,
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

	log.Printf("mini_goga listening on :%s, scraping %d targets every %s", port, len(targets), interval)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("http server: %v", err)
	}
	<-idleConns
	log.Println("shutdown complete")
}
