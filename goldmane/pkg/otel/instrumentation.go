// Copyright (c) 2025 Tigera, Inc. All rights reserved.

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package otel

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/projectcalico/calico/goldmane/pkg/types"
	"github.com/projectcalico/calico/goldmane/proto"
)

// Common span names used throughout Goldmane
const (
	SpanFlowReceive    = "goldmane.flow.receive"
	SpanFlowAggregate  = "goldmane.flow.aggregate"
	SpanFlowEmit       = "goldmane.flow.emit"
	SpanFlowQuery      = "goldmane.flow.query"
	SpanFlowStream     = "goldmane.flow.stream"
	SpanBucketRollover = "goldmane.bucket.rollover"
	SpanStorageWrite   = "goldmane.storage.write"
	SpanStorageRead    = "goldmane.storage.read"
)

// Common attribute keys
const (
	AttrFlowKey         = "goldmane.flow.key"
	AttrFlowStartTime   = "goldmane.flow.start_time"
	AttrFlowEndTime     = "goldmane.flow.end_time"
	AttrFlowSource      = "goldmane.flow.source"
	AttrFlowDest        = "goldmane.flow.dest"
	AttrFlowProtocol    = "goldmane.flow.protocol"
	AttrFlowPackets     = "goldmane.flow.packets"
	AttrFlowBytes       = "goldmane.flow.bytes"
	AttrBucketTimestamp = "goldmane.bucket.timestamp"
	AttrClientNode      = "goldmane.client.node"
	AttrEmitURL         = "goldmane.emit.url"
	AttrQueryFilter     = "goldmane.query.filter"
	AttrResultCount     = "goldmane.result.count"
)

// FlowInstrumentation provides OpenTelemetry instrumentation for flow operations
type FlowInstrumentation struct {
	tracer trace.Tracer
}

// NewFlowInstrumentation creates a new flow instrumentation instance
func NewFlowInstrumentation(componentName string) *FlowInstrumentation {
	return &FlowInstrumentation{
		tracer: otel.Tracer(fmt.Sprintf("goldmane.%s", componentName)),
	}
}

// StartFlowReceiveSpan starts a span for flow reception
func (fi *FlowInstrumentation) StartFlowReceiveSpan(ctx context.Context, clientNode string) (context.Context, trace.Span) {
	return fi.tracer.Start(ctx, SpanFlowReceive,
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String(AttrClientNode, clientNode),
		),
	)
}

// StartFlowAggregateSpan starts a span for flow aggregation
func (fi *FlowInstrumentation) StartFlowAggregateSpan(ctx context.Context, bucketTimestamp int64) (context.Context, trace.Span) {
	return fi.tracer.Start(ctx, SpanFlowAggregate,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.Int64(AttrBucketTimestamp, bucketTimestamp),
		),
	)
}

// StartFlowEmitSpan starts a span for flow emission
func (fi *FlowInstrumentation) StartFlowEmitSpan(ctx context.Context, url string) (context.Context, trace.Span) {
	return fi.tracer.Start(ctx, SpanFlowEmit,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String(AttrEmitURL, url),
		),
	)
}

// StartFlowQuerySpan starts a span for flow queries
func (fi *FlowInstrumentation) StartFlowQuerySpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	return fi.tracer.Start(ctx, SpanFlowQuery,
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("operation", operation),
		),
	)
}

// StartStorageSpan starts a span for storage operations
func (fi *FlowInstrumentation) StartStorageSpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	spanName := fmt.Sprintf("goldmane.storage.%s", operation)
	return fi.tracer.Start(ctx, spanName,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("storage.operation", operation),
		),
	)
}

// AddFlowAttributes adds flow-specific attributes to a span
func (fi *FlowInstrumentation) AddFlowAttributes(span trace.Span, flow *types.Flow) {
	if flow == nil {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.Int64(AttrFlowStartTime, flow.StartTime),
		attribute.Int64(AttrFlowEndTime, flow.EndTime),
		attribute.Int64(AttrFlowPackets, flow.PacketsIn+flow.PacketsOut),
		attribute.Int64(AttrFlowBytes, flow.BytesIn+flow.BytesOut),
	}

	if flow.Key != nil {
		attrs = append(attrs,
			attribute.String(AttrFlowSource, flow.Key.SourceName()),
			attribute.String(AttrFlowDest, flow.Key.DestName()),
			attribute.Int64(AttrFlowProtocol, int64(flow.Key.DestPort())),
		)

		// Create a flow key string for easier identification
		flowKey := fmt.Sprintf("%s->%s:%d/%s",
			flow.Key.SourceName(), flow.Key.DestName(), flow.Key.DestPort(), flow.Key.Proto())
		attrs = append(attrs, attribute.String(AttrFlowKey, flowKey))
	}

	span.SetAttributes(attrs...)
}

