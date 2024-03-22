package crccaddyplugin

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/redhatinsights/crcauthlib"
)

var responseLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: "caddy",
	Subsystem: "http",
	Name:      "response_latency_sec",
	Help:      "Histogram of the latency time (in seconds)",
	Buckets:   append(prometheus.DefBuckets, 15, 30, 60, 180, 240, 960, 1800),
}, []string{"api", "status"})

var responseDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: "caddy",
	Subsystem: "http",
	Name:      "response_duration_sec",
	Help:      "Histogram of the duration time (in seconds)",
	Buckets:   append(prometheus.DefBuckets, 15, 30, 60, 180, 240, 960, 1800),
}, []string{"api", "status"})

type IdentResponse struct {
	Name  string `json:"name,omitempty"`
	ID    string `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
}

func init() {
	caddy.RegisterModule(Middleware{})
	httpcaddyfile.RegisterHandlerDirective("crcauth", parseCaddyfile)
}

type Middleware struct {
	Output    string   `json:"output,omitempty"`
	BOP       string   `json:"url,omitempty"`
	Whitelist []string `json:"whitelist,omitempty"`
	validator *crcauthlib.CRCAuthValidator

	w io.Writer
}

// CaddyModule returns the Caddy module information.
func (Middleware) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.crcauth",
		New: func() caddy.Module { return new(Middleware) },
	}
}

// Provision implements caddy.Provisioner.
func (m *Middleware) Provision(ctx caddy.Context) error {
	switch m.Output {
	case "stdout":
		m.w = os.Stdout
	case "stderr":
		m.w = os.Stderr
	default:
		return fmt.Errorf("an output stream is required")
	}
	config := crcauthlib.ValidatorConfig{BOPUrl: m.BOP}
	validator, err := crcauthlib.NewCRCAuthValidator(&config)

	if err != nil {
		return err
	}

	m.validator = validator

	return nil
}

// Validate implements caddy.Validator.
func (m *Middleware) Validate() error {
	if m.w == nil {
		return fmt.Errorf("no writer")
	}
	return nil
}

type durationWriter struct {
	start time.Time
	http.ResponseWriter
}

func (d *durationWriter) Write(data []byte) (int, error) {
	d.doWrite()
	return d.ResponseWriter.Write(data)
}

func (d *durationWriter) WriteHeader(status int) {
	d.doWrite()
	d.ResponseWriter.WriteHeader(status)
}

func (d *durationWriter) doWrite() {
	if d.start.IsZero() {
		d.start = time.Now()
	}
}

func matchWhitelist(path, whitelist string) bool {

	// Exact match: “abc” will match on value “abc”.
	// Prefix match: “abc*” will match on value “abc” and “abcd”.
	// Suffix match: “*abc” will match on value “abc” and “xabc”.
	// Presence match: “*” will match when value is not empty.

	if !strings.Contains(whitelist, "*") {
		return path == whitelist
	}

	if strings.HasPrefix(whitelist, "*") {
		suffix := whitelist[1:]
		return strings.HasSuffix(path, suffix)
	}

	if strings.HasSuffix(whitelist, "*") {
		prefix := whitelist[:len(whitelist)-1]
		return strings.HasPrefix(path, prefix)
	}

	if whitelist == "*" {
		return true
	}

	return false
}

// ServeHTTP implements caddyhttp.MiddlewareHandler.
func (m Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	api := "unknown"
	var err error

	if r.Method != http.MethodOptions {

		urlComponents := strings.Split(r.RequestURI, "/")
		fmt.Printf("\n\n%v\n\n", urlComponents)

		for _, whitelist := range m.Whitelist {
			if matchWhitelist(r.RequestURI, whitelist) {
				return next.ServeHTTP(w, r)
			}
		}
		if len(urlComponents) >= 2 {
			if urlComponents[1] == "api" {
				api = urlComponents[2]
			}
		}

		ident, err := m.validator.ProcessRequest(r)
		if err != nil {
			// Check if we want to uncomment later
			//w.Write([]byte(fmt.Sprintf("%s", err)))
			return caddyhttp.Error(403, err)
		}

		jdata, err := json.Marshal(ident)

		if err != nil {
			return caddyhttp.Error(403, err)
		}

		output := base64.StdEncoding.EncodeToString(jdata)

		r.Header["x-rh-identity"] = []string{output}
		//m.w.Write([]byte(fmt.Sprintf("%v", r)))

	}

	start := time.Now()

	rr := caddyhttp.NewResponseRecorder(w, nil, nil)

	durWriter := &durationWriter{ResponseWriter: rr}

	err = next.ServeHTTP(durWriter, r)

	durWriter.doWrite()

	statusCode := strconv.Itoa(rr.Status())

	responseLatency.WithLabelValues(api, statusCode).Observe(durWriter.start.Sub(start).Seconds())
	responseDuration.WithLabelValues(api, statusCode).Observe(time.Since(start).Seconds())

	if err != nil {
		return err
	}

	if !rr.Buffered() {
		return nil
	}

	return err
}

// UnmarshalCaddyfile implements caddyfile.Unmarshaler.
func (m *Middleware) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	d.Next()
	d.Next()
	for d.Next() {
		seg := d.NextSegment()
		switch seg.Directive() {
		case "output":
			if len(seg) != 2 {
				return d.ArgErr()
			}
			m.Output = seg[1].Text
		case "bop":
			if len(seg) != 2 {
				return d.ArgErr()
			}
			m.BOP = seg[1].Text
		case "whitelist":
			if len(seg) > 2 {
				return d.ArgErr()
			}
			if len(seg) == 2 {
				m.Whitelist = strings.Split(seg[1].Text, ",")
			}
		}
	}
	return nil
}

// parseCaddyfile unmarshals tokens from h into a new Middleware.
func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {

	prometheus.MustRegister(responseLatency)
	prometheus.MustRegister(responseDuration)

	handler := promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{
		ErrorHandling: promhttp.HTTPErrorOnError,
		ErrorLog:      log.New(os.Stderr, "", log.LstdFlags),
	})

	http.Handle("/metrics", handler)
	go func() {
		err := http.ListenAndServe(":9080", nil)
		if err != nil {
			fmt.Printf("[ERROR] Starting handler: %v", err)
		}
	}()

	var m Middleware
	err := m.UnmarshalCaddyfile(h.Dispenser)
	return m, err
}

// Interface guards
var (
	_ caddy.Provisioner           = (*Middleware)(nil)
	_ caddy.Validator             = (*Middleware)(nil)
	_ caddyhttp.MiddlewareHandler = (*Middleware)(nil)
	_ caddyfile.Unmarshaler       = (*Middleware)(nil)
)
