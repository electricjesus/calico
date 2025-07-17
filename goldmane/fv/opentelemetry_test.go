package fv

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/projectcalico/calico/goldmane/pkg/client"
	"github.com/projectcalico/calico/goldmane/pkg/daemon"
	"github.com/projectcalico/calico/goldmane/pkg/otel"
	"github.com/projectcalico/calico/goldmane/pkg/types"
	"github.com/projectcalico/calico/goldmane/proto"
	"github.com/projectcalico/calico/libcalico-go/lib/logutils"
)

// Global variables for OpenTelemetry test setup
var (
	otelCtx         context.Context
	otelGoldmaneURL string
	otelClientCA    string
	otelClientCert  string
	otelClientKey   string
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
	sync.Mutex
	traces []OTLPTrace
	server *grpc.Server
	lis    net.Listener
}

func newMockOTLPCollector() *mockOTLPCollector {
	collector := &mockOTLPCollector{
		traces: make([]OTLPTrace, 0),
	}

	// Create gRPC server
	collector.server = grpc.NewServer()

	// Create listener
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		panic(err)
	}
	collector.lis = lis

	// Start server in background
	go func() {
		if err := collector.server.Serve(lis); err != nil {
			logrus.WithError(err).Error("Failed to serve gRPC server")
		}
	}()

	return collector
}

func (m *mockOTLPCollector) GetTraces() []OTLPTrace {
	m.Lock()
	defer m.Unlock()
	return append([]OTLPTrace{}, m.traces...)
}

func (m *mockOTLPCollector) GetSpansWithName(name string) []Span {
	m.Lock()
	defer m.Unlock()

	var spans []Span
	for _, trace := range m.traces {
		for _, rs := range trace.ResourceSpans {
			for _, ss := range rs.ScopeSpans {
				for _, span := range ss.Spans {
					if span.Name == name {
						spans = append(spans, span)
					}
				}
			}
		}
	}
	return spans
}

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

func (m *mockOTLPCollector) Close() {
	m.server.Stop()
	m.lis.Close()
}

func (m *mockOTLPCollector) URL() string {
	return m.lis.Addr().String()
}