// AddProtoFlowAttributes adds proto flow-specific attributes to a span
func (fi *FlowInstrumentation) AddProtoFlowAttributes(span trace.Span, flow *proto.Flow) {
	if flow == nil {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.Int64(AttrFlowStartTime, flow.StartTime),
		attribute.Int64(AttrFlowEndTime, flow.EndTime),
		attribute.Int64(AttrFlowPackets, flow.PacketsIn+flow.PacketsOut),
		attribute.Int64(AttrFlowBytes, flow.BytesIn+flow.BytesOut),
	}

	if flow.Key != nil {
		attrs = append(attrs,
			attribute.String(AttrFlowSource, flow.Key.SourceName),
			attribute.String(AttrFlowDest, flow.Key.DestName),
			attribute.Int64(AttrFlowProtocol, flow.Key.DestPort),
		)

		// Create a flow key string for easier identification
		flowKey := fmt.Sprintf("%s->%s:%d",
			flow.Key.SourceName, flow.Key.DestName, flow.Key.DestPort)
		attrs = append(attrs, attribute.String(AttrFlowKey, flowKey))
	}

	span.SetAttributes(attrs...)
}

// AddQueryAttributes adds query-specific attributes to a span
func (fi *FlowInstrumentation) AddQueryAttributes(span trace.Span, req interface{}) {
	switch r := req.(type) {
	case *proto.FlowListRequest:
		span.SetAttributes(
			attribute.Int64("query.start_time_gte", r.StartTimeGte),
			attribute.Int64("query.start_time_lt", r.StartTimeLt),
			attribute.Int64("query.page", r.Page),
			attribute.Int64("query.page_size", r.PageSize),
			attribute.Int64("query.aggregation_interval", r.AggregationInterval),
		)
	case *proto.FlowStreamRequest:
		span.SetAttributes(
			attribute.Int64("query.start_time_gte", r.StartTimeGte),
			attribute.Int64("query.aggregation_interval", r.AggregationInterval),
		)
	}
}

// AddResultAttributes adds result-specific attributes to a span
func (fi *FlowInstrumentation) AddResultAttributes(span trace.Span, count int) {
	span.SetAttributes(
		attribute.Int64(AttrResultCount, int64(count)),
	)
}

// RecordError records an error in the span
func (fi *FlowInstrumentation) RecordError(span trace.Span, err error) {
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}
}

// AddServiceGraphAttributes adds attributes for service graph generation
func (fi *FlowInstrumentation) AddServiceGraphAttributes(span trace.Span, flow *types.Flow) {
	if flow == nil || flow.Key == nil {
		return
	}

	// Extract service information from flow keys
	// This helps generate service maps in observability tools
	span.SetAttributes(
		attribute.String("service.source", extractServiceName(flow.Key.SourceName())),
		attribute.String("service.destination", extractServiceName(flow.Key.DestName())),
		attribute.String("network.protocol", flow.Key.Proto()),
		attribute.Int64("network.destination.port", flow.Key.DestPort()),
	)
}

// Helper function to extract service name from a flow source/dest
func extractServiceName(endpoint string) string {
	// This is a simplified extraction - in practice, you might want to
	// parse Kubernetes service names, IP addresses, etc.
	if endpoint == "" {
		return "unknown"
	}
	return endpoint
}

// Helper function to convert protocol number to string
func protocolToString(protocol int32) string {
	switch protocol {
	case 6:
		return "tcp"
	case 17:
		return "udp"
	case 1:
		return "icmp"
	default:
		return fmt.Sprintf("protocol-%d", protocol)
	}
}

