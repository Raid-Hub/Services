// This code was created by cbro

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/netip"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"

	"raidhub/lib/env"
	"raidhub/lib/utils/logging"
)

const (
	DEFAULT_INTERFACE = "enp1s0"
	DEFAULT_V6_N      = 16
	USER_AGENT        = "RaidHub-Zeus/1.0.0 (contact=admin@raidhub.io)"
)

var (
	port          = flag.Int("port", 0, "port to listen on (defaults to ZEUS_PORT env var)")
	dev           = flag.Bool("dev", false, "disable round robin for development (single transport)")
	ipv6interface = flag.String("interface", DEFAULT_INTERFACE, "ipv6 interface")
	ipv6n         = flag.Int("v6_n", 16, "number of sequential ipv6 addresses")
	securityKey   = ""
	logger        = logging.NewLogger("zeus")
)

type transport struct {
	nW      int64
	nS      int64
	rt      []http.RoundTripper
	statsRl []*rate.Limiter
	wwwRl   []*rate.Limiter
}

var proxyTransport = &transport{}

func main() {
	flag.Parse()

	// Use env port if flag not provided
	if *port == 0 {
		if env.ZeusPort != "" {
			var err error
			if *port, err = strconv.Atoi(env.ZeusPort); err != nil {
				logger.Fatal("INVALID_ZEUS_PORT", err, map[string]any{
					"port": env.ZeusPort,
				})
			}
		} else {
			*port = 7777 // fallback default
		}
	}

	// Start metrics worker goroutine
	go metricsWorker()

	// Get security key from environment for API key forwarding
	securityKey = env.BungieAPIKey
	// Initialize transport with IPv6 support if available, otherwise use default
	// In dev mode, use single transport (no round robin). In production, use multiple transports.
	numIPs := 1
	if !*dev && env.ZeusIPV6 != "" {
		numIPs = *ipv6n
	}

	if env.ZeusIPV6 != "" && numIPs > 1 {
		// Production mode with IPv6: create multiple transports for round robin
		addr := netip.MustParseAddr(env.ZeusIPV6)
		for i := 0; i < numIPs; i++ {
			d := &net.Dialer{
				LocalAddr: &net.TCPAddr{
					IP: net.IP(addr.AsSlice()),
				},
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}
			rt := http.DefaultTransport.(*http.Transport).Clone()
			rt.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				return d.DialContext(ctx, network, addr)
			}
			proxyTransport.rt = append(proxyTransport.rt, rt)

			// Initialize rate limiters per IP (always enabled)
			proxyTransport.statsRl = append(proxyTransport.statsRl, rate.NewLimiter(rate.Every(time.Second/40), 90))
			proxyTransport.wwwRl = append(proxyTransport.wwwRl, rate.NewLimiter(rate.Every(time.Second/12), 25))
			addr = addr.Next()
		}
		logger.Info("IPV6_LOAD_BALANCING_ENABLED", map[string]any{
			"ips":         numIPs,
			"interface":   *ipv6interface,
			"round_robin": true,
		})
	} else if env.ZeusIPV6 != "" {
		// Dev mode with IPv6: use single IPv6 address
		addr := netip.MustParseAddr(env.ZeusIPV6)
		d := &net.Dialer{
			LocalAddr: &net.TCPAddr{
				IP: net.IP(addr.AsSlice()),
			},
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}
		rt := http.DefaultTransport.(*http.Transport).Clone()
		rt.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return d.DialContext(ctx, network, addr)
		}
		proxyTransport.rt = append(proxyTransport.rt, rt)
		proxyTransport.statsRl = append(proxyTransport.statsRl, rate.NewLimiter(rate.Every(time.Second/40), 90))
		proxyTransport.wwwRl = append(proxyTransport.wwwRl, rate.NewLimiter(rate.Every(time.Second/12), 25))
		logger.Info("IPV6_SINGLE_TRANSPORT_MODE", map[string]any{
			"interface":   *ipv6interface,
			"round_robin": false,
		})
	} else {
		// No IPv6: use default transport (single transport)
		proxyTransport.rt = append(proxyTransport.rt, http.DefaultTransport)
		proxyTransport.statsRl = append(proxyTransport.statsRl, rate.NewLimiter(rate.Every(time.Second/40), 90))
		proxyTransport.wwwRl = append(proxyTransport.wwwRl, rate.NewLimiter(rate.Every(time.Second/12), 25))
		logger.Info("USING_DEFAULT_TRANSPORT", map[string]any{
			"round_robin": false,
		})
	}

	rp := &httputil.ReverseProxy{
		Director: func(r *http.Request) {
			r.URL.Scheme = "https"
			r.Header.Set("User-Agent", USER_AGENT)
			if strings.Contains(r.URL.Path, "Destiny2/Stats/PostGameCarnageReport") {
				r.URL.Host = "stats.bungie.net"
			} else {
				r.URL.Host = "www.bungie.net"
			}
			r.Header.Del("x-forwarded-for")
		},
		Transport: proxyTransport,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			// Handle context cancellation gracefully (client disconnect is normal)
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				logger.Debug("REQUEST_CANCELED", map[string]any{
					logging.METHOD:   r.Method,
					logging.ENDPOINT: r.URL.String(),
				})
			} else if strings.Contains(err.Error(), "connection reset by peer") {
				// Connection reset errors are common and should be handled silently
				logger.Debug("CONNECTION_RESET", map[string]any{
					logging.METHOD:   r.Method,
					logging.ENDPOINT: r.URL.String(),
				})
			} else if strings.Contains(err.Error(), "no such host") {
				// DNS lookup errors are network issues that should be logged at info level
				logger.Warn("PROXY_DNS_ERROR", err, map[string]any{
					logging.METHOD:   r.Method,
					logging.ENDPOINT: r.URL.String(),
				})
			} else {
				logger.Warn("PROXY_ERROR", err, map[string]any{
					logging.METHOD:   r.Method,
					logging.ENDPOINT: r.URL.String(),
				})
			}
			w.WriteHeader(http.StatusBadGateway)
		},
	}

	mainHandler := http.HandlerFunc(rp.ServeHTTP)
	logger.Info("SERVICE_READY", map[string]any{
		"port": *port,
	})

	if err := http.ListenAndServe(fmt.Sprintf(":%d", *port), mainHandler); err != nil {
		logger.Error("SERVER_FAILED", err, nil)
	}
}

