package grpc

import (
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/josephgoksu/TaskWing/internal/agents/impl"
	"github.com/josephgoksu/TaskWing/internal/app"
	"github.com/josephgoksu/TaskWing/internal/codeintel"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/task"

	pb "github.com/josephgoksu/TaskWing/gen/go/taskwing/v1"
)

// ─── Error Mapping ────────────────────────────────────────────────

// mapError converts app/domain errors to gRPC status codes.
func mapError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "not found"):
		return status.Error(codes.NotFound, msg)
	case strings.Contains(msg, "invalid"),
		strings.Contains(msg, "required"),
		strings.Contains(msg, "must be"):
		return status.Error(codes.InvalidArgument, msg)
	case strings.Contains(msg, "already"),
		strings.Contains(msg, "conflict"):
		return status.Error(codes.AlreadyExists, msg)
	case strings.Contains(msg, "unavailable"),
		strings.Contains(msg, "llm"),
		strings.Contains(msg, "provider"):
		return status.Error(codes.Unavailable, msg)
	default:
		return status.Error(codes.Internal, msg)
	}
}

// ─── Plan Converters ──────────────────────────────────────────────

func planToProto(p *task.Plan) *pb.Plan {
	if p == nil {
		return nil
	}
	out := &pb.Plan{
		Id:             p.ID,
		Goal:           p.Goal,
		EnrichedGoal:   p.EnrichedGoal,
		Status:         planStatusToProto(p.Status),
		TaskCount:      int32(p.TaskCount),
		GenerationMode: generationModeToProto(p.GenerationMode),
		LastAuditReport: p.LastAuditReport,
		CreatedAt:      timestamppb.New(p.CreatedAt),
		UpdatedAt:      timestamppb.New(p.UpdatedAt),
	}

	if p.DraftState != nil {
		out.DraftState = &pb.PlanDraftState{
			CurrentStage:    p.DraftState.CurrentStage,
			CurrentPhaseIdx: int32(p.DraftState.CurrentPhaseIdx),
			EnrichedGoal:    p.DraftState.EnrichedGoal,
			ClarifyHistory:  p.DraftState.ClarifyHistory,
			PhasesFeedback:  p.DraftState.PhasesFeedback,
			LastUpdated:     p.DraftState.LastUpdated,
		}
	}

	for i := range p.Phases {
		out.Phases = append(out.Phases, phaseToProto(&p.Phases[i]))
	}
	for i := range p.Tasks {
		out.Tasks = append(out.Tasks, taskToProto(&p.Tasks[i]))
	}
	return out
}

func phaseToProto(p *task.Phase) *pb.Phase {
	if p == nil {
		return nil
	}
	out := &pb.Phase{
		Id:            p.ID,
		PlanId:        p.PlanID,
		Title:         p.Title,
		Description:   p.Description,
		Rationale:     p.Rationale,
		OrderIndex:    int32(p.OrderIndex),
		Status:        phaseStatusToProto(p.Status),
		ExpectedTasks: int32(p.ExpectedTasks),
		CreatedAt:     timestamppb.New(p.CreatedAt),
		UpdatedAt:     timestamppb.New(p.UpdatedAt),
	}
	for i := range p.Tasks {
		out.Tasks = append(out.Tasks, taskToProto(&p.Tasks[i]))
	}
	return out
}

func planStatusToProto(s task.PlanStatus) pb.PlanStatus {
	switch s {
	case task.PlanStatusDraft:
		return pb.PlanStatus_PLAN_STATUS_DRAFT
	case task.PlanStatusActive:
		return pb.PlanStatus_PLAN_STATUS_ACTIVE
	case task.PlanStatusCompleted:
		return pb.PlanStatus_PLAN_STATUS_COMPLETED
	case task.PlanStatusVerified:
		return pb.PlanStatus_PLAN_STATUS_VERIFIED
	case task.PlanStatusNeedsRevision:
		return pb.PlanStatus_PLAN_STATUS_NEEDS_REVISION
	case task.PlanStatusArchived:
		return pb.PlanStatus_PLAN_STATUS_ARCHIVED
	default:
		return pb.PlanStatus_PLAN_STATUS_UNSPECIFIED
	}
}

func generationModeToProto(m task.GenerationMode) pb.GenerationMode {
	switch m {
	case task.GenerationModeBatch:
		return pb.GenerationMode_GENERATION_MODE_BATCH
	case task.GenerationModeInteractive:
		return pb.GenerationMode_GENERATION_MODE_INTERACTIVE
	default:
		return pb.GenerationMode_GENERATION_MODE_UNSPECIFIED
	}
}