// CreateServiceFlowTrace creates a trace span representing communication between services
// This transforms network flow data into service-to-service OpenTelemetry traces
func (fi *FlowInstrumentation) CreateServiceFlowTrace(ctx context.Context, flow *types.Flow) (context.Context, trace.Span) {
	if flow == nil || flow.Key == nil {
		return ctx, trace.SpanFromContext(ctx)
	}

	// Create both client and server spans for better service graph visibility
	return fi.CreateFullServiceTrace(ctx, flow)
}

// CreateFullServiceTrace creates a comprehensive service-to-service trace with both client and server perspectives
func (fi *FlowInstrumentation) CreateFullServiceTrace(ctx context.Context, flow *types.Flow) (context.Context, trace.Span) {
	if flow == nil || flow.Key == nil {
		return ctx, trace.SpanFromContext(ctx)
	}

	// Extract service names with namespace for uniqueness
	sourceServiceName := fmt.Sprintf("%s.%s", flow.Key.SourceName(), flow.Key.SourceNamespace())
	destServiceName := fmt.Sprintf("%s.%s", flow.Key.DestName(), flow.Key.DestNamespace())

	// Use Kubernetes service name if available
	if flow.Key.DestServiceName() != "" {
		destServiceName = fmt.Sprintf("%s.%s", flow.Key.DestServiceName(), flow.Key.DestServiceNamespace())
	}

	// Clean up service names
	if sourceServiceName == "." || sourceServiceName == "" {
		sourceServiceName = "unknown-source"
	}
	if destServiceName == "." || destServiceName == "" {
		destServiceName = "unknown-dest"
	}

	// Create parent span from source service perspective (CLIENT)
	sourceTracer := otel.Tracer(sourceServiceName)
	operationName := fmt.Sprintf("call %s", destServiceName)

	parentCtx, parentSpan := sourceTracer.Start(ctx, operationName,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithTimestamp(time.Unix(flow.StartTime, 0)),
	)

	// Add client-side attributes
	clientAttrs := []attribute.KeyValue{
		attribute.String("service.name", sourceServiceName),
		attribute.String("service.namespace", flow.Key.SourceNamespace()),
		attribute.String("peer.service", destServiceName),
		attribute.String("span.kind", "client"),
		attribute.String("component", "network-flow"),
		attribute.String("operation.type", "outbound-call"),

		// Network details
		attribute.String("network.protocol.name", flow.Key.Proto()),
		attribute.Int64("network.peer.port", flow.Key.DestPort()),

		// Flow metrics
		attribute.Int64("flow.packets.out", flow.PacketsOut),
		attribute.Int64("flow.bytes.out", flow.BytesOut),
		attribute.Int64("flow.connections.started", flow.NumConnectionsStarted),
		attribute.Int64("duration.ms", (flow.EndTime-flow.StartTime)*1000),
	}

	parentSpan.SetAttributes(clientAttrs...)

	// Create child span from destination service perspective (SERVER)
	destTracer := otel.Tracer(destServiceName)
	serverOperationName := fmt.Sprintf("receive from %s", sourceServiceName)

	_, serverSpan := destTracer.Start(parentCtx, serverOperationName,
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithTimestamp(time.Unix(flow.StartTime, 0)),
	)

	// Add server-side attributes
	serverAttrs := []attribute.KeyValue{
		attribute.String("service.name", destServiceName),
		attribute.String("service.namespace", flow.Key.DestNamespace()),
		attribute.String("peer.service", sourceServiceName),
		attribute.String("span.kind", "server"),
		attribute.String("component", "network-flow"),
		attribute.String("operation.type", "inbound-receive"),

		// Network details
		attribute.String("network.protocol.name", flow.Key.Proto()),
		attribute.Int64("network.local.port", flow.Key.DestPort()),

		// Flow metrics
		attribute.Int64("flow.packets.in", flow.PacketsIn),
		attribute.Int64("flow.bytes.in", flow.BytesIn),
		attribute.Int64("flow.connections.live", flow.NumConnectionsLive),
		attribute.Int64("duration.ms", (flow.EndTime-flow.StartTime)*1000),
	}

	if flow.Key.DestServiceName() != "" {
		serverAttrs = append(serverAttrs,
			attribute.String("k8s.service.name", flow.Key.DestServiceName()),
			attribute.String("k8s.service.namespace", flow.Key.DestServiceNamespace()),
		)
	}

	serverSpan.SetAttributes(serverAttrs...)

	// End spans in proper order (server first, then client)
	serverSpan.SetStatus(codes.Ok, "Flow received successfully")
	serverSpan.End(trace.WithTimestamp(time.Unix(flow.EndTime, 0)))

	parentSpan.SetStatus(codes.Ok, "Flow sent successfully")
	parentSpan.End(trace.WithTimestamp(time.Unix(flow.EndTime, 0)))

	return parentCtx, parentSpan
}

