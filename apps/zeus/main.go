// This code was created by cbro

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/netip"
	"strings"
	"sync/atomic"
	"time"

	"github.com/joho/godotenv"
	"github.com/paulbellamy/ratecounter"
	"golang.org/x/time/rate"

	"raidhub/lib/env"
)

var (
	ipv6interface = flag.String("interface", "enp1s0", "ipv6 interface")
	ipv6n         = flag.Int("v6_n", 16, "number of sequential ipv6 addresses")
	port          = flag.Int("port", 7777, "port to listen on")
	printAddrs    = flag.Bool("print_addrs", false, "print ipv6 addresses")
	verbose       = flag.Bool("verbose", false, "print logs")
)

var (
	securityKey         = ""
	rateIntervalSeconds = 10
	rateInterval        = time.Second * time.Duration(rateIntervalSeconds)
	writeCounter        = ratecounter.NewRateCounter(rateInterval)
	readCounter         = ratecounter.NewRateCounter(rateInterval)
)

type transport struct {
	nW      int64
	nS      int64
	rt      []http.RoundTripper
	statsRl []*rate.Limiter
	wwwRl   []*rate.Limiter
	apiKeys []string
}

var proxyTransport = &transport{}

func main() {
	flag.Parse()
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	securityKey = env.BungieAPIKey
	if securityKey == "" {
		log.Fatal("must pass BUNGIE_API_KEY")
	}

	proxyTransport.apiKeys = strings.Split(env.ZeusAPIKeys, ",")

	addr := netip.MustParseAddr(env.IPV6)
	for i := 0; i < *ipv6n; i++ {
		d := &net.Dialer{
			LocalAddr: &net.TCPAddr{
				IP: net.IP(addr.AsSlice()),
			},
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}
		rt := http.DefaultTransport.(*http.Transport).Clone()
		rt.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := d.DialContext(ctx, network, addr)
			if err != nil {
				return conn, err
			}
			return wrappedConn{conn}, err
		}
		if *printAddrs {
			fmt.Printf("sudo ip -6 addr add %s/64 dev %s\n", addr.String(), *ipv6interface)
		}
		proxyTransport.statsRl = append(proxyTransport.statsRl, rate.NewLimiter(rate.Every(time.Second/40), 90))
		proxyTransport.wwwRl = append(proxyTransport.wwwRl, rate.NewLimiter(rate.Every(time.Second/12), 25))
		proxyTransport.rt = append(proxyTransport.rt, rt)
		addr = addr.Next()
	}
	rp := &httputil.ReverseProxy{
		Director: func(r *http.Request) {
			r.URL.Scheme = "https"
			r.Header.Set("User-Agent", "RaidHub-Zeus/1.0.0 (contact=admin@raidhub.io)")
			if strings.Contains(r.URL.Path, "Destiny2/Stats/PostGameCarnageReport") {
				r.URL.Host = "stats.bungie.net"
			} else {
				r.URL.Host = "www.bungie.net"
			}
			r.Header.Del("x-forwarded-for")
		},
		Transport: proxyTransport,
	}
	if *printAddrs {
		return
	}
	mainHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-betteruptime-probe") != "" {
			io.WriteString(w, "ok")
			return
		}

		rp.ServeHTTP(w, r)
	})
	log.Printf("Ready on port %d", *port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), mainHandler))
}

func (t *transport) RoundTrip(r *http.Request) (*http.Response, error) {
	var rl *rate.Limiter
	var n int64
	if strings.Contains(r.URL.Path, "Destiny2/Stats/PostGameCarnageReport") {
		n = atomic.AddInt64(&t.nS, 1)
		r.Host = "stats.bungie.net"
		rl = t.statsRl[n%int64(len(t.statsRl))]
	} else {
		n = atomic.AddInt64(&t.nW, 1)
		r.Host = "www.bungie.net"
		rl = t.wwwRl[n%int64(len(t.wwwRl))]
	}
	apiKey := t.apiKeys[n%int64(len(t.apiKeys))]
	if r.Header.Get("x-api-key") == securityKey {
		if *verbose {
			fmt.Printf("Security key provided: %s\n", r.Header.Get("x-api-key"))
			fmt.Printf("Using API Key: %s\n", apiKey)
		}
		r.Header.Set("X-API-KEY", apiKey)
		r.Header.Add("x-forwarded-for", apiKey)
	}
	if *verbose {
		fmt.Printf("Sending Request: %s\n", r.URL.String())
		fmt.Printf("Request Headers: %s\n", r.Header)
	}
	rt := t.rt[n%int64(len(t.rt))]
	rl.Wait(r.Context())
	return rt.RoundTrip(r)
}

type wrappedConn struct {
	net.Conn
}

// Read writes all data read from the underlying connection to sc.Writer.
func (c wrappedConn) Read(b []byte) (int, error) {
	readCounter.Incr(int64(len(b)))
	return c.Conn.Read(b)
}

// Write writes all data written to the underlying connection to sc.Writer.
func (c wrappedConn) Write(b []byte) (int, error) {
	writeCounter.Incr(int64(len(b)))
	return c.Conn.Write(b)
}
