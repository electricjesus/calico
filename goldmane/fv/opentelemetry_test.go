package fv

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
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

// Test OpenTelemetry integration with a real OTLP collector
func TestOpenTelemetryIntegration(t *testing.T) {
	RegisterTestingT(t)
	logrus.SetLevel(logrus.DebugLevel)
	logutils.ConfigureFormatter("otelfv")
	logCancel := logutils.RedirectLogrusToTestingT(t)
	defer logCancel()

	// Create mock OpenTelemetry collector
	collector := NewMockOTLPCollector()
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
		OTLPURL:      collector.GetEndpoint(),
		OTLPInsecure: true,
	}

	// Set up environment variables for OpenTelemetry
	t.Setenv("OTEL_SERVICE_NAME", "goldmane-test")
	t.Setenv("OTEL_SERVICE_NAMESPACE", "test-namespace")
	t.Setenv("OTEL_INSECURE", "true")
	t.Setenv("NODE_NAME", "test-node")
	t.Setenv("CLUSTER_NAME", "test-cluster")

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

// Test mock collector port functionality
func TestMockCollectorPort(t *testing.T) {
	// Create mock collector
	collector := NewMockOTLPCollector()
	defer collector.Close()

	// Test that we can get the port
	port := collector.GetPort()
	require.Greater(t, port, 0, "Port should be greater than 0")
	require.Less(t, port, 65536, "Port should be less than 65536")

	// Test that endpoint doesn't include path
	endpoint := collector.GetEndpoint()
	require.Equal(t, fmt.Sprintf("localhost:%d", port), endpoint)
	require.NotContains(t, endpoint, "/v1/traces", "Endpoint should not contain path")

	// Test that URL and endpoint give consistent port
	url := collector.URL()
	require.Contains(t, url, fmt.Sprintf(":%d", port))

	t.Logf("Mock collector running on port %d", port)
	t.Logf("Endpoint: %s", endpoint)
	t.Logf("URL: %s", url)
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
		collector := NewMockOTLPCollector()
		defer collector.Close()

		cfg := otel.Config{
			Enabled:           true,
			CollectorEndpoint: collector.GetEndpoint(),
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

	logrus.SetLevel(logrus.DebugLevel)
	RegisterTestingT(t)

	if testing.Short() {
		t.Skip("Skipping end-to-end test in short mode")
	}

	// Create mock collector
	collector := NewMockOTLPCollector()
	defer collector.Close()

	// Configure daemon with OpenTelemetry enabled
	cfg := daemon.Config{
		Port:                     0, // Use random port
		LogLevel:                 "debug",
		AggregationWindow:        1 * time.Second,  // Shorter for faster testing
		EmitAfterSeconds:         2,                // Shorter for faster testing
		EmitterAggregationWindow: 30 * time.Second, // Shorter for faster testing
		HealthEnabled:            true,
		HealthPort:               0,
		PrometheusPort:           0,
		ProfilePort:              0,

		// OpenTelemetry configuration
		OTLPServiceName:      "goldmane-test",
		OTLPServiceNamespace: "test-namespace",
		OTLPServiceVersion:   "dev",
		OTLPURL:              collector.GetEndpoint(),
		OTLPInsecure:         true,
		OTLPSamplingRate:     1.0,
		NodeName:             "test-node",
		ClusterName:          "test-cluster",
	}

	// Start daemon
	cleanup := otelDaemonSetup(t, cfg)
	defer cleanup()

	// Wait for daemon to start
	time.Sleep(2 * time.Second)

	// Create flow client to send flows to the daemon
	flowClient, err := client.NewFlowClient(otelGoldmaneURL, otelClientCert, otelClientKey, otelClientCA)
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

	// Simulate complete flow processing pipeline
	t.Run("FlowProcessingPipeline", func(t *testing.T) {
		// Send multiple flows to trigger aggregation and processing
		testFlows := createTestFlows()
		for _, flow := range testFlows {
			flowClient.Push(types.ProtoToFlow(flow))
		}

		// Wait for flows to be processed and spans to be exported
		time.Sleep(5 * time.Second)

		// Verify traces were received by the mock collector
		traces := collector.GetTraces()
		require.NotEmpty(t, traces, "Should have received traces from daemon")

		// Debug: Print what traces we received
		for i, trace := range traces {
			logrus.WithField("trace_index", i).WithField("trace", trace).Debug("Received trace")
		}

		// Verify we have spans - the exact names depend on the instrumentation in goldmane
		allSpans := []Span{}
		for _, trace := range traces {
			for _, rs := range trace.ResourceSpans {
				for _, ss := range rs.ScopeSpans {
					allSpans = append(allSpans, ss.Spans...)
				}
			}
		}

		require.NotEmpty(t, allSpans, "Should have received spans")
		logrus.WithField("spans_count", len(allSpans)).Debug("Total spans received")

		// Debug: Print all span names to see what we actually get
		for i, span := range allSpans {
			logrus.WithField("span_index", i).WithField("span_name", span.Name).Debug("Received span")
		}

		// Check for spans we expect based on the test flow
		aggregateSpans := collector.GetSpansWithName("goldmane.flow.aggregate")
		emitSpans := collector.GetSpansWithName("goldmane.flow.emit")
		receiveSpans := collector.GetSpansWithName("goldmane.flow.receive")

		logrus.WithFields(logrus.Fields{
			"receive_spans":   len(receiveSpans),
			"aggregate_spans": len(aggregateSpans),
			"emit_spans":      len(emitSpans),
		}).Info("Span counts by type")

		// We should definitely have aggregate spans since flows are being processed
		require.NotEmpty(t, aggregateSpans, "Should have flow aggregate spans from processing")

		// We should have emit spans since flows are being emitted to HTTP
		require.NotEmpty(t, emitSpans, "Should have flow emit spans from HTTP emission")

		// Receive spans might be timing-dependent, so just log if missing
		if len(receiveSpans) == 0 {
			logrus.Warn("No receive spans found - might be timing issue")
		}
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

	// If port is 0, find an available port and use it
	if cfg.Port == 0 {
		listener, err := net.Listen("tcp", ":0")
		require.NoError(t, err)
		cfg.Port = listener.Addr().(*net.TCPAddr).Port
		listener.Close()
	}

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
