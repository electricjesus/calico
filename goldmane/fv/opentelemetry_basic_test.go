package fv

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/projectcalico/calico/goldmane/pkg/otel"
	"github.com/projectcalico/calico/goldmane/pkg/types"
	"github.com/projectcalico/calico/goldmane/proto"
)

// TestOpenTelemetryBasic tests basic OpenTelemetry functionality
func TestOpenTelemetryBasic(t *testing.T) {
	// Test disabled OpenTelemetry
	t.Run("DisabledProvider", func(t *testing.T) {
		cfg := otel.Config{
			Enabled: false,
		}

		provider, err := otel.NewProvider(context.Background(), cfg)
		require.NoError(t, err)
		require.False(t, provider.IsEnabled())

		// Should shutdown gracefully
		err = provider.Shutdown(context.Background())
		require.NoError(t, err)
	})

	// Test instrumentation without real collector
	t.Run("InstrumentationTest", func(t *testing.T) {
		ctx := context.Background()
		instr := otel.NewFlowInstrumentation("test")

		// Test flow receive span
		_, span := instr.StartFlowReceiveSpan(ctx, "test-node")
		require.NotNil(t, span)
		span.End()

		// Test flow aggregate span
		_, span = instr.StartFlowAggregateSpan(ctx, time.Now().Unix())
		require.NotNil(t, span)
		span.End()

		// Test flow emit span
		_, span = instr.StartFlowEmitSpan(ctx, "http://test.example.com")
		require.NotNil(t, span)
		span.End()

		// Test flow query span
		_, span = instr.StartFlowQuerySpan(ctx, "test-operation")
		require.NotNil(t, span)
		span.End()
	})

	// Test flow attributes
	t.Run("FlowAttributesTest", func(t *testing.T) {
		ctx := context.Background()
		instr := otel.NewFlowInstrumentation("test")

		flow := &types.Flow{
			StartTime:  time.Now().Unix() - 60,
			EndTime:    time.Now().Unix(),
			PacketsIn:  100,
			PacketsOut: 50,
			BytesIn:    10000,
			BytesOut:   5000,
			Key: types.NewFlowKey(
				&types.FlowKeySource{
					SourceName:      "test-source",
					SourceNamespace: "default",
					SourceType:      proto.EndpointType_WorkloadEndpoint,
				},
				&types.FlowKeyDestination{
					DestName:      "test-dest",
					DestNamespace: "default",
					DestType:      proto.EndpointType_WorkloadEndpoint,
					DestPort:      8080,
				},
				&types.FlowKeyMeta{
					Proto:    "TCP",
					Reporter: proto.Reporter_Src,
					Action:   proto.Action_Allow,
				},
				nil,
			),
		}

		_, span := instr.StartFlowAggregateSpan(ctx, flow.StartTime)

		// Should not panic
		instr.AddFlowAttributes(span, flow)
		instr.AddServiceGraphAttributes(span, flow)

		span.End()
	})

	// Test error recording
	t.Run("ErrorRecordingTest", func(t *testing.T) {
		ctx := context.Background()
		instr := otel.NewFlowInstrumentation("test")

		_, span := instr.StartFlowQuerySpan(ctx, "test-error")

		// Should not panic
		testErr := fmt.Errorf("test error")
		instr.RecordError(span, testErr)

		span.End()
	})

	// Test result attributes
	t.Run("ResultAttributesTest", func(t *testing.T) {
		ctx := context.Background()
		instr := otel.NewFlowInstrumentation("test")

		_, span := instr.StartFlowQuerySpan(ctx, "test-results")

		// Should not panic
		instr.AddResultAttributes(span, 42)

		span.End()
	})
}

// TestOpenTelemetryConfig tests OpenTelemetry configuration
func TestOpenTelemetryConfig(t *testing.T) {
	t.Run("ConfigFromEnv", func(t *testing.T) {
		// Test default config
		cfg := otel.ConfigFromEnv()
		require.False(t, cfg.Enabled) // Should be false without OTEL_ENABLED=true

		// Test with environment variables
		os.Setenv("OTEL_ENABLED", "true")
		os.Setenv("OTEL_SERVICE_NAME", "test-service")
		os.Setenv("OTEL_SERVICE_VERSION", "1.0.0")
		defer func() {
			os.Unsetenv("OTEL_ENABLED")
			os.Unsetenv("OTEL_SERVICE_NAME")
			os.Unsetenv("OTEL_SERVICE_VERSION")
		}()

		cfg = otel.ConfigFromEnv()
		require.True(t, cfg.Enabled)
		require.Equal(t, "test-service", cfg.ServiceName)
		require.Equal(t, "1.0.0", cfg.ServiceVersion)
	})
}
