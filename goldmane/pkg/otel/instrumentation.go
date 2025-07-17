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