func (t *transport) RoundTrip(r *http.Request) (*http.Response, error) {
	var rl *rate.Limiter
	var n int64
	var rt http.RoundTripper
	var endpointType string
	startTime := time.Now()
	var rateLimiterWait time.Duration

	if strings.Contains(r.URL.Path, "Destiny2/Stats/PostGameCarnageReport") {
		n = atomic.AddInt64(&t.nS, 1)
		r.Host = "stats.bungie.net"
		endpointType = "stats"
		if len(t.statsRl) > 0 {
			// In dev mode, always use the first rate limiter (single transport)
			// In production, round robin through rate limiters
			rl = t.statsRl[n%int64(len(t.statsRl))]
		}
	} else {
		n = atomic.AddInt64(&t.nW, 1)
		r.Host = "www.bungie.net"
		endpointType = "www"
		if len(t.wwwRl) > 0 {
			// In dev mode, always use the first rate limiter (single transport)
			// In production, round robin through rate limiters
			rl = t.wwwRl[n%int64(len(t.wwwRl))]
		}
	}

	// Forward API key if security key is provided
	if securityKey != "" && r.Header.Get("x-api-key") == securityKey {
		r.Header.Set("x-api-key", securityKey)
		r.Header.Add("x-forwarded-for", securityKey)
	} else if securityKey != "" {
		// Only log warning if security key is configured but not provided
		logger.Warn("SECURITY_CHECK_FAILED", errors.New("api key mismatch"), map[string]any{
			logging.HOST:   r.Host,
			logging.PATH:   r.URL.Path,
			logging.METHOD: r.Method,
			"x_api_key": r.Header.Get("x-api-key"),
		})
	}

	// Measure rate limiter wait time
	if rl != nil {
		rateLimiterStart := time.Now()
		if err := rl.Wait(r.Context()); err != nil {
			// Context canceled or deadline exceeded - return error
			return nil, err
		}
		rateLimiterWait = time.Since(rateLimiterStart)
	}

	// In dev mode, always use the first transport (no round robin)
	// In production, round robin through transports
	rt = t.rt[n%int64(len(t.rt))]

	logger.Debug("FORWARDING_REQUEST", map[string]any{
		logging.METHOD:   r.Method,
		logging.ENDPOINT: r.URL.String(),
		"host":           r.Host,
	})

	// Make the actual request
	resp, err := rt.RoundTrip(r)
	duration := time.Since(startTime)

	// Determine status code for metrics
	status := "0" // Default for errors/no response
	if err == nil && resp != nil {
		status = strconv.Itoa(resp.StatusCode)
	}

	// Send metrics event to channel (non-blocking with buffered channel)
	select {
	case metricsChan <- metricsEvent{
		endpointType:    endpointType,
		duration:        duration,
		rateLimiterWait: rateLimiterWait,
		status:          status,
	}:
	default:
		// Channel full, drop metric to avoid blocking (buffer should be large enough)
		logger.Warn("METRICS_CHANNEL_FULL", errors.New("unable to process zeus metrics"), nil)
	}

	return resp, err
}