func taskPlanStatusFromString(s string) task.PlanStatus {
	switch s {
	case "draft":
		return task.PlanStatusDraft
	case "active":
		return task.PlanStatusActive
	case "completed":
		return task.PlanStatusCompleted
	case "verified":
		return task.PlanStatusVerified
	case "needs_revision":
		return task.PlanStatusNeedsRevision
	case "archived":
		return task.PlanStatusArchived
	default:
		return task.PlanStatusDraft
	}
}

func phaseStatusToProto(s task.PhaseStatus) pb.PhaseStatus {
	switch s {
	case task.PhaseStatusPending:
		return pb.PhaseStatus_PHASE_STATUS_PENDING
	case task.PhaseStatusExpanded:
		return pb.PhaseStatus_PHASE_STATUS_EXPANDED
	case task.PhaseStatusSkipped:
		return pb.PhaseStatus_PHASE_STATUS_SKIPPED
	default:
		return pb.PhaseStatus_PHASE_STATUS_UNSPECIFIED
	}
}

// ─── Task Converters ──────────────────────────────────────────────

func taskToProto(t *task.Task) *pb.Task {
	if t == nil {
		return nil
	}
	return &pb.Task{
		Id:                 t.ID,
		PlanId:             t.PlanID,
		PhaseId:            t.PhaseID,
		Title:              t.Title,
		Description:        t.Description,
		Status:             taskStatusToProto(t.Status),
		Priority:           int32(t.Priority),
		Complexity:         t.Complexity,
		AssignedAgent:      t.AssignedAgent,
		ParentTaskId:       t.ParentTaskID,
		ContextSummary:     t.ContextSummary,
		AcceptanceCriteria: t.AcceptanceCriteria,
		ValidationSteps:    t.ValidationSteps,
		Scope:              t.Scope,
		Keywords:           t.Keywords,
		SuggestedAskQueries: t.SuggestedAskQueries,
		ClaimedBy:          t.ClaimedBy,
		ClaimedAt:          timestamppb.New(t.ClaimedAt),
		CompletedAt:        timestamppb.New(t.CompletedAt),
		CompletionSummary:  t.CompletionSummary,
		FilesModified:      t.FilesModified,
		ExpectedFiles:      t.ExpectedFiles,
		GitBaseline:        t.GitBaseline,
		Dependencies:       t.Dependencies,
		ContextNodes:       t.ContextNodes,
		CreatedAt:          timestamppb.New(t.CreatedAt),
		UpdatedAt:          timestamppb.New(t.UpdatedAt),
	}
}

func taskStatusToProto(s task.TaskStatus) pb.TaskStatus {
	switch s {
	case task.StatusDraft:
		return pb.TaskStatus_TASK_STATUS_DRAFT
	case task.StatusPending:
		return pb.TaskStatus_TASK_STATUS_PENDING
	case task.StatusInProgress:
		return pb.TaskStatus_TASK_STATUS_IN_PROGRESS
	case task.StatusVerifying:
		return pb.TaskStatus_TASK_STATUS_VERIFYING
	case task.StatusCompleted:
		return pb.TaskStatus_TASK_STATUS_COMPLETED
	case task.StatusFailed:
		return pb.TaskStatus_TASK_STATUS_FAILED
	case task.StatusBlocked:
		return pb.TaskStatus_TASK_STATUS_BLOCKED
	case task.StatusReady:
		return pb.TaskStatus_TASK_STATUS_READY
	default:
		return pb.TaskStatus_TASK_STATUS_UNSPECIFIED
	}
}

func taskResultToProto(r *app.TaskResult) *pb.TaskResponse {
	if r == nil {
		return &pb.TaskResponse{}
	}
	return &pb.TaskResponse{
		Success:            r.Success,
		Message:            r.Message,
		Task:               taskToProto(r.Task),
		Plan:               planToProto(r.Plan),
		Hint:               r.Hint,
		Context:            r.Context,
		GitBranch:          r.GitBranch,
		GitWorkflowApplied: r.GitWorkflowApplied,
	}
}

// ─── Knowledge Converters ─────────────────────────────────────────

func knowledgeNodeToProto(n *knowledge.NodeResponse) *pb.KnowledgeNode {
	if n == nil {
		return nil
	}
	evidence := make([]*pb.EvidenceRef, len(n.Evidence))
	for i, e := range n.Evidence {
		evidence[i] = &pb.EvidenceRef{File: e.File, Lines: e.Lines}
	}
	return &pb.KnowledgeNode{
		Id:                 n.ID,
		Type:               n.Type,
		Summary:            n.Summary,
		Content:            n.Content,
		ConfidenceScore:    n.ConfidenceScore,
		VerificationStatus: n.VerificationStatus,
		MatchScore:         n.MatchScore,
		Evidence:           evidence,
		DebtScore:          n.DebtScore,
		DebtReason:         n.DebtReason,
		RefactorHint:       n.RefactorHint,
		DebtWarning:        n.DebtWarning,
	}
}

