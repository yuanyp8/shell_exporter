package controller

import (
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"sync"
)

type ShellCollector struct {
	Collectors map[string]Collector
	logger     log.Logger
}

func NewShellCollector(logger log.Logger) (*ShellCollector, error) {
	f := make(map[string]bool)

	collectors := make(map[string]Collector)

	initiatedCollectorsMtx.Lock()
	defer initiatedCollectorsMtx.Unlock()

	for key, enabled := range collectorState {
		if !*enabled || (len(f) > 0 && !f[key]) {
			continue
		}
		if collector, ok := initiatedCollectors[key]; ok {
			collectors[key] = collector
		} else {
			collector, err := factories[key](log.With(logger, "collector", key))
			if err != nil {
				return nil, err
			}
			collectors[key] = collector
			initiatedCollectors[key] = collector
		}
	}
	return &ShellCollector{Collectors: collectors, logger: logger}, nil

}

// Describe implements the prometheus.Collector interface.
func (s ShellCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- scrapeDurationDesc
	ch <- scrapeSuccessDesc
}

// Collect implements the prometheus.Collector interface.
func (s ShellCollector) Collect(ch chan<- prometheus.Metric) {
	wg := sync.WaitGroup{}
	wg.Add(len(s.Collectors))
	for name, c := range s.Collectors {
		go func(name string, c Collector) {
			Execute(name, c, ch, s.logger)
			wg.Done()
		}(name, c)
	}
	wg.Wait()
}
