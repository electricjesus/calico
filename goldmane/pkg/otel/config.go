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
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	// ServiceName is the name of the Goldmane service for OpenTelemetry
	ServiceName = "goldmane"

	// ServiceVersion will be set at build time
	ServiceVersion = "dev"
)

// Config holds the OpenTelemetry configuration
type Config struct {
	// Enabled determines if OpenTelemetry is enabled
	Enabled bool

	// CollectorEndpoint is the endpoint of the OpenTelemetry collector
	CollectorEndpoint string

	// CollectorEndpointUseInsecure determines if the connection to the collector is insecure
	// (adds `insecure` option to the gRPC client)
	CollectorEndpointUseInsecure bool

	// ServiceName is the name of the service
	ServiceName string

	// ServiceVersion is the version of the service
	ServiceVersion string

	// ServiceNamespace is the namespace of the service (e.g., "calico-system")
	ServiceNamespace string

	// NodeName is the name of the node where this service is running
	NodeName string

	// ClusterName is the name of the Kubernetes cluster
	ClusterName string

	// SamplingRate is the rate at which traces are sampled (0.0 to 1.0)
	SamplingRate float64
}

// Provider manages OpenTelemetry providers and their lifecycle
type Provider struct {
	tracerProvider *sdktrace.TracerProvider
	config         Config
}

// NewProvider creates a new OpenTelemetry provider
func NewProvider(ctx context.Context, config Config) (*Provider, error) {
	if !config.Enabled {
		return &Provider{config: config}, nil
	}

	// Create resource with service information
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(config.ServiceName),
			semconv.ServiceVersion(config.ServiceVersion),
			semconv.ServiceNamespace(config.ServiceNamespace),
			semconv.K8SNodeName(config.NodeName),
			semconv.K8SClusterName(config.ClusterName),
			attribute.String("service.component", "flow-aggregator"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create OTLP trace exporter
	newClientOptions := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(config.CollectorEndpoint),
	}

	if config.CollectorEndpointUseInsecure {
		newClientOptions = append(newClientOptions, otlptracegrpc.WithInsecure())
	}

	traceExporter, err := otlptrace.New(ctx, otlptracegrpc.NewClient(newClientOptions...))
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Create tracer provider
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(config.SamplingRate)),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tracerProvider)

	// Set global propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &Provider{
		tracerProvider: tracerProvider,
		config:         config,
	}, nil
}

// Shutdown gracefully shuts down the OpenTelemetry provider
func (p *Provider) Shutdown(ctx context.Context) error {
	if !p.config.Enabled || p.tracerProvider == nil {
		return nil
	}

	// Create a context with timeout for shutdown
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return p.tracerProvider.Shutdown(shutdownCtx)
}

// IsEnabled returns true if OpenTelemetry is enabled
func (p *Provider) IsEnabled() bool {
	return p.config.Enabled
}

// GetTracer returns a tracer for the given name
func (p *Provider) GetTracer(name string) trace.Tracer {
	if !p.config.Enabled {
		return otel.GetTracerProvider().Tracer(name)
	}
	return otel.Tracer(name)
}

// Helper functions

func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvFloatWithDefault(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if parsed, err := fmt.Sscanf(value, "%f", &defaultValue); err == nil && parsed == 1 {
			return defaultValue
		}
	}
	return defaultValue
}