// generateTraceIDFromFlow creates a deterministic trace ID based on flow characteristics
func generateTraceIDFromFlow(flow *types.Flow) trace.TraceID {
	if flow == nil || flow.Key == nil {
		return trace.TraceID{}
	}

	// Create a hash based on flow key for consistent trace IDs
	h := fmt.Sprintf("%s->%s:%d@%d",
		flow.Key.SourceName(),
		flow.Key.DestName(),
		flow.Key.DestPort(),
		flow.StartTime/300) // Group by 5-minute windows

	// Convert to bytes and pad/truncate to 16 bytes for TraceID
	hashBytes := []byte(h)
	var traceID [16]byte
	copy(traceID[:], hashBytes)
	return trace.TraceID(traceID)
}

// CreateSimpleServiceTrace creates a simpler service-to-service trace optimized for Jaeger service graph
// This creates a single span that represents a call from source service to destination service
func (fi *FlowInstrumentation) CreateSimpleServiceTrace(ctx context.Context, flow *types.Flow) (context.Context, trace.Span) {
	if flow == nil || flow.Key == nil {
		return ctx, trace.SpanFromContext(ctx)
	}

	// Extract clean service names with namespace prefix for uniqueness
	sourceServiceName := fmt.Sprintf("%s.%s", flow.Key.SourceName(), flow.Key.SourceNamespace())
	destServiceName := fmt.Sprintf("%s.%s", flow.Key.DestName(), flow.Key.DestNamespace())

	// Use Kubernetes service name if available for cleaner service graph
	if flow.Key.DestServiceName() != "" {
		destServiceName = fmt.Sprintf("%s.%s", flow.Key.DestServiceName(), flow.Key.DestServiceNamespace())
	}

	// Clean up service names to be more readable
	if sourceServiceName == "." {
		sourceServiceName = "unknown-source"
	}
	if destServiceName == "." {
		destServiceName = "unknown-dest"
	}

	// Create operation name that shows the call
	operationName := fmt.Sprintf("call %s", destServiceName)

	// Create tracer with source service name - this groups spans by service
	serviceTracer := otel.Tracer(sourceServiceName)

	spanCtx, span := serviceTracer.Start(ctx, operationName,
		trace.WithSpanKind(trace.SpanKindClient), // Client span indicates outgoing call
		trace.WithTimestamp(time.Unix(flow.StartTime, 0)),
	)

	// Essential attributes for Jaeger service graph
	attrs := []attribute.KeyValue{
		// Critical: These attributes define the service nodes and edges
		attribute.String("service.name", sourceServiceName),
		attribute.String("service.namespace", flow.Key.SourceNamespace()),
		attribute.String("peer.service", destServiceName), // Key for service graph edges

		// Network context
		attribute.String("network.protocol.name", flow.Key.Proto()),
		attribute.Int64("network.peer.port", flow.Key.DestPort()),

		// Flow metrics for observability
		attribute.Int64("flow.packets.total", flow.PacketsIn+flow.PacketsOut),
		attribute.Int64("flow.bytes.total", flow.BytesIn+flow.BytesOut),
		attribute.Int64("flow.connections.active", flow.NumConnectionsLive),

		// Add HTTP-like attributes to make Jaeger happier
		attribute.String("span.kind", "client"),
		attribute.String("component", "network-flow"),

		// Add operation type and duration for better tracing
		attribute.String("operation.type", "network-communication"),
		attribute.Int64("duration.ms", (flow.EndTime-flow.StartTime)*1000), // Convert to milliseconds
	}

	// Add destination details for richer context
	if flow.Key.DestServiceName() != "" {
		attrs = append(attrs,
			attribute.String("k8s.destination.service.name", flow.Key.DestServiceName()),
			attribute.String("k8s.destination.service.namespace", flow.Key.DestServiceNamespace()),
		)
		if flow.Key.DestServicePortName() != "" {
			attrs = append(attrs, attribute.String("k8s.destination.port.name", flow.Key.DestServicePortName()))
		}
	}

	// Add protocol-specific attributes
	switch flow.Key.Proto() {
	case "tcp":
		attrs = append(attrs,
			attribute.String("network.transport", "tcp"),
			attribute.Bool("network.connection.reliable", true),
		)
	case "udp":
		attrs = append(attrs,
			attribute.String("network.transport", "udp"),
			attribute.Bool("network.connection.reliable", false),
		)
	}

	span.SetAttributes(attrs...)

	// End the span with flow end time and mark as successful
	span.SetStatus(codes.Ok, "Flow completed successfully")
	span.End(trace.WithTimestamp(time.Unix(flow.EndTime, 0)))

	return spanCtx, span
}

