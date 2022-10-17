package controller

import (
	"fmt"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var (
	textFileDirectory = kingpin.Flag("collector.textfile.directory", "Directory to read text files with metrics from.").Default("").String()

	mtimeDesc = prometheus.NewDesc(
		"node_textfile_mtime_seconds",
		"Unix_time mtime of text files successful read.",
		[]string{"file"},
		map[string]string{
			"source": "static_file",
		},
	)
)

type TextFileCollector struct {
	path   string
	mtime  *float64
	logger log.Logger
}

func NewTextFileCollector(logger log.Logger) (Collector, error) {
	return &TextFileCollector{
		path:   *textFileDirectory,
		logger: logger,
	}, nil
}

func (c *TextFileCollector) exportMTime(mtimes map[string]time.Time, ch chan<- prometheus.Metric) {
	if len(mtimes) == 0 {
		return
	}

	// Export the mtimes of the successful files.
	// Sorting is needed for predictable output comparison in tests.
	filePaths := make([]string, 0, len(mtimes))
	for path := range mtimes {
		filePaths = append(filePaths, path)
	}
	sort.Strings(filePaths)

	for _, path := range filePaths {
		mtime := float64(mtimes[path].UnixNano() / 1e9)
		if c.mtime != nil {
			mtime = *c.mtime
		}
		ch <- prometheus.MustNewConstMetric(mtimeDesc, prometheus.GaugeValue, mtime, path)
	}
}

func convertMetricsFamily(metricsFamily *dto.MetricFamily, ch chan<- prometheus.Metric, logger log.Logger) {
	var (
		valType prometheus.ValueType
		val     float64
	)

	allLabelsNames := map[string]struct{}{}

	for _, metric := range metricsFamily.Metric {
		labels := metric.GetLabel()
		for _, label := range labels {
			if _, ok := allLabelsNames[label.GetName()]; !ok {
				allLabelsNames[label.GetName()] = struct{}{}
			}
		}
	}

	for _, metric := range metricsFamily.Metric {
		if metric.TimestampMs != nil {
			level.Warn(logger).Log("msg", "Ignoring unsupported custom timestamp on textfile collector metric", "metric", metric)
		}

		labels := metric.GetLabel()
		var names []string
		var values []string
		for _, label := range labels {
			names = append(names, label.GetName())
			values = append(values, label.GetValue())
		}

		for k := range allLabelsNames {
			present := false
			for _, name := range names {
				if k == name {
					present = true
					break
				}
			}
			if !present {
				names = append(names, k)
				values = append(values, "")
			}
		}

		metricType := metricsFamily.GetType()
		switch metricType {
		case dto.MetricType_COUNTER:
			valType = prometheus.CounterValue
			val = metric.Counter.GetValue()

		case dto.MetricType_GAUGE:
			valType = prometheus.GaugeValue
			val = metric.Gauge.GetValue()

		case dto.MetricType_UNTYPED:
			valType = prometheus.UntypedValue
			val = metric.Untyped.GetValue()

		case dto.MetricType_SUMMARY:
			quantiles := map[float64]float64{}
			for _, q := range metric.Summary.Quantile {
				quantiles[q.GetQuantile()] = q.GetValue()
			}
			ch <- prometheus.MustNewConstSummary(
				prometheus.NewDesc(
					*metricsFamily.Name,
					metricsFamily.GetHelp(),
					names,
					nil,
				),
				metric.Summary.GetSampleCount(),
				metric.Summary.GetSampleSum(),
				quantiles, values...,
			)
		case dto.MetricType_HISTOGRAM:
			buckets := map[float64]uint64{}
			for _, b := range metric.Histogram.Bucket {
				buckets[b.GetUpperBound()] = b.GetCumulativeCount()
			}
			ch <- prometheus.MustNewConstHistogram(
				prometheus.NewDesc(
					*metricsFamily.Name,
					metricsFamily.GetHelp(),
					names, nil,
				),
				metric.Histogram.GetSampleCount(),
				metric.Histogram.GetSampleSum(),
				buckets, values...,
			)
		default:
			panic("unknown metric type")
		}

		if metricType == dto.MetricType_GAUGE || metricType == dto.MetricType_COUNTER || metricType == dto.MetricType_UNTYPED {
			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc(
					*metricsFamily.Name,
					metricsFamily.GetHelp(),
					names, nil,
				),
				valType, val, values...,
			)
		}
	}
}