func askResultToProto(r *app.AskResult) *pb.SearchResponse {
	if r == nil {
		return &pb.SearchResponse{}
	}
	nodes := make([]*pb.KnowledgeNode, len(r.Results))
	for i := range r.Results {
		nodes[i] = knowledgeNodeToProto(&r.Results[i])
	}
	symbols := make([]*pb.SymbolResult, len(r.Symbols))
	for i, s := range r.Symbols {
		symbols[i] = &pb.SymbolResult{
			Name:       s.Name,
			Kind:       s.Kind,
			FilePath:   s.FilePath,
			StartLine:  int32(s.StartLine),
			EndLine:    int32(s.EndLine),
			Signature:  s.Signature,
			DocComment: s.DocComment,
			ModulePath: s.ModulePath,
			Visibility: s.Visibility,
			Language:   s.Language,
			Location:   s.Location,
		}
	}
	return &pb.SearchResponse{
		Query:          r.Query,
		RewrittenQuery: r.RewrittenQuery,
		Pipeline:       r.Pipeline,
		Results:        nodes,
		Symbols:        symbols,
		Total:          int32(r.Total),
		TotalSymbols:   int32(r.TotalSymbols),
		Answer:         r.Answer,
		Warning:        r.Warning,
	}
}

func projectSummaryToProto(s *knowledge.ProjectSummary) *pb.ProjectSummary {
	if s == nil {
		return nil
	}
	out := &pb.ProjectSummary{
		Total: int32(s.Total),
		Types: make(map[string]*pb.TypeSummary, len(s.Types)),
	}
	if s.Overview != nil {
		out.Overview = &pb.ProjectOverview{
			ShortDescription: s.Overview.ShortDescription,
			LongDescription:  s.Overview.LongDescription,
		}
	}
	for k, v := range s.Types {
		out.Types[k] = &pb.TypeSummary{Count: int32(v.Count)}
	}
	return out
}

// ─── Code Intelligence Converters ─────────────────────────────────

func symbolToProto(s *codeintel.Symbol) *pb.Symbol {
	if s == nil {
		return nil
	}
	return &pb.Symbol{
		Id:         s.ID,
		Name:       s.Name,
		Kind:       symbolKindToProto(s.Kind),
		FilePath:   s.FilePath,
		StartLine:  int32(s.StartLine),
		EndLine:    int32(s.EndLine),
		Signature:  s.Signature,
		DocComment: s.DocComment,
		ModulePath: s.ModulePath,
		Visibility: s.Visibility,
		Language:   s.Language,
		Location:   s.Location(),
	}
}

func symbolKindToProto(k codeintel.SymbolKind) pb.SymbolKind {
	switch k {
	case codeintel.SymbolFunction:
		return pb.SymbolKind_SYMBOL_KIND_FUNCTION
	case codeintel.SymbolMethod:
		return pb.SymbolKind_SYMBOL_KIND_METHOD
	case codeintel.SymbolStruct:
		return pb.SymbolKind_SYMBOL_KIND_STRUCT
	case codeintel.SymbolInterface:
		return pb.SymbolKind_SYMBOL_KIND_INTERFACE
	case codeintel.SymbolType:
		return pb.SymbolKind_SYMBOL_KIND_TYPE
	case codeintel.SymbolVariable:
		return pb.SymbolKind_SYMBOL_KIND_VARIABLE
	case codeintel.SymbolConstant:
		return pb.SymbolKind_SYMBOL_KIND_CONSTANT
	case codeintel.SymbolField:
		return pb.SymbolKind_SYMBOL_KIND_FIELD
	case codeintel.SymbolPackage:
		return pb.SymbolKind_SYMBOL_KIND_PACKAGE
	default:
		return pb.SymbolKind_SYMBOL_KIND_UNSPECIFIED
	}
}

func symbolSearchResultToProto(r *codeintel.SymbolSearchResult) *pb.SymbolSearchResult {
	if r == nil {
		return nil
	}
	return &pb.SymbolSearchResult{
		Symbol: symbolToProto(&r.Symbol),
		Score:  r.Score,
		Source: r.Source,
	}
}

func impactNodeToProto(n *codeintel.ImpactNode) *pb.ImpactNode {
	if n == nil {
		return nil
	}
	return &pb.ImpactNode{
		Symbol:   symbolToProto(&n.Symbol),
		Depth:    int32(n.Depth),
		Relation: n.Relation,
	}
}