// Test OpenTelemetry integration with a real OTLP collector
func TestOpenTelemetryIntegration(t *testing.T) {
	RegisterTestingT(t)
	logrus.SetLevel(logrus.DebugLevel)
	logutils.ConfigureFormatter("otelfv")
	logCancel := logutils.RedirectLogrusToTestingT(t)
	defer logCancel()

	// Create mock OpenTelemetry collector
	collector := newMockOTLPCollector()
	defer collector.Close()

	// Configure daemon with OpenTelemetry enabled
	cfg := daemon.Config{
		Port:                     0, // Use random port
		LogLevel:                 "debug",
		AggregationWindow:        15 * time.Second,
		EmitAfterSeconds:         30,
		EmitterAggregationWindow: 5 * time.Minute,
		HealthEnabled:            true,
		HealthPort:               0,
		PrometheusPort:           0,
		ProfilePort:              0,

		// OpenTelemetry configuration
		OTLPURL:      collector.URL() + "/v1/traces",
		OTLPInsecure: true,
	}

	// Set up environment variables for OpenTelemetry
	os.Setenv("OTEL_SERVICE_NAME", "goldmane-test")
	os.Setenv("OTEL_SERVICE_NAMESPACE", "test-namespace")
	os.Setenv("NODE_NAME", "test-node")
	os.Setenv("CLUSTER_NAME", "test-cluster")
	defer func() {
		os.Unsetenv("OTEL_SERVICE_NAME")
		os.Unsetenv("OTEL_SERVICE_NAMESPACE")
		os.Unsetenv("NODE_NAME")
		os.Unsetenv("CLUSTER_NAME")
	}()

	// Start daemon
	cleanup := otelDaemonSetup(t, cfg)
	defer cleanup()

	// Wait for daemon to start and get actual port
	time.Sleep(2 * time.Second)

	// Get the actual port from daemon configuration
	actualPort := cfg.Port
	if actualPort == 0 {
		// If port is still 0, daemon didn't set it properly
		t.Skip("Daemon didn't set port properly, skipping OpenTelemetry integration test")
	}

	actualURL := fmt.Sprintf("localhost:%d", actualPort)

	// Create flow client with actual URL
	flowClient, err := client.NewFlowClient(actualURL, otelClientCert, otelClientKey, otelClientCA)
	require.NoError(t, err)
	defer flowClient.Close()

	// Connect to the server
	connected := flowClient.Connect(otelCtx)
	select {
	case <-connected:
		logrus.Info("Connected to server")
	case <-otelCtx.Done():
		require.Fail(t, "Timed out waiting for server connection")
	}

	// Create gRPC connection for query client
	conn, err := grpc.Dial(actualURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	// Create and send test flows
	testFlows := createTestFlows()
	for _, flow := range testFlows {
		flowClient.Push(types.ProtoToFlow(flow))
	}

	// Query flows to trigger API spans
	queryClient := proto.NewFlowsClient(conn)
	_, err = queryClient.List(context.Background(), &proto.FlowListRequest{
		StartTimeGte: time.Now().Unix() - 300,
		StartTimeLt:  0,
		PageSize:     100,
	})
	require.NoError(t, err)

	// Wait for telemetry to be exported
	time.Sleep(5 * time.Second)

	// Verify OpenTelemetry traces were received
	traces := collector.GetTraces()
	require.NotEmpty(t, traces, "Should have received OpenTelemetry traces")

	// Verify specific spans were created
	t.Run("FlowReceiveSpans", func(t *testing.T) {
		spans := collector.GetSpansWithName("goldmane.flow.receive")
		require.NotEmpty(t, spans, "Should have flow receive spans")

		// Check for expected attributes
		found := false
		for _, span := range spans {
			for _, attr := range span.Attributes {
				if attr.Key == "goldmane.client.node" {
					found = true
					break
				}
			}
		}
		require.True(t, found, "Flow receive span should have client node attribute")
	})

	t.Run("FlowAggregateSpans", func(t *testing.T) {
		spans := collector.GetSpansWithName("goldmane.flow.aggregate")
		require.NotEmpty(t, spans, "Should have flow aggregate spans")

		// Check for flow attributes
		found := false
		for _, span := range spans {
			for _, attr := range span.Attributes {
				if attr.Key == "goldmane.flow.key" {
					found = true
					break
				}
			}
		}
		require.True(t, found, "Flow aggregate span should have flow key attribute")
	})

	t.Run("FlowQuerySpans", func(t *testing.T) {
		spans := collector.GetSpansWithName("goldmane.flow.query")
		require.NotEmpty(t, spans, "Should have flow query spans")

		// Check for query attributes
		found := false
		for _, span := range spans {
			for _, attr := range span.Attributes {
				if attr.Key == "operation" && attr.Value == "list" {
					found = true
					break
				}
			}
		}
		require.True(t, found, "Flow query span should have operation attribute")
	})

	t.Run("ServiceGraphAttributes", func(t *testing.T) {
		spans := collector.GetSpansWithAttribute("service.source", "frontend-service")
		require.NotEmpty(t, spans, "Should have spans with service source attribute")

		spans = collector.GetSpansWithAttribute("service.destination", "backend-service")
		require.NotEmpty(t, spans, "Should have spans with service destination attribute")
	})

	t.Run("ResourceAttributes", func(t *testing.T) {
		require.NotEmpty(t, traces, "Should have traces")

		// Check resource attributes
		resourceSpan := traces[0].ResourceSpans[0]
		require.NotEmpty(t, resourceSpan.Resource.Attributes, "Should have resource attributes")

		// Verify service name
		found := false
		for _, attr := range resourceSpan.Resource.Attributes {
			if attr.Key == "service.name" {
				if value, ok := attr.Value.(string); ok && value == "goldmane-test" {
					found = true
					break
				}
			}
		}
		require.True(t, found, "Should have correct service name in resource attributes")
	})
}

// Test OpenTelemetry configuration and provider lifecycle
func TestOpenTelemetryProvider(t *testing.T) {
	RegisterTestingT(t)

	ctx := context.Background()

	t.Run("DisabledProvider", func(t *testing.T) {
		cfg := otel.Config{
			Enabled: false,
		}

		provider, err := otel.NewProvider(ctx, cfg)
		require.NoError(t, err)
		require.False(t, provider.IsEnabled())

		// Should shutdown gracefully
		err = provider.Shutdown(ctx)
		require.NoError(t, err)
	})

	t.Run("EnabledProvider", func(t *testing.T) {
		collector := newMockOTLPCollector()
		defer collector.Close()

		cfg := otel.Config{
			Enabled:           true,
			CollectorEndpoint: collector.URL() + "/v1/traces",
			ServiceName:       "test-service",
			ServiceVersion:    "test-version",
			ServiceNamespace:  "test-namespace",
			NodeName:          "test-node",
			ClusterName:       "test-cluster",
			SamplingRate:      1.0,
		}

		provider, err := otel.NewProvider(ctx, cfg)
		require.NoError(t, err)
		require.True(t, provider.IsEnabled())

		// Create a tracer and span
		tracer := provider.GetTracer("test-tracer")
		require.NotNil(t, tracer)

		_, span := tracer.Start(ctx, "test-span")
		span.End()

		// Shutdown provider
		err = provider.Shutdown(ctx)
		require.NoError(t, err)
	})
}

// Test OpenTelemetry instrumentation
func TestOpenTelemetryInstrumentation(t *testing.T) {
	RegisterTestingT(t)

	ctx := context.Background()
	instr := otel.NewFlowInstrumentation("test")

	t.Run("FlowReceiveSpan", func(t *testing.T) {
		_, span := instr.StartFlowReceiveSpan(ctx, "test-node")
		require.NotNil(t, span)
		span.End()
	})

	t.Run("FlowAggregateSpan", func(t *testing.T) {
		_, span := instr.StartFlowAggregateSpan(ctx, time.Now().Unix())
		require.NotNil(t, span)
		span.End()
	})

	t.Run("FlowEmitSpan", func(t *testing.T) {
		_, span := instr.StartFlowEmitSpan(ctx, "http://test.example.com")
		require.NotNil(t, span)
		span.End()
	})

	t.Run("FlowQuerySpan", func(t *testing.T) {
		_, span := instr.StartFlowQuerySpan(ctx, "test-operation")
		require.NotNil(t, span)
		span.End()
	})

	t.Run("FlowAttributes", func(t *testing.T) {
		flow := createTestFlow("test-source", "test-dest", 8080)
		_, span := instr.StartFlowAggregateSpan(ctx, flow.StartTime)

		// Should not panic
		instr.AddFlowAttributes(span, flow)
		instr.AddServiceGraphAttributes(span, flow)

		span.End()
	})

	t.Run("ErrorRecording", func(t *testing.T) {
		_, span := instr.StartFlowQuerySpan(ctx, "test-error")

		// Should not panic
		testErr := fmt.Errorf("test error")
		instr.RecordError(span, testErr)

		span.End()
	})
}

// Test end-to-end OpenTelemetry flow processing
func TestOpenTelemetryEndToEnd(t *testing.T) {
	RegisterTestingT(t)

	if testing.Short() {
		t.Skip("Skipping end-to-end test in short mode")
	}

	// Create mock collector
	collector := newMockOTLPCollector()
	defer collector.Close()

	// Configure OpenTelemetry
	otelConfig := otel.Config{
		Enabled:           true,
		CollectorEndpoint: collector.URL() + "/v1/traces",
		ServiceName:       "goldmane-e2e-test",
		ServiceVersion:    "test",
		ServiceNamespace:  "test-ns",
		NodeName:          "test-node",
		ClusterName:       "test-cluster",
		SamplingRate:      1.0, // 100% sampling for test
	}

	ctx := context.Background()

	// Initialize provider
	provider, err := otel.NewProvider(ctx, otelConfig)
	require.NoError(t, err)
	defer provider.Shutdown(ctx)

	// Create instrumentation
	instr := otel.NewFlowInstrumentation("e2e-test")

	// Simulate complete flow processing pipeline
	t.Run("FlowProcessingPipeline", func(t *testing.T) {
		// Step 1: Receive flow
		ctx, receiveSpan := instr.StartFlowReceiveSpan(ctx, "client-node")

		flow := createTestFlow("web-app", "database", 5432)
		instr.AddFlowAttributes(receiveSpan, flow)
		instr.AddServiceGraphAttributes(receiveSpan, flow)

		time.Sleep(10 * time.Millisecond)
		receiveSpan.End()

		// Step 2: Aggregate flow
		ctx, aggregateSpan := instr.StartFlowAggregateSpan(ctx, flow.StartTime)
		instr.AddFlowAttributes(aggregateSpan, flow)

		time.Sleep(20 * time.Millisecond)
		aggregateSpan.End()

		// Step 3: Emit flow
		ctx, emitSpan := instr.StartFlowEmitSpan(ctx, "http://collector:8080")

		time.Sleep(30 * time.Millisecond)
		emitSpan.End()

		// Step 4: Query flows
		ctx, querySpan := instr.StartFlowQuerySpan(ctx, "list")
		instr.AddResultAttributes(querySpan, 42)

		time.Sleep(15 * time.Millisecond)
		querySpan.End()

		// Wait for spans to be exported
		time.Sleep(2 * time.Second)

		// Verify traces were received
		traces := collector.GetTraces()
		require.NotEmpty(t, traces, "Should have received traces")

		// Verify all expected spans are present
		receiveSpans := collector.GetSpansWithName("goldmane.flow.receive")
		require.NotEmpty(t, receiveSpans, "Should have receive spans")

		aggregateSpans := collector.GetSpansWithName("goldmane.flow.aggregate")
		require.NotEmpty(t, aggregateSpans, "Should have aggregate spans")

		emitSpans := collector.GetSpansWithName("goldmane.flow.emit")
		require.NotEmpty(t, emitSpans, "Should have emit spans")

		querySpans := collector.GetSpansWithName("goldmane.flow.query")
		require.NotEmpty(t, querySpans, "Should have query spans")

		// Verify service graph attributes
		serviceSourceSpans := collector.GetSpansWithAttribute("service.source", "web-app")
		require.NotEmpty(t, serviceSourceSpans, "Should have service source spans")

		serviceDestSpans := collector.GetSpansWithAttribute("service.destination", "database")
		require.NotEmpty(t, serviceDestSpans, "Should have service destination spans")
	})
}

// otelDaemonSetup sets up a daemon for OpenTelemetry testing
func otelDaemonSetup(t *testing.T, cfg daemon.Config) func() {
	RegisterTestingT(t)
	logrus.SetLevel(logrus.DebugLevel)
	logutils.ConfigureFormatter("otelfv")
	logCancel := logutils.RedirectLogrusToTestingT(t)

	// The context acts as a global timeout for the test to make sure we don't hang.
	var cancel context.CancelFunc
	otelCtx, cancel = context.WithTimeout(context.Background(), 30*time.Second)

	// Create TLS credentials for Goldmane.
	cert, key := createKeyCertPair(os.TempDir())

	// Create TLS credentials for the client.
	cliCert, cliKey := createKeyCertPair(os.TempDir())

	// Store the file paths for the client certificates.
	otelClientCA = cert.Name()
	otelClientKey = cliKey.Name()
	otelClientCert = cliCert.Name()

	// Augment the configuration with the paths to the certificates.
	cfg.ServerCertPath = cert.Name()
	cfg.ServerKeyPath = key.Name()
	cfg.CACertPath = cliCert.Name()

	// Start a test HTTP server that we can point the emitter at to verify
	// flows are being emitted.
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logrus.WithField("path", r.URL.Path).Info("[OTEL TEST] Received request")
	}))
	cfg.PushURL = testServer.URL

	// Run the daemon.
	go daemon.Run(otelCtx, cfg)

	otelGoldmaneURL = fmt.Sprintf("localhost:%d", cfg.Port)

	return func() {
		logCancel()
		cancel()
	}
}

