package otel

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/projectcalico/calico/goldmane/pkg/types"
	"github.com/projectcalico/calico/goldmane/proto"
)

func TestOTelProvider(t *testing.T) {
	ctx := context.Background()

	// Test disabled provider
	cfg := Config{Enabled: false}
	provider, err := NewProvider(ctx, cfg)
	require.NoError(t, err)
	assert.False(t, provider.IsEnabled())

	// Test graceful shutdown
	err = provider.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestFlowInstrumentation(t *testing.T) {
	instr := NewFlowInstrumentation("test")
	assert.NotNil(t, instr)

	ctx := context.Background()

	// Test span creation
	ctx, span := instr.StartFlowReceiveSpan(ctx, "test-node")
	assert.NotNil(t, span)
	span.End()

	ctx, span = instr.StartFlowAggregateSpan(ctx, time.Now().Unix())
	assert.NotNil(t, span)
	span.End()

	ctx, span = instr.StartFlowEmitSpan(ctx, "http://test.example.com")
	assert.NotNil(t, span)
	span.End()

	ctx, span = instr.StartFlowQuerySpan(ctx, "list")
	assert.NotNil(t, span)
	span.End()
}

func TestFlowAttributes(t *testing.T) {
	instr := NewFlowInstrumentation("test")
	ctx := context.Background()

	// Test with types.Flow
	flow := &types.Flow{
		StartTime:  time.Now().Unix(),
		EndTime:    time.Now().Unix() + 60,
		PacketsIn:  100,
		PacketsOut: 50,
		BytesIn:    10000,
		BytesOut:   5000,
		Key: types.NewFlowKey(
			&types.FlowKeySource{
				SourceName:      "app-1",
				SourceNamespace: "default",
				SourceType:      proto.EndpointType_WorkloadEndpoint,
			},
			&types.FlowKeyDestination{
				DestName:      "app-2",
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
	instr.AddFlowAttributes(span, flow)
	instr.AddServiceGraphAttributes(span, flow)
	span.End()

	// Test with proto.Flow
	protoFlow := &proto.Flow{
		StartTime:  time.Now().Unix(),
		EndTime:    time.Now().Unix() + 60,
		PacketsIn:  100,
		PacketsOut: 50,
		BytesIn:    10000,
		BytesOut:   5000,
		Key: &proto.FlowKey{
			SourceName: "app-1",
			DestName:   "app-2",
			DestPort:   8080,
		},
	}

	_, span = instr.StartFlowQuerySpan(ctx, "test")
	instr.AddProtoFlowAttributes(span, protoFlow)
	span.End()
}

func TestQueryAttributes(t *testing.T) {
	instr := NewFlowInstrumentation("test")
	ctx := context.Background()

	// Test with FlowListRequest
	listReq := &proto.FlowListRequest{
		StartTimeGte:        time.Now().Unix() - 3600,
		StartTimeLt:         time.Now().Unix(),
		Page:                1,
		PageSize:            100,
		AggregationInterval: 15,
	}

	_, span := instr.StartFlowQuerySpan(ctx, "list")
	instr.AddQueryAttributes(span, listReq)
	span.End()

	// Test with FlowStreamRequest
	streamReq := &proto.FlowStreamRequest{
		StartTimeGte:        time.Now().Unix() - 3600,
		AggregationInterval: 15,
	}

	_, span = instr.StartFlowQuerySpan(ctx, "stream")
	instr.AddQueryAttributes(span, streamReq)
	span.End()
}

func TestErrorRecording(t *testing.T) {
	instr := NewFlowInstrumentation("test")
	ctx := context.Background()

	_, span := instr.StartFlowQuerySpan(ctx, "test")

	// Test error recording
	testErr := assert.AnError
	instr.RecordError(span, testErr)

	span.End()
}

func TestProtocolToString(t *testing.T) {
	tests := []struct {
		protocol int32
		expected string
	}{
		{6, "tcp"},
		{17, "udp"},
		{1, "icmp"},
		{99, "protocol-99"},
	}

	for _, tt := range tests {
		result := protocolToString(tt.protocol)
		assert.Equal(t, tt.expected, result)
	}
}

func TestExtractServiceName(t *testing.T) {
	tests := []struct {
		endpoint string
		expected string
	}{
		{"app-1", "app-1"},
		{"", "unknown"},
		{"192.168.1.1", "192.168.1.1"},
	}

	for _, tt := range tests {
		result := extractServiceName(tt.endpoint)
		assert.Equal(t, tt.expected, result)
	}
}
