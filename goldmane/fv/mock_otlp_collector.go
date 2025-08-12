package fv

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/sirupsen/logrus"
	otlptracev1 "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	otlpcommonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	"google.golang.org/grpc"
)

// OpenTelemetry trace data structures for verification
type OTLPTrace struct {
	ResourceSpans []ResourceSpan `json:"resourceSpans"`
}

type ResourceSpan struct {
	Resource   Resource    `json:"resource"`
	ScopeSpans []ScopeSpan `json:"scopeSpans"`
}

type Resource struct {
	Attributes []Attribute `json:"attributes"`
}

type ScopeSpan struct {
	Scope string `json:"scope"`
	Spans []Span `json:"spans"`
}

type Span struct {
	TraceID           string      `json:"traceId"`
	SpanID            string      `json:"spanId"`
	ParentSpanID      string      `json:"parentSpanId,omitempty"`
	Name              string      `json:"name"`
	Kind              int         `json:"kind"`
	StartTimeUnixNano string      `json:"startTimeUnixNano"`
	EndTimeUnixNano   string      `json:"endTimeUnixNano"`
	Attributes        []Attribute `json:"attributes"`
	Status            Status      `json:"status"`
}

type Attribute struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

type Status struct {
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
}

// Mock OpenTelemetry Collector using gRPC
type mockOTLPCollector struct {
	otlptracev1.UnimplementedTraceServiceServer
	sync.Mutex
	traces []OTLPTrace
	server *grpc.Server
	lis    net.Listener
	port   int
	// Store raw OTLP requests for debugging
	requests []*otlptracev1.ExportTraceServiceRequest
}

// NewMockOTLPCollector creates a new mock OTLP collector for testing
func NewMockOTLPCollector() *mockOTLPCollector {
	collector := &mockOTLPCollector{
		traces:   make([]OTLPTrace, 0),
		requests: make([]*otlptracev1.ExportTraceServiceRequest, 0),
	}

	// Create gRPC server
	collector.server = grpc.NewServer()

	// Register the OTLP trace service
	otlptracev1.RegisterTraceServiceServer(collector.server, collector)

	// Create listener on random port
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		panic(err)
	}
	collector.lis = lis

	// Extract the actual port number
	if tcpAddr, ok := lis.Addr().(*net.TCPAddr); ok {
		collector.port = tcpAddr.Port
	} else {
		panic("Failed to get TCP address from listener")
	}

	// Start server in background
	go func() {
		if err := collector.server.Serve(lis); err != nil {
			logrus.WithError(err).Debug("gRPC server stopped")
		}
	}()

	return collector
}

// Export implements the OTLP TraceServiceServer interface
func (m *mockOTLPCollector) Export(ctx context.Context, req *otlptracev1.ExportTraceServiceRequest) (*otlptracev1.ExportTraceServiceResponse, error) {
	m.Lock()
	defer m.Unlock()

	// Store the raw request for debugging
	m.requests = append(m.requests, req)

	// Convert OTLP request to our trace format for testing
	// For simplicity, we're just tracking that we received the request
	// A full implementation would convert the OTLP spans to our OTLPTrace format
	logrus.WithField("spans_count", len(req.ResourceSpans)).Debug("Mock OTLP collector received traces")

	// Create a simple trace record to indicate we received something
	if len(req.ResourceSpans) > 0 {
		trace := OTLPTrace{
			ResourceSpans: make([]ResourceSpan, len(req.ResourceSpans)),
		}

		for i, rs := range req.ResourceSpans {
			resourceSpan := ResourceSpan{
				Resource: Resource{
					Attributes: make([]Attribute, 0),
				},
				ScopeSpans: make([]ScopeSpan, len(rs.ScopeSpans)),
			}

			for j, ss := range rs.ScopeSpans {
				scopeSpan := ScopeSpan{
					Scope: ss.Scope.Name,
					Spans: make([]Span, len(ss.Spans)),
				}

				for k, span := range ss.Spans {
					scopeSpan.Spans[k] = Span{
						TraceID:           fmt.Sprintf("%x", span.TraceId),
						SpanID:            fmt.Sprintf("%x", span.SpanId),
						Name:              span.Name,
						Kind:              int(span.Kind),
						StartTimeUnixNano: fmt.Sprintf("%d", span.StartTimeUnixNano),
						EndTimeUnixNano:   fmt.Sprintf("%d", span.EndTimeUnixNano),
						Attributes:        make([]Attribute, len(span.Attributes)),
						Status: Status{
							Code:    int(span.Status.Code),
							Message: span.Status.Message,
						},
					}

					// Convert attributes
					for l, attr := range span.Attributes {
						var value interface{}
						switch v := attr.Value.Value.(type) {
						case *otlpcommonv1.AnyValue_StringValue:
							value = v.StringValue
						case *otlpcommonv1.AnyValue_IntValue:
							value = v.IntValue
						case *otlpcommonv1.AnyValue_DoubleValue:
							value = v.DoubleValue
						case *otlpcommonv1.AnyValue_BoolValue:
							value = v.BoolValue
						default:
							value = "unknown"
						}

						scopeSpan.Spans[k].Attributes[l] = Attribute{
							Key:   attr.Key,
							Value: value,
						}
					}
				}

				resourceSpan.ScopeSpans[j] = scopeSpan
			}

			trace.ResourceSpans[i] = resourceSpan
		}

		m.traces = append(m.traces, trace)
	}

	return &otlptracev1.ExportTraceServiceResponse{}, nil
}

