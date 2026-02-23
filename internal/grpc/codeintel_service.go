package grpc

import (
	"bytes"
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/josephgoksu/TaskWing/internal/app"
	"github.com/josephgoksu/TaskWing/internal/codeintel"

	pb "github.com/josephgoksu/TaskWing/gen/go/taskwing/v1"
)

// CodeIntelService adapts internal/app.CodeIntelApp and ExplainApp to the gRPC CodeIntelService.
type CodeIntelService struct {
	pb.UnimplementedCodeIntelServiceServer
	appCtx *app.Context
}

// NewCodeIntelService creates a CodeIntelService adapter.
func NewCodeIntelService(appCtx *app.Context) *CodeIntelService {
	return &CodeIntelService{appCtx: appCtx}
}

func (s *CodeIntelService) FindSymbol(ctx context.Context, req *pb.FindSymbolRequest) (*pb.FindSymbolResponse, error) {
	ciApp := app.NewCodeIntelApp(s.appCtx)

	result, err := ciApp.FindSymbol(ctx, app.FindSymbolOptions{
		Name:     req.Name,
		ID:       req.Id,
		FilePath: req.FilePath,
		Language: req.Language,
	})
	if err != nil {
		return nil, mapError(err)
	}

	symbols := make([]*pb.Symbol, len(result.Symbols))
	for i := range result.Symbols {
		symbols[i] = symbolToProto(&result.Symbols[i])
	}

	return &pb.FindSymbolResponse{
		Success: result.Success,
		Symbols: symbols,
		Count:   int32(result.Count),
		Message: result.Message,
	}, nil
}

func (s *CodeIntelService) SearchCode(ctx context.Context, req *pb.SearchCodeRequest) (*pb.SearchCodeResponse, error) {
	if req.Query == "" {
		return nil, status.Error(codes.InvalidArgument, "query is required")
	}

	ciApp := app.NewCodeIntelApp(s.appCtx)

	result, err := ciApp.SearchCode(ctx, app.SearchCodeOptions{
		Query:    req.Query,
		Limit:    int(req.Limit),
		Kind:     symbolKindFromProto(req.Kind),
		FilePath: req.FilePath,
	})
	if err != nil {
		return nil, mapError(err)
	}

	results := make([]*pb.SymbolSearchResult, len(result.Results))
	for i := range result.Results {
		results[i] = symbolSearchResultToProto(&result.Results[i])
	}

	return &pb.SearchCodeResponse{
		Success: result.Success,
		Results: results,
		Count:   int32(result.Count),
		Query:   result.Query,
		Message: result.Message,
	}, nil
}

func (s *CodeIntelService) GetCallers(ctx context.Context, req *pb.GetCallersRequest) (*pb.GetCallersResponse, error) {
	ciApp := app.NewCodeIntelApp(s.appCtx)

	direction := req.Direction
	if direction == "" {
		direction = "both"
	}

	result, err := ciApp.GetCallers(ctx, app.GetCallersOptions{
		SymbolID:   req.SymbolId,
		SymbolName: req.SymbolName,
		Direction:  direction,
	})
	if err != nil {
		return nil, mapError(err)
	}

	callers := make([]*pb.Symbol, len(result.Callers))
	for i := range result.Callers {
		callers[i] = symbolToProto(&result.Callers[i])
	}

	callees := make([]*pb.Symbol, len(result.Callees))
	for i := range result.Callees {
		callees[i] = symbolToProto(&result.Callees[i])
	}

	var sym *pb.Symbol
	if result.Symbol != nil {
		sym = symbolToProto(result.Symbol)
	}

	return &pb.GetCallersResponse{
		Success: result.Success,
		Symbol:  sym,
		Callers: callers,
		Callees: callees,
		Count:   int32(result.Count),
		Message: result.Message,
	}, nil
}

func (s *CodeIntelService) AnalyzeImpact(ctx context.Context, req *pb.AnalyzeImpactRequest) (*pb.AnalyzeImpactResponse, error) {
	ciApp := app.NewCodeIntelApp(s.appCtx)

	maxDepth := int(req.MaxDepth)
	if maxDepth == 0 {
		maxDepth = 5
	}

	result, err := ciApp.AnalyzeImpact(ctx, app.AnalyzeImpactOptions{
		SymbolID:   req.SymbolId,
		SymbolName: req.SymbolName,
		MaxDepth:   maxDepth,
	})
	if err != nil {
		return nil, mapError(err)
	}

	affected := make([]*pb.ImpactNode, len(result.Affected))
	for i := range result.Affected {
		affected[i] = impactNodeToProto(&result.Affected[i])
	}

	var source *pb.Symbol
	if result.Source != nil {
		source = symbolToProto(result.Source)
	}

	byDepth := make(map[int32]*pb.DepthGroup, len(result.ByDepth))
	for depth, syms := range result.ByDepth {
		group := &pb.DepthGroup{}
		for i := range syms {
			group.Symbols = append(group.Symbols, symbolToProto(&syms[i]))
		}
		byDepth[int32(depth)] = group
	}

	return &pb.AnalyzeImpactResponse{
		Success:       result.Success,
		Source:        source,
		Affected:      affected,
		AffectedCount: int32(result.AffectedCount),
		AffectedFiles: int32(result.AffectedFiles),
		MaxDepth:      int32(result.MaxDepth),
		ByDepth:       byDepth,
		Message:       result.Message,
	}, nil
}

