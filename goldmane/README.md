## Goldmane

Goldmane is a flow aggregation service. It provides a central, aggregated view of network flows in a Kubernetes cluster.

Some key packages:

- **proto/** defines the Flow structure and gRPC services provided by Goldmane.
- **pkg/aggregator/** collects flow information from across the cluster and aggregates those flows across all nodes, building a cluster-wide view of network activity.
- **pkg/client/** contains Golang wrappers for the Goldmane gRPC client code.
- **pkg/server/** contains Golang wrappers for the Goldmane gRPC server code.
- **pkg/emitter/** periodically emits time-aggregated flow information to a configured endpoint.
- **pkg/types/** contains types used by Goldmane.
- **pkg/otel/** provides OpenTelemetry integration for enhanced observability.

### Connecting to Goldmane

The following provides an example of how to interact with Goldmane APIs on a Calico cluster from your local machine.

To connect to the Goldmane gRPC API in a production Calico cluster, you will need a few things:

- **Client certificate and key** - Goldmane mandates mTLS with clients.
- **CA certificate** to verify server TLS.

You can fetch these from a typical Calico cluster, for example the following commands collect calico/node client credentials for use:

```
kubectl get secret -n calico-system node-certs --template='{{index .data "tls.key"}}' | base64 -d > tls.key
kubectl get secret -n calico-system node-certs --template='{{index .data "tls.crt"}}' | base64 -d > tls.crt
kubectl get secret -n calico-system goldmane-key-pair --template='{{index .data "tls.crt"}}' | base64 -d > ca.crt
```

Goldmane itself is accessible via port-forwarding:

```
kubectl port-forward -n calico-system svc/goldmane 7443:7443
```

You can now write code using the API defined in [proto/api.proto](proto/api.proto), and access Goldmane APIs directly at `localhost:7443`. It may be useful to use the existing client code at [pkg/client/flowservice.go](pkg/client/flowservice.go) as a starting point.

## OpenTelemetry Integration

Goldmane now includes OpenTelemetry support for enhanced observability, including distributed tracing, service graph generation, and improved metrics. This provides better visibility into flow processing performance and service relationships.

### Key Features

- **Distributed Tracing**: Track flows through the entire processing pipeline
- **Service Graph**: Visualize service-to-service communication patterns
- **Enhanced Metrics**: Additional telemetry beyond Prometheus metrics
- **Error Correlation**: Better debugging with trace correlation

### Configuration

Enable OpenTelemetry by setting environment variables:

```bash
export OTEL_ENABLED=true
export OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317
export OTEL_SERVICE_NAME=goldmane
export OTEL_SERVICE_NAMESPACE=calico-system
export NODE_NAME=node-1
export CLUSTER_NAME=my-cluster
export OTEL_SAMPLING_RATE=0.1
```

For detailed configuration and deployment instructions, see [docs/opentelemetry.md](docs/opentelemetry.md).
