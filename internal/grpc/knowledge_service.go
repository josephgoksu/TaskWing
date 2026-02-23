package grpc

import (
	"bytes"
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/josephgoksu/TaskWing/internal/app"

	pb "github.com/josephgoksu/TaskWing/gen/go/taskwing/v1"
)

// KnowledgeService adapts internal/app.AskApp and MemoryApp to the gRPC KnowledgeService.
type KnowledgeService struct {
	pb.UnimplementedKnowledgeServiceServer
	appCtx *app.Context
}

// NewKnowledgeService creates a KnowledgeService adapter.
func NewKnowledgeService(appCtx *app.Context) *KnowledgeService {
	return &KnowledgeService{appCtx: appCtx}
}

func (s *KnowledgeService) Search(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
	if req.Query == "" {
		return nil, status.Error(codes.InvalidArgument, "query is required")
	}

	askApp := app.NewAskApp(s.appCtx)

	limit := int(req.Limit)
	if limit == 0 {
		limit = 5
	}
	symbolLimit := int(req.SymbolLimit)
	if symbolLimit == 0 {
		symbolLimit = 5
	}

	result, err := askApp.Query(ctx, req.Query, app.AskOptions{
		Limit:          limit,
		SymbolLimit:    symbolLimit,
		GenerateAnswer: req.GenerateAnswer,
		IncludeSymbols: req.IncludeSymbols,
		NoRewrite:      req.NoRewrite,
		DisableVector:  req.DisableVector,
		DisableRerank:  req.DisableRerank,
		Workspace:      req.Workspace,
		IncludeRoot:    req.IncludeRoot,
	})
	if err != nil {
		return nil, mapError(err)
	}

	return askResultToProto(result), nil
}

func (s *KnowledgeService) SearchStream(req *pb.SearchRequest, stream pb.KnowledgeService_SearchStreamServer) error {
	if req.Query == "" {
		return status.Error(codes.InvalidArgument, "query is required")
	}

	askApp := app.NewAskApp(s.appCtx)

	limit := int(req.Limit)
	if limit == 0 {
		limit = 5
	}
	symbolLimit := int(req.SymbolLimit)
	if symbolLimit == 0 {
		symbolLimit = 5
	}

	// Use a buffer to capture the streaming answer, then send chunks.
	var answerBuf bytes.Buffer
	var streamWriter *bytes.Buffer
	if req.GenerateAnswer {
		streamWriter = &answerBuf
	}

	result, err := askApp.Query(stream.Context(), req.Query, app.AskOptions{
		Limit:          limit,
		SymbolLimit:    symbolLimit,
		GenerateAnswer: req.GenerateAnswer,
		IncludeSymbols: req.IncludeSymbols,
		NoRewrite:      req.NoRewrite,
		DisableVector:  req.DisableVector,
		DisableRerank:  req.DisableRerank,
		Workspace:      req.Workspace,
		IncludeRoot:    req.IncludeRoot,
		StreamWriter:   streamWriter,
	})
	if err != nil {
		return mapError(err)
	}

	// Send search results batch first.
	resp := askResultToProto(result)
	nodes := make([]*pb.KnowledgeNode, len(resp.Results))
	copy(nodes, resp.Results)
	symbols := make([]*pb.SymbolResult, len(resp.Symbols))
	copy(symbols, resp.Symbols)

	if err := stream.Send(&pb.SearchStreamResponse{
		Event: &pb.SearchStreamResponse_Results{Results: &pb.SearchResultBatch{
			Nodes:          nodes,
			Symbols:        symbols,
			Query:          resp.Query,
			RewrittenQuery: resp.RewrittenQuery,
			Pipeline:       resp.Pipeline,
		}},
	}); err != nil {
		return err
	}

	// Send the answer as a single chunk.
	// TODO(taskwing#search-streaming): integrate token callbacks for true incremental LLM streaming.
	if result.Answer != "" {
		if err := stream.Send(&pb.SearchStreamResponse{
			Event: &pb.SearchStreamResponse_AnswerChunk{AnswerChunk: &pb.AnswerChunk{
				Token: result.Answer,
			}},
		}); err != nil {
			return err
		}
	}

	// Send completion message.
	return stream.Send(&pb.SearchStreamResponse{
		Event: &pb.SearchStreamResponse_Complete{Complete: &pb.SearchComplete{
			Total:        int32(result.Total),
			TotalSymbols: int32(result.TotalSymbols),
			Warning:      result.Warning,
		}},
	})
}

func (s *KnowledgeService) GetSummary(ctx context.Context, _ *pb.GetSummaryRequest) (*pb.GetSummaryResponse, error) {
	askApp := app.NewAskApp(s.appCtx)
	summary, err := askApp.Summary(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	return &pb.GetSummaryResponse{Summary: projectSummaryToProto(summary)}, nil
}

func (s *KnowledgeService) GetStats(ctx context.Context, _ *pb.GetStatsRequest) (*pb.GetStatsResponse, error) {
	nodes, err := s.appCtx.Repo.ListNodes("")
	if err != nil {
		return nil, mapError(err)
	}
	pbCounts := make(map[string]int32)
	for _, n := range nodes {
		pbCounts[n.Type]++
	}
	return &pb.GetStatsResponse{Counts: pbCounts, Total: int32(len(nodes))}, nil
}

func (s *KnowledgeService) Add(ctx context.Context, req *pb.AddKnowledgeRequest) (*pb.AddKnowledgeResponse, error) {
	if req.Content == "" {
		return nil, status.Error(codes.InvalidArgument, "content is required")
	}

	memApp := app.NewMemoryApp(s.appCtx)
	result, err := memApp.Add(ctx, req.Content, app.AddOptions{
		Type:   req.Type,
		SkipAI: req.SkipAi,
	})
	if err != nil {
		return nil, mapError(err)
	}

	return &pb.AddKnowledgeResponse{
		Id:           result.ID,
		Type:         result.Type,
		Summary:      result.Summary,
		HasEmbedding: result.HasEmbedding,
	}, nil
}

func (s *KnowledgeService) Get(ctx context.Context, req *pb.GetKnowledgeRequest) (*pb.GetKnowledgeResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	memApp := app.NewMemoryApp(s.appCtx)
	node, err := memApp.Get(ctx, req.Id)
	if err != nil {
		return nil, mapError(err)
	}

	return &pb.GetKnowledgeResponse{Node: knowledgeNodeToProto(node)}, nil
}

func (s *KnowledgeService) List(ctx context.Context, req *pb.ListKnowledgeRequest) (*pb.ListKnowledgeResponse, error) {
	memApp := app.NewMemoryApp(s.appCtx)
	nodes, err := memApp.List(ctx, req.Type)
	if err != nil {
		return nil, mapError(err)
	}

	pbNodes := make([]*pb.KnowledgeNode, len(nodes))
	for i := range nodes {
		pbNodes[i] = knowledgeNodeToProto(&nodes[i])
	}
	return &pb.ListKnowledgeResponse{Nodes: pbNodes}, nil
}

func (s *KnowledgeService) Delete(ctx context.Context, req *pb.DeleteKnowledgeRequest) (*pb.DeleteKnowledgeResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	memApp := app.NewMemoryApp(s.appCtx)
	if err := memApp.Delete(ctx, req.Id); err != nil {
		return nil, mapError(err)
	}

	return &pb.DeleteKnowledgeResponse{}, nil
}