func (s *CodeIntelService) Explain(ctx context.Context, req *pb.ExplainRequest) (*pb.ExplainResponse, error) {
	explainApp := app.NewExplainApp(s.appCtx)

	depth := int(req.Depth)
	if depth == 0 {
		depth = 2
	}

	result, err := explainApp.Explain(ctx, app.ExplainRequest{
		Query:       req.Query,
		SymbolID:    req.SymbolId,
		Depth:       depth,
		IncludeCode: req.IncludeCode,
	})
	if err != nil {
		return nil, mapError(err)
	}

	return explainResultToProto(result), nil
}

func (s *CodeIntelService) ExplainStream(req *pb.ExplainRequest, stream pb.CodeIntelService_ExplainStreamServer) error {
	explainApp := app.NewExplainApp(s.appCtx)

	depth := int(req.Depth)
	if depth == 0 {
		depth = 2
	}

	// Capture streaming explanation in a buffer.
	var buf bytes.Buffer
	result, err := explainApp.Explain(stream.Context(), app.ExplainRequest{
		Query:        req.Query,
		SymbolID:     req.SymbolId,
		Depth:        depth,
		IncludeCode:  req.IncludeCode,
		StreamWriter: &buf,
	})
	if err != nil {
		return mapError(err)
	}

	// Send structure data first.
	proto := explainResultToProto(result)
	if err := stream.Send(&pb.ExplainStreamResponse{
		Event: &pb.ExplainStreamResponse_Structure{Structure: &pb.ExplainStructure{
			Symbol:      proto.Symbol,
			Callers:     proto.Callers,
			Callees:     proto.Callees,
			ImpactStats: proto.ImpactStats,
			Decisions:   proto.Decisions,
			Patterns:    proto.Patterns,
			SourceCode:  proto.SourceCode,
		}},
	}); err != nil {
		return err
	}

	// Send explanation as a single chunk.
	// TODO(taskwing#explain-streaming): stream LLM tokens directly from explain callbacks.
	if result.Explanation != "" {
		if err := stream.Send(&pb.ExplainStreamResponse{
			Event: &pb.ExplainStreamResponse_ExplanationChunk{ExplanationChunk: &pb.ExplainChunk{
				Token: result.Explanation,
			}},
		}); err != nil {
			return err
		}
	}

	// Send completion.
	return stream.Send(&pb.ExplainStreamResponse{
		Event: &pb.ExplainStreamResponse_Complete{Complete: &pb.ExplainComplete{
			FullExplanation: result.Explanation,
		}},
	})
}

func (s *CodeIntelService) GetStats(ctx context.Context, _ *pb.GetCodeStatsRequest) (*pb.GetCodeStatsResponse, error) {
	ciApp := app.NewCodeIntelApp(s.appCtx)

	result, err := ciApp.GetStats(ctx)
	if err != nil {
		return nil, mapError(err)
	}

	return &pb.GetCodeStatsResponse{
		Success:      result.Success,
		TotalSymbols: int32(result.SymbolsFound),
		TotalFiles:   int32(result.FilesIndexed),
		Message:      result.Message,
	}, nil
}

// symbolKindFromProto converts a proto SymbolKind to the domain SymbolKind.
func symbolKindFromProto(k pb.SymbolKind) codeintel.SymbolKind {
	switch k {
	case pb.SymbolKind_SYMBOL_KIND_FUNCTION:
		return codeintel.SymbolFunction
	case pb.SymbolKind_SYMBOL_KIND_METHOD:
		return codeintel.SymbolMethod
	case pb.SymbolKind_SYMBOL_KIND_STRUCT:
		return codeintel.SymbolStruct
	case pb.SymbolKind_SYMBOL_KIND_INTERFACE:
		return codeintel.SymbolInterface
	case pb.SymbolKind_SYMBOL_KIND_TYPE:
		return codeintel.SymbolType
	case pb.SymbolKind_SYMBOL_KIND_VARIABLE:
		return codeintel.SymbolVariable
	case pb.SymbolKind_SYMBOL_KIND_CONSTANT:
		return codeintel.SymbolConstant
	case pb.SymbolKind_SYMBOL_KIND_FIELD:
		return codeintel.SymbolField
	case pb.SymbolKind_SYMBOL_KIND_PACKAGE:
		return codeintel.SymbolPackage
	default:
		return ""
	}
}