// CreateServiceFlowTraceFromProto creates a trace span from a proto.Flow
func (fi *FlowInstrumentation) CreateServiceFlowTraceFromProto(ctx context.Context, flow *proto.Flow) (context.Context, trace.Span) {
	if flow == nil || flow.Key == nil {
		return ctx, trace.SpanFromContext(ctx)
	}

	// Create a span representing the communication from source to destination service
	serviceName := fmt.Sprintf("%s.%s", flow.Key.SourceName, flow.Key.SourceNamespace)
	destinationService := fmt.Sprintf("%s.%s", flow.Key.DestName, flow.Key.DestNamespace)

	spanName := fmt.Sprintf("%s → %s", serviceName, destinationService)

	spanCtx, span := fi.tracer.Start(ctx, spanName,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithTimestamp(time.Unix(flow.StartTime, 0)),
	)

	// Add service graph attributes for observability tools
	attrs := []attribute.KeyValue{
		// Service identification
		attribute.String("service.name", serviceName),
		attribute.String("service.namespace", flow.Key.SourceNamespace),
		attribute.String("service.destination.name", destinationService),
		attribute.String("service.destination.namespace", flow.Key.DestNamespace),

		// Network details
		attribute.String("network.protocol", flow.Key.Proto),
		attribute.Int64("network.destination.port", flow.Key.DestPort),

		// Flow metrics
		attribute.Int64("flow.packets.in", flow.PacketsIn),
		attribute.Int64("flow.packets.out", flow.PacketsOut),
		attribute.Int64("flow.bytes.in", flow.BytesIn),
		attribute.Int64("flow.bytes.out", flow.BytesOut),
		attribute.Int64("flow.connections.started", flow.NumConnectionsStarted),
		attribute.Int64("flow.connections.completed", flow.NumConnectionsCompleted),
		attribute.Int64("flow.connections.live", flow.NumConnectionsLive),

		// Timing
		attribute.Int64("flow.start_time", flow.StartTime),
		attribute.Int64("flow.end_time", flow.EndTime),
		attribute.Int64("flow.duration", flow.EndTime-flow.StartTime),
	}

	// Add destination service information if available
	if flow.Key.DestServiceName != "" {
		attrs = append(attrs,
			attribute.String("service.destination.k8s.name", flow.Key.DestServiceName),
			attribute.String("service.destination.k8s.namespace", flow.Key.DestServiceNamespace),
		)
		if flow.Key.DestServicePortName != "" {
			attrs = append(attrs, attribute.String("service.destination.k8s.port.name", flow.Key.DestServicePortName))
		}
	}

	// Add policy enforcement information
	if flow.Key != nil && flow.Key.Policies != nil && (len(flow.Key.Policies.EnforcedPolicies) > 0 || len(flow.Key.Policies.PendingPolicies) > 0) {
		attrs = append(attrs,
			attribute.String("flow.policies.enforced", "true"),
			attribute.Int64("flow.policies.enforced.count", int64(len(flow.Key.Policies.EnforcedPolicies))),
			attribute.Int64("flow.policies.pending.count", int64(len(flow.Key.Policies.PendingPolicies))),
		)
	}

	span.SetAttributes(attrs...)

	// Set the span end time
	span.End(trace.WithTimestamp(time.Unix(flow.EndTime, 0)))

	return spanCtx, span
}
