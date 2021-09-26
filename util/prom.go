package util

import (
	"github.com/prometheus/client_golang/prometheus"
)

// NewExporterMetric - Convenience function to create, register and set a gauge containing exporter info.
func NewExporterMetric(registry *prometheus.Registry, namespace string, version string) {
	infoLabels := make(prometheus.Labels)
	infoLabels["version"] = version
	NewGauge(registry, namespace, "exporter", "info", "Metadata about the exporter.", infoLabels).Set(1)
}

// NewGauge - Convenience function to create, register and return a gauge.
func NewGauge(registry *prometheus.Registry, namespace string, subsystem string, name string, help string, constLabels prometheus.Labels) prometheus.Gauge {
	var metric = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   namespace,
		Subsystem:   subsystem,
		Name:        name,
		Help:        help,
		ConstLabels: constLabels,
	})
	registry.MustRegister(metric)
	return metric
}

// NewGaugeVec - Convenience function to create, register and return a labeled gauge.
func NewGaugeVec(registry *prometheus.Registry, namespace string, subsystem string, name string, help string, constLabels prometheus.Labels, labels prometheus.Labels) *prometheus.GaugeVec {
	var metric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   namespace,
		Subsystem:   subsystem,
		Name:        name,
		Help:        help,
		ConstLabels: constLabels,
	}, MapKeys(labels))
	registry.MustRegister(metric)
	return metric
}

// MergeLabels - Merge multiple label maps into one. If they have overlapping keys, the value from the most right map will be used.
func MergeLabels(maps ...prometheus.Labels) prometheus.Labels {
	result := make(prometheus.Labels)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}