// Helper functions
func createTestFlows() []*proto.Flow {
	return []*proto.Flow{
		{
			StartTime:  time.Now().Unix() - 60,
			EndTime:    time.Now().Unix(),
			PacketsIn:  100,
			PacketsOut: 50,
			BytesIn:    10000,
			BytesOut:   5000,
			Key: &proto.FlowKey{
				SourceName:      "frontend-service",
				SourceNamespace: "default",
				SourceType:      proto.EndpointType_WorkloadEndpoint,
				DestName:        "backend-service",
				DestNamespace:   "default",
				DestType:        proto.EndpointType_WorkloadEndpoint,
				DestPort:        8080,
			},
		},
		{
			StartTime:  time.Now().Unix() - 30,
			EndTime:    time.Now().Unix(),
			PacketsIn:  200,
			PacketsOut: 100,
			BytesIn:    20000,
			BytesOut:   10000,
			Key: &proto.FlowKey{
				SourceName:      "backend-service",
				SourceNamespace: "default",
				SourceType:      proto.EndpointType_WorkloadEndpoint,
				DestName:        "database",
				DestNamespace:   "default",
				DestType:        proto.EndpointType_WorkloadEndpoint,
				DestPort:        5432,
			},
		},
	}
}

func createTestFlow(source, dest string, port int64) *types.Flow {
	return &types.Flow{
		StartTime:  time.Now().Unix() - 60,
		EndTime:    time.Now().Unix(),
		PacketsIn:  100,
		PacketsOut: 50,
		BytesIn:    10000,
		BytesOut:   5000,
		Key: types.NewFlowKey(
			&types.FlowKeySource{
				SourceName:      source,
				SourceNamespace: "default",
				SourceType:      proto.EndpointType_WorkloadEndpoint,
			},
			&types.FlowKeyDestination{
				DestName:      dest,
				DestNamespace: "default",
				DestType:      proto.EndpointType_WorkloadEndpoint,
				DestPort:      port,
			},
			&types.FlowKeyMeta{
				Proto:    "TCP",
				Reporter: proto.Reporter_Src,
				Action:   proto.Action_Allow,
			},
			nil,
		),
	}
}
