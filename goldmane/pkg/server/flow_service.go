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

package server

import (
	"context"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"github.com/projectcalico/calico/goldmane/pkg/goldmane"
	"github.com/projectcalico/calico/goldmane/pkg/otel"
	"github.com/projectcalico/calico/goldmane/proto"
)

func NewFlowsServer(aggr *goldmane.Goldmane) *FlowsServer {
	return &FlowsServer{
		gm:        aggr,
		otelInstr: otel.NewFlowInstrumentation("flows"),
	}
}

type FlowsServer struct {
	proto.UnimplementedFlowsServer

	gm        *goldmane.Goldmane
	otelInstr *otel.FlowInstrumentation
}

func (s *FlowsServer) RegisterWith(srv *grpc.Server) {
	// Register the server with the gRPC server.
	proto.RegisterFlowsServer(srv, s)
	logrus.Info("Registered FlowAPI Server")
}

func (s *FlowsServer) List(ctx context.Context, req *proto.FlowListRequest) (*proto.FlowListResult, error) {
	ctx, span := s.otelInstr.StartFlowQuerySpan(ctx, "list")
	defer span.End()

	s.otelInstr.AddQueryAttributes(span, req)

	result, err := s.gm.List(req)
	if err != nil {
		s.otelInstr.RecordError(span, err)
		return nil, err
	}

	if result != nil {
		s.otelInstr.AddResultAttributes(span, len(result.Flows))
	}

	return result, nil
}

func (s *FlowsServer) Stream(req *proto.FlowStreamRequest, server proto.Flows_StreamServer) error {
	ctx, span := s.otelInstr.StartFlowQuerySpan(server.Context(), "stream")
	defer span.End()

	s.otelInstr.AddQueryAttributes(span, req)

	// Get a new Stream from the aggregator.
	stream, err := s.gm.Stream(req)
	if err != nil {
		s.otelInstr.RecordError(span, err)
		return err
	}
	defer stream.Close()

	// Share memory for each flow result.
	result := &proto.FlowResult{Flow: &proto.Flow{}}

	flowCount := 0
	defer func() {
		s.otelInstr.AddResultAttributes(span, flowCount)
	}()

	for {
		select {
		case flow := <-stream.Flows():
			if flow.BuildInto(req.Filter, result) {
				if err := server.Send(result); err != nil {
					s.otelInstr.RecordError(span, err)
					return err
				}
				flowCount++
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (s *FlowsServer) FilterHints(ctx context.Context, req *proto.FilterHintsRequest) (*proto.FilterHintsResult, error) {
	ctx, span := s.otelInstr.StartFlowQuerySpan(ctx, "filter_hints")
	defer span.End()

	result, err := s.gm.Hints(req)
	if err != nil {
		s.otelInstr.RecordError(span, err)
		return nil, err
	}

	return result, nil
}
