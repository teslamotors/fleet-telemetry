package adapter

// Labels provides values for labels
type Labels map[string]string

// CollectorOptions stores details needed for creating new Collector
type CollectorOptions struct {
	Name   string
	Help   string
	Labels []string
}

// Gauge can be set to anything
type Gauge interface {
	Add(int64, Labels)
	Sub(int64, Labels)
	Inc(Labels)
	Set(int64, Labels)
}

// Counter goes up
type Counter interface {
	Add(int64, Labels)
	Inc(Labels)
}

// Timer observes trends
type Timer interface {
	Observe(int64, Labels)
}
