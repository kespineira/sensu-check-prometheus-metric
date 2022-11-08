package main

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	corev2 "github.com/sensu/sensu-go/api/core/v2"
	"github.com/sensu/sensu-plugin-sdk/sensu"
)

type Config struct {
	sensu.PluginConfig
	Host     string
	Port     string
	Query    string
	Critical float64
	Warning  float64
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name: "sensu-check-prometheus-metric",
		},
	}

	options = []sensu.ConfigOption{
		&sensu.PluginConfigOption[string]{
			Path:      "host",
			Env:       "SENSU_CHECK_PROMETHEUS_METRIC_HOST",
			Argument:  "host",
			Shorthand: "H",
			Usage:     "Prometheus host",
			Value:     &plugin.Host,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "port",
			Env:       "SENSU_CHECK_PROMETHEUS_METRIC_PORT",
			Argument:  "port",
			Shorthand: "p",
			Usage:     "Prometheus port",
			Value:     &plugin.Port,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "query",
			Env:       "SENSU_CHECK_PROMETHEUS_METRIC_QUERY",
			Argument:  "query",
			Shorthand: "q",
			Usage:     "Prometheus query",
			Value:     &plugin.Query,
		},
		&sensu.PluginConfigOption[float64]{
			Path:      "critical",
			Env:       "SENSU_CHECK_PROMETHEUS_METRIC_CRITICAL",
			Argument:  "critical",
			Shorthand: "c",
			Usage:     "Critical threshold",
			Value:     &plugin.Critical,
		},
		&sensu.PluginConfigOption[float64]{
			Path:      "warning",
			Env:       "SENSU_CHECK_PROMETHEUS_METRIC_WARNING",
			Argument:  "warning",
			Shorthand: "w",
			Usage:     "Warning threshold",
			Value:     &plugin.Warning,
		},
	}
)

func main() {
	check := sensu.NewGoCheck(&plugin.PluginConfig, options, checkArgs, executeCheck, false)
	check.Execute()
}

func checkArgs(_ *corev2.Event) (int, error) {
	if plugin.Host == "" {
		return sensu.CheckStateWarning, fmt.Errorf("--host or SENSU_CHECK_PROMETHEUS_METRIC_HOST environment variable is required")
	}
	if plugin.Port == "" {
		return sensu.CheckStateWarning, fmt.Errorf("--port or SENSU_CHECK_PROMETHEUS_METRIC_PORT environment variable is required")
	}
	if plugin.Query == "" {
		return sensu.CheckStateWarning, fmt.Errorf("--query or SENSU_CHECK_PROMETHEUS_METRIC_QUERY environment variable is required")
	}
	if plugin.Critical == 0 {
		return sensu.CheckStateWarning, fmt.Errorf("--critical or SENSU_CHECK_PROMETHEUS_METRIC_CRITICAL environment variable is required")
	}
	if plugin.Warning == 0 {
		return sensu.CheckStateWarning, fmt.Errorf("--warning or SENSU_CHECK_PROMETHEUS_METRIC_WARNING environment variable is required")
	}
	return sensu.CheckStateOK, nil
}

func executeCheck(_ *corev2.Event) (int, error) {
	client, err := api.NewClient(api.Config{
		Address: fmt.Sprintf("http://%s:%s", plugin.Host, plugin.Port),
	})
	if err != nil {
		return sensu.CheckStateCritical, err
	}

	promClient := promv1.NewAPI(client)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	defer cancel()

	result, warnings, err := promClient.Query(ctx, plugin.Query, time.Now(), promv1.WithTimeout(10*time.Second))
	if err != nil {
		return sensu.CheckStateCritical, err
	}

	if len(warnings) > 0 {
		return sensu.CheckStateWarning, fmt.Errorf("warnings: %v", warnings)
	}
	if result.Type() != model.ValVector {
		return sensu.CheckStateCritical, fmt.Errorf("unexpected result type %s", result.Type())
	}

	vector := result.(model.Vector)

	if len(vector) == 0 {
		return sensu.CheckStateCritical, fmt.Errorf("no metrics returned")
	}

	if len(vector) > 1 {
		return sensu.CheckStateCritical, fmt.Errorf("more than one metric returned")
	}

	metric := vector[0]

    if float64(metric.Value) > plugin.Critical {
		fmt.Printf("CheckPrometheusMetric CRITICAL: %s is %f", plugin.Query, metric.Value)
		return sensu.CheckStateCritical, nil
    } else if float64(metric.Value) > plugin.Warning {
		fmt.Printf("CheckPrometheusMetric WARNING: %s is %f", plugin.Query, metric.Value)
		return sensu.CheckStateWarning, nil
	}

	fmt.Printf("CheckPrometheusMetric OK: %s is %f", plugin.Query, metric.Value)
	return sensu.CheckStateOK, nil
}
