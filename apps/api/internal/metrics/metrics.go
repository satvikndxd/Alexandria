// Package metrics exposes the handful of counters Alexandria actually needs,
// in Prometheus text format, with no client dependency.
//
// Deliberately small: a reading platform's health is request flow, friction
// refusals, outbox depth and ingest state — not a thousand gauges. Every
// series here is one a human would page on or graph during an incident.
package metrics

import (
	"fmt"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
)

var (
	requestsMu sync.RWMutex
	requests   = map[string]*atomic.Int64{} // key: method|statusClass

	frictionMu sync.RWMutex
	friction   = map[string]*atomic.Int64{} // key: error code

	gaugesMu sync.RWMutex
	gauges   = map[string]*atomic.Int64{}
)

func counter(m *sync.RWMutex, table map[string]*atomic.Int64, key string) *atomic.Int64 {
	m.RLock()
	c, ok := table[key]
	m.RUnlock()
	if ok {
		return c
	}
	m.Lock()
	defer m.Unlock()
	if c, ok := table[key]; ok {
		return c
	}
	c = &atomic.Int64{}
	table[key] = c
	return c
}

// ObserveRequest counts one HTTP request by method and status class (2xx…).
func ObserveRequest(method string, status int) {
	key := method + "|" + fmt.Sprintf("%dxx", status/100)
	counter(&requestsMu, requests, key).Add(1)
}

// ObserveFriction counts a refusal by its error code, so the anti-slop
// system's effect is visible: a spike in invalid_review or daily_limit is
// either an attack or a UI bug, and both are worth knowing about.
func ObserveFriction(code string) {
	counter(&frictionMu, friction, code).Add(1)
}

// SetGauge publishes a point-in-time value (outbox depth, ingest queue).
func SetGauge(name string, v int64) {
	counter(&gaugesMu, gauges, name).Store(v)
}

// Handler renders the Prometheus text exposition format.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		fmt.Fprintln(w, "# HELP alexandria_http_requests_total HTTP requests by method and status class.")
		fmt.Fprintln(w, "# TYPE alexandria_http_requests_total counter")
		for _, k := range sortedKeys(&requestsMu, requests) {
			fmt.Fprintf(w, "alexandria_http_requests_total{method=%q,class=%q} %d\n",
				splitKey(k)[0], splitKey(k)[1], requests[k].Load())
		}
		fmt.Fprintln(w, "# HELP alexandria_friction_refusals_total Requests refused by structural friction, by code.")
		fmt.Fprintln(w, "# TYPE alexandria_friction_refusals_total counter")
		for _, k := range sortedKeys(&frictionMu, friction) {
			fmt.Fprintf(w, "alexandria_friction_refusals_total{code=%q} %d\n", k, friction[k].Load())
		}
		fmt.Fprintln(w, "# HELP alexandria_gauge Point-in-time values.")
		fmt.Fprintln(w, "# TYPE alexandria_gauge gauge")
		for _, k := range sortedKeys(&gaugesMu, gauges) {
			fmt.Fprintf(w, "alexandria_gauge{name=%q} %d\n", k, gauges[k].Load())
		}
	})
}

func sortedKeys(m *sync.RWMutex, table map[string]*atomic.Int64) []string {
	m.RLock()
	defer m.RUnlock()
	out := make([]string, 0, len(table))
	for k := range table {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func splitKey(k string) [2]string {
	for i := 0; i < len(k); i++ {
		if k[i] == '|' {
			return [2]string{k[:i], k[i+1:]}
		}
	}
	return [2]string{k, ""}
}
