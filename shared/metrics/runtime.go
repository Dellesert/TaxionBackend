package metrics

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

// RuntimeMetrics represents Go runtime metrics
type RuntimeMetrics struct {
	// Memory metrics
	MemoryAlloc      uint64  `json:"memory_alloc"`       // bytes allocated and still in use
	MemoryTotalAlloc uint64  `json:"memory_total_alloc"` // bytes allocated (even if freed)
	MemorySys        uint64  `json:"memory_sys"`         // bytes obtained from system
	MemoryHeapAlloc  uint64  `json:"memory_heap_alloc"`  // heap bytes allocated
	MemoryHeapInuse  uint64  `json:"memory_heap_inuse"`  // heap bytes in use
	MemoryStackInuse uint64  `json:"memory_stack_inuse"` // stack bytes in use
	MemoryUsagePercent float64 `json:"memory_usage_percent"` // percentage of heap in use

	// Goroutine metrics
	NumGoroutines int `json:"num_goroutines"` // number of goroutines

	// GC metrics
	NumGC        uint32  `json:"num_gc"`         // number of GC runs
	PauseNs      uint64  `json:"pause_ns"`       // last GC pause in nanoseconds
	PauseTotal   uint64  `json:"pause_total_ns"` // total GC pause time
	GCCPUPercent float64 `json:"gc_cpu_percent"` // percentage of CPU time spent in GC

	// CPU metrics (approximation)
	NumCPU      int     `json:"num_cpu"`       // number of logical CPUs
	NumCGoCalls int64   `json:"num_cgo_calls"` // number of cgo calls

	// Timestamp
	Timestamp time.Time `json:"timestamp"`
}

// HTTPMetrics represents HTTP request metrics
type HTTPMetrics struct {
	TotalRequests    int64         `json:"total_requests"`
	SuccessRequests  int64         `json:"success_requests"`  // 2xx
	ClientErrors     int64         `json:"client_errors"`     // 4xx
	ServerErrors     int64         `json:"server_errors"`     // 5xx
	AverageLatencyMs float64       `json:"average_latency_ms"`
	RequestsPerMin   float64       `json:"requests_per_min"`
	Endpoints        []EndpointStat `json:"endpoints,omitempty"` // top 10 endpoints
	mu               sync.RWMutex
	startTime        time.Time
	latencies        []time.Duration
	maxLatencies     int
}

// EndpointStat represents statistics for a single endpoint
type EndpointStat struct {
	Method   string  `json:"method"`
	Path     string  `json:"path"`
	Count    int64   `json:"count"`
	AvgMs    float64 `json:"avg_ms"`
	ErrorRate float64 `json:"error_rate"`
}

// ServiceMetrics combines all metrics for a service
type ServiceMetrics struct {
	ServiceName string          `json:"service_name"`
	Runtime     RuntimeMetrics  `json:"runtime"`
	HTTP        HTTPMetrics     `json:"http"`
	Uptime      string          `json:"uptime"`
	StartTime   time.Time       `json:"start_time"`
}

// GetRuntimeMetrics collects current Go runtime metrics
func GetRuntimeMetrics() RuntimeMetrics {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Calculate memory usage percentage
	var memUsagePercent float64
	if m.HeapSys > 0 {
		memUsagePercent = float64(m.HeapInuse) / float64(m.HeapSys) * 100
	}

	// Calculate GC CPU percentage (approximate)
	var gcCPUPercent float64
	if m.GCCPUFraction > 0 {
		gcCPUPercent = m.GCCPUFraction * 100
	}

	return RuntimeMetrics{
		MemoryAlloc:      m.Alloc,
		MemoryTotalAlloc: m.TotalAlloc,
		MemorySys:        m.Sys,
		MemoryHeapAlloc:  m.HeapAlloc,
		MemoryHeapInuse:  m.HeapInuse,
		MemoryStackInuse: m.StackInuse,
		MemoryUsagePercent: memUsagePercent,
		NumGoroutines:    runtime.NumGoroutine(),
		NumGC:            m.NumGC,
		PauseNs:          m.PauseNs[(m.NumGC+255)%256],
		PauseTotal:       m.PauseTotalNs,
		GCCPUPercent:     gcCPUPercent,
		NumCPU:           runtime.NumCPU(),
		NumCGoCalls:      runtime.NumCgoCall(),
		Timestamp:        time.Now(),
	}
}

// NewHTTPMetrics creates a new HTTP metrics collector
func NewHTTPMetrics() *HTTPMetrics {
	return &HTTPMetrics{
		startTime:    time.Now(),
		latencies:    make([]time.Duration, 0, 1000),
		maxLatencies: 1000,
	}
}

// RecordRequest records a single HTTP request
func (m *HTTPMetrics) RecordRequest(statusCode int, latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalRequests++

	// Categorize by status code
	if statusCode >= 200 && statusCode < 300 {
		m.SuccessRequests++
	} else if statusCode >= 400 && statusCode < 500 {
		m.ClientErrors++
	} else if statusCode >= 500 {
		m.ServerErrors++
	}

	// Store latency (keep only last N)
	m.latencies = append(m.latencies, latency)
	if len(m.latencies) > m.maxLatencies {
		m.latencies = m.latencies[1:]
	}
}

// GetSnapshot returns current metrics snapshot
func (m *HTTPMetrics) GetSnapshot() HTTPMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot := HTTPMetrics{
		TotalRequests:   m.TotalRequests,
		SuccessRequests: m.SuccessRequests,
		ClientErrors:    m.ClientErrors,
		ServerErrors:    m.ServerErrors,
	}

	// Calculate average latency
	if len(m.latencies) > 0 {
		var sum time.Duration
		for _, l := range m.latencies {
			sum += l
		}
		snapshot.AverageLatencyMs = float64(sum.Milliseconds()) / float64(len(m.latencies))
	}

	// Calculate requests per minute
	elapsed := time.Since(m.startTime).Minutes()
	if elapsed > 0 {
		snapshot.RequestsPerMin = float64(m.TotalRequests) / elapsed
	}

	return snapshot
}

// FormatBytes formats bytes to human readable format
func FormatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
