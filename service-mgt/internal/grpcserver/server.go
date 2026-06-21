package grpcserver

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go-stock-prediction/service-mgt/internal/registry"
	"go-stock-prediction/service-mgt/internal/store"
	registrypb "go-stock-prediction/service-mgt/proto/registry"
)

// Server adapts the registry core to the gRPC Registry service.
type Server struct {
	registrypb.UnimplementedRegistryServer
	core *registry.Core
}

func New(core *registry.Core) *Server { return &Server{core: core} }

func toPB(i *store.Instance) *registrypb.Instance {
	return &registrypb.Instance{
		ServiceName: i.ServiceName, InstanceId: i.InstanceID, Address: i.Address,
		Port: i.Port, Metadata: i.Metadata, Status: i.Status,
	}
}

func (s *Server) Register(_ context.Context, in *registrypb.RegisterRequest) (*registrypb.RegisterResponse, error) {
	if in.ServiceName == "" || in.Address == "" || in.Port == 0 {
		return nil, status.Error(codes.InvalidArgument, "service_name, address, port required")
	}
	id, ttl, err := s.core.Register(&store.Instance{
		ServiceName: in.ServiceName, InstanceID: in.InstanceId, Address: in.Address,
		Port: in.Port, Metadata: in.Metadata, TTLSeconds: in.TtlSeconds,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "register: %v", err)
	}
	return &registrypb.RegisterResponse{InstanceId: id, LeaseTtlSeconds: ttl}, nil
}

func (s *Server) Heartbeat(_ context.Context, in *registrypb.HeartbeatRequest) (*registrypb.HeartbeatResponse, error) {
	if !s.core.Heartbeat(in.InstanceId) {
		return nil, status.Error(codes.NotFound, "unknown instance; re-register")
	}
	return &registrypb.HeartbeatResponse{Ok: true}, nil
}

func (s *Server) Deregister(_ context.Context, in *registrypb.DeregisterRequest) (*registrypb.Empty, error) {
	if err := s.core.Deregister(in.InstanceId); err != nil {
		return nil, status.Errorf(codes.Internal, "deregister: %v", err)
	}
	return &registrypb.Empty{}, nil
}

func (s *Server) Discover(_ context.Context, in *registrypb.DiscoverRequest) (*registrypb.DiscoverResponse, error) {
	insts := s.core.Discover(in.ServiceName)
	out := make([]*registrypb.Instance, 0, len(insts))
	for _, i := range insts {
		out = append(out, toPB(i))
	}
	return &registrypb.DiscoverResponse{Instances: out}, nil
}

func (s *Server) ListServices(_ context.Context, _ *registrypb.Empty) (*registrypb.ListServicesResponse, error) {
	insts := s.core.List()
	out := make([]*registrypb.Instance, 0, len(insts))
	for _, i := range insts {
		out = append(out, toPB(i))
	}
	return &registrypb.ListServicesResponse{Instances: out}, nil
}
