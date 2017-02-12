package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics collects the metrics.
type Metrics struct {
	AuthValid,
	AuthExchange,
	Errs *prometheus.CounterVec
}

// M structure to collect all metrics together.
var M Metrics

// Register metrics and metrics page.
func Register() {
	M = Metrics{
		AuthValid: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "cashier",
			Subsystem: "auth",
			Name:      "valid_total",
			Help:      "Auth Valid calls",
		}, []string{"module"}),
		AuthExchange: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "cashier",
			Subsystem: "auth",
			Name:      "exchange_total",
			Help:      "Auth Exchange calls",
		}, []string{"module"}),
		Errs: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "cashier",
			Subsystem: "sys",
			Name:      "error_total",
			Help:      "Error counts by module",
		}, []string{"module"}),
	}
	prometheus.MustRegister(M.Errs)
	prometheus.MustRegister(M.AuthValid)
	prometheus.MustRegister(M.AuthExchange)
}
