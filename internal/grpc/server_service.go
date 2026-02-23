package grpc

import (
	"context"
	"time"

	"github.com/josephgoksu/TaskWing/internal/agents/impl"
	"github.com/josephgoksu/TaskWing/internal/app"
	"github.com/josephgoksu/TaskWing/internal/config"

	pb "github.com/josephgoksu/TaskWing/gen/go/taskwing/v1"
)

// ServerServiceImpl adapts server info and activity to the gRPC ServerService.
type ServerServiceImpl struct {
	pb.UnimplementedServerServiceServer
	appCtx  *app.Context
	version string
}

// NewServerService creates a ServerService adapter.
func NewServerService(appCtx *app.Context, version string) *ServerServiceImpl {
	return &ServerServiceImpl{appCtx: appCtx, version: version}
}

func (s *ServerServiceImpl) GetInfo(_ context.Context, _ *pb.GetInfoRequest) (*pb.GetInfoResponse, error) {
	projectPath, _ := config.GetProjectRoot()
	memoryPath := s.appCtx.BasePath

	return &pb.GetInfoResponse{
		ProjectPath: projectPath,
		Version:     s.version,
		MemoryPath:  memoryPath,
	}, nil
}

func (s *ServerServiceImpl) GetActivity(_ context.Context, req *pb.GetActivityRequest) (*pb.GetActivityResponse, error) {
	limit := int(req.Limit)
	if limit == 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	activityLog := impl.NewActivityLog(s.appCtx.BasePath)
	entries := activityLog.GetRecent(limit)

	pbEntries := make([]*pb.ActivityEntry, len(entries))
	for i := range entries {
		pbEntries[i] = activityEntryToProto(&entries[i])
	}

	return &pb.GetActivityResponse{Entries: pbEntries}, nil
}

func (s *ServerServiceImpl) StreamActivity(req *pb.StreamActivityRequest, stream pb.ServerService_StreamActivityServer) error {
	initialLimit := int(req.InitialLimit)
	if initialLimit == 0 {
		initialLimit = 20
	}

	activityLog := impl.NewActivityLog(s.appCtx.BasePath)
	entries := activityLog.GetRecent(initialLimit)

	pbEntries := make([]*pb.ActivityEntry, len(entries))
	seen := make(map[int64]struct{}, len(entries))
	for i := range entries {
		seen[entries[i].ID] = struct{}{}
		pbEntries[i] = activityEntryToProto(&entries[i])
	}

	// Send initial batch.
	if err := stream.Send(&pb.StreamActivityResponse{
		Event: &pb.StreamActivityResponse_Initial{Initial: &pb.ActivityBatch{
			Entries: pbEntries,
		}},
	}); err != nil {
		return err
	}

	// TODO(taskwing#activity-streaming): replace polling with pub/sub from app layer.
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case <-ticker.C:
			latest := activityLog.GetRecent(50)
			for i := len(latest) - 1; i >= 0; i-- {
				entry := latest[i]
				if _, ok := seen[entry.ID]; ok {
					continue
				}
				seen[entry.ID] = struct{}{}

				if err := stream.Send(&pb.StreamActivityResponse{
					Event: &pb.StreamActivityResponse_NewEntry{NewEntry: activityEntryToProto(&entry)},
				}); err != nil {
					return err
				}
			}
		}
	}
}