func (c *TextFileCollector) Update(ch chan<- prometheus.Metric) error {
	// Iterate over files and accumulate their metrics, but also track any
	// parsing errors so an error metrics can be reported.
	var errored bool
	var parsedFamilies []*dto.MetricFamily
	metricsNameToFiles := map[string][]string{}

	// path maybe has sub path, the paths has no sub path
	paths, err := filepath.Glob(c.path)
	if err != nil || len(paths) == 0 {
		// not glob or not accessible path either way assume single directory and let os.ReadDir handle it.
		paths = []string{c.path}
	}

	// to store data like : {"test.prom": time.Time{}}
	mtimes := make(map[string]time.Time)

	for _, path := range paths {
		files, err := os.ReadDir(path)
		if err != nil && path != "" {
			errored = true
			level.Error(c.logger).Log("msg", "failed to read textfile collector directory", "path", path, "err", err)
		}

		for _, f := range files {
			metricsFilePath := filepath.Join(path, f.Name())
			// only regex file with suffix '.prom'
			if !strings.HasSuffix(f.Name(), ".prom") {
				continue
			}

			mtime, families, err := c.processFile(path, f.Name(), ch)

			for _, mf := range families {
				metricsNameToFiles[*mf.Name] = append(metricsNameToFiles[*mf.Name], metricsFilePath)
				parsedFamilies = append(parsedFamilies, mf)
			}

			if err != nil {
				errored = true
				level.Error(c.logger).Log("msg", "failed to collect textfile data", "file", f.Name(), "err", err)
				continue
			}

			mtimes[metricsFilePath] = *mtime
		}

	}

	// determine if the prometheus metrics has help information.
	for _, mf := range parsedFamilies {
		if mf.Help == nil {
			help := fmt.Sprintf("Metric read from %s", strings.Join(metricsNameToFiles[*mf.Name], ", "))
			mf.Help = &help
		}
	}

	for _, mf := range parsedFamilies {
		convertMetricsFamily(mf, ch, c.logger)
	}

	c.exportMTime(mtimes, ch)

	var errVal float64
	if errored {
		errVal = 1.0
	}

	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc(
			"node_textfile_scrape_error",
			"1 if there was an error opening or reading a file, 0 otherwise",
			nil, nil,
		),
		prometheus.GaugeValue, errVal,
	)
	return nil
}

// processFile convert file inner data to prometheus family data and get the file.stat.ModTime()
func (c *TextFileCollector) processFile(dir, name string, ch chan<- prometheus.Metric) (*time.Time, map[string]*dto.MetricFamily, error) {
	path := filepath.Join(dir, name)

	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open textfile data file %q: %w", path, err)
	}
	defer f.Close()

	var parser expfmt.TextParser
	families, err := parser.TextToMetricFamilies(f)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse textfile data from %q: %w", path, err)
	}

	if hasTimeStamps(families) {
		return nil, nil, fmt.Errorf("textfile %q contains unsupported client-side timestamps, skipping entire file", path)
	}

	// Only stat the file once it has been parsed and validated, so that
	// a failure does not appear fresh.
	stat, err := f.Stat()
	if err != nil {
		return nil, families, fmt.Errorf("failed to stat %q: %w", path, err)
	}

	t := stat.ModTime()
	return &t, families, nil
}

// hasTimeStamps returns true when metrics contains unsupported timestamps
func hasTimeStamps(parsedFamilies map[string]*dto.MetricFamily) bool {
	for _, mf := range parsedFamilies {
		for _, m := range mf.Metric {
			if m.TimestampMs != nil {
				return true
			}
		}
	}
	return false
}

func init() {
	registryCollector("textfile", NewTextFileCollector)
}