// GetTraces returns all traces received by the mock collector
func (m *mockOTLPCollector) GetTraces() []OTLPTrace {
	m.Lock()
	defer m.Unlock()
	return append([]OTLPTrace{}, m.traces...)
}

// GetSpansWithName returns all spans with the specified name
func (m *mockOTLPCollector) GetSpansWithName(name string) []Span {
	m.Lock()
	defer m.Unlock()

	var spans []Span
	logrus.WithField("spanName", name).Debug("Searching for spans with name")
	logrus.WithField("traceCount", len(m.traces)).Debug("Total traces to search")
	logrus.WithField("spanCount", len(spans)).Debug("Initial span count")
	for _, trace := range m.traces {
		logrus.WithField("traceSpans", len(trace.ResourceSpans)).Debug("Processing trace spans")
		for _, rs := range trace.ResourceSpans {
			logrus.WithField("scopeSpansCount", len(rs.ScopeSpans)).Debug("Processing resource spans")
			for _, ss := range rs.ScopeSpans {
				logrus.WithField("scopeName", ss.Scope).Debug("Processing scope spans")
				for _, span := range ss.Spans {
					if span.Name == name {
						spans = append(spans, span)
					} else {
						logrus.WithField("spanName", span.Name).Debug("Skipping span with different name")
					}
				}
			}
		}
	}
	return spans
}

// GetSpansWithAttribute returns all spans that have an attribute with the specified key and value
func (m *mockOTLPCollector) GetSpansWithAttribute(key, value string) []Span {
	m.Lock()
	defer m.Unlock()

	var spans []Span
	for _, trace := range m.traces {
		for _, rs := range trace.ResourceSpans {
			for _, ss := range rs.ScopeSpans {
				for _, span := range ss.Spans {
					for _, attr := range span.Attributes {
						if attr.Key == key {
							if attrValue, ok := attr.Value.(string); ok && attrValue == value {
								spans = append(spans, span)
							}
						}
					}
				}
			}
		}
	}
	return spans
}

// GetAllSpans returns all spans from all traces
func (m *mockOTLPCollector) GetAllSpans() []Span {
	m.Lock()
	defer m.Unlock()

	var allSpans []Span
	for _, trace := range m.traces {
		for _, rs := range trace.ResourceSpans {
			for _, ss := range rs.ScopeSpans {
				allSpans = append(allSpans, ss.Spans...)
			}
		}
	}
	return allSpans
}

// GetRawRequests returns all raw OTLP requests received
func (m *mockOTLPCollector) GetRawRequests() []*otlptracev1.ExportTraceServiceRequest {
	m.Lock()
	defer m.Unlock()
	return append([]*otlptracev1.ExportTraceServiceRequest{}, m.requests...)
}

// Close stops the mock collector and releases resources
func (m *mockOTLPCollector) Close() {
	m.server.Stop()
	m.lis.Close()
}

// URL returns the full address string of the collector
func (m *mockOTLPCollector) URL() string {
	return m.lis.Addr().String()
}

// GetPort returns the port the collector is listening on
func (m *mockOTLPCollector) GetPort() int {
	return m.port
}

// GetEndpoint returns the gRPC endpoint (host:port) without any path
func (m *mockOTLPCollector) GetEndpoint() string {
	return fmt.Sprintf("localhost:%d", m.port)
}

// Reset clears all collected traces and requests
func (m *mockOTLPCollector) Reset() {
	m.Lock()
	defer m.Unlock()
	m.traces = make([]OTLPTrace, 0)
	m.requests = make([]*otlptracev1.ExportTraceServiceRequest, 0)
}