// symbolResponseToProto converts an app.SymbolResponse to a proto Symbol.
func symbolResponseToProto(s *app.SymbolResponse) *pb.Symbol {
	if s == nil {
		return nil
	}
	return &pb.Symbol{
		Name:       s.Name,
		Kind:       symbolKindStringToProto(s.Kind),
		FilePath:   s.FilePath,
		StartLine:  int32(s.StartLine),
		EndLine:    int32(s.EndLine),
		Signature:  s.Signature,
		DocComment: s.DocComment,
		ModulePath: s.ModulePath,
		Visibility: s.Visibility,
		Language:   s.Language,
		Location:   s.Location,
	}
}

func symbolKindStringToProto(k string) pb.SymbolKind {
	switch codeintel.SymbolKind(k) {
	case codeintel.SymbolFunction:
		return pb.SymbolKind_SYMBOL_KIND_FUNCTION
	case codeintel.SymbolMethod:
		return pb.SymbolKind_SYMBOL_KIND_METHOD
	case codeintel.SymbolStruct:
		return pb.SymbolKind_SYMBOL_KIND_STRUCT
	case codeintel.SymbolInterface:
		return pb.SymbolKind_SYMBOL_KIND_INTERFACE
	case codeintel.SymbolType:
		return pb.SymbolKind_SYMBOL_KIND_TYPE
	case codeintel.SymbolVariable:
		return pb.SymbolKind_SYMBOL_KIND_VARIABLE
	case codeintel.SymbolConstant:
		return pb.SymbolKind_SYMBOL_KIND_CONSTANT
	case codeintel.SymbolField:
		return pb.SymbolKind_SYMBOL_KIND_FIELD
	case codeintel.SymbolPackage:
		return pb.SymbolKind_SYMBOL_KIND_PACKAGE
	default:
		return pb.SymbolKind_SYMBOL_KIND_UNSPECIFIED
	}
}

func explainResultToProto(r *app.ExplainResult) *pb.ExplainResponse {
	if r == nil {
		return &pb.ExplainResponse{}
	}

	callers := make([]*pb.CallNode, len(r.Callers))
	for i := range r.Callers {
		callers[i] = &pb.CallNode{
			Symbol:   symbolResponseToProto(&r.Callers[i].Symbol),
			Relation: r.Callers[i].Relation,
			Depth:    int32(r.Callers[i].Depth),
		}
	}

	callees := make([]*pb.CallNode, len(r.Callees))
	for i := range r.Callees {
		callees[i] = &pb.CallNode{
			Symbol:   symbolResponseToProto(&r.Callees[i].Symbol),
			Relation: r.Callees[i].Relation,
			Depth:    int32(r.Callees[i].Depth),
		}
	}

	decisions := make([]*pb.KnowledgeNode, len(r.Decisions))
	for i := range r.Decisions {
		decisions[i] = knowledgeNodeToProto(&r.Decisions[i])
	}

	patterns := make([]*pb.KnowledgeNode, len(r.Patterns))
	for i := range r.Patterns {
		patterns[i] = knowledgeNodeToProto(&r.Patterns[i])
	}

	sourceCode := make([]*pb.CodeSnippet, len(r.SourceCode))
	for i, sc := range r.SourceCode {
		sourceCode[i] = &pb.CodeSnippet{
			FilePath:  sc.FilePath,
			StartLine: int32(sc.StartLine),
			EndLine:   int32(sc.EndLine),
			Code:      sc.Content,
			Language:  sc.Kind,
		}
	}

	return &pb.ExplainResponse{
		Symbol:  symbolResponseToProto(&r.Symbol),
		Callers: callers,
		Callees: callees,
		ImpactStats: &pb.ImpactStats{
			DirectCallers:        int32(r.ImpactStats.DirectCallers),
			DirectCallees:        int32(r.ImpactStats.DirectCallees),
			TransitiveDependents: int32(r.ImpactStats.TransitiveDependents),
			AffectedFiles:        int32(r.ImpactStats.AffectedFiles),
			MaxDepthReached:      int32(r.ImpactStats.MaxDepthReached),
		},
		Decisions:   decisions,
		Patterns:    patterns,
		SourceCode:  sourceCode,
		Explanation: r.Explanation,
	}
}

// ─── Activity Converters ──────────────────────────────────────────

func activityEntryToProto(e *impl.ActivityEntry) *pb.ActivityEntry {
	if e == nil {
		return nil
	}
	return &pb.ActivityEntry{
		Id:        fmt.Sprintf("%d", e.ID),
		Type:      e.Type,
		Message:   e.Message,
		Timestamp: timestamppb.New(e.Timestamp),
	}
}
