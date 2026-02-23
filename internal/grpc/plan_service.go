package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/josephgoksu/TaskWing/internal/app"
	"github.com/josephgoksu/TaskWing/internal/task"

	pb "github.com/josephgoksu/TaskWing/gen/go/taskwing/v1"
)

// PlanService adapts internal/app.PlanApp to the gRPC PlanService.
type PlanService struct {
	pb.UnimplementedPlanServiceServer
	appCtx *app.Context
}

// NewPlanService creates a PlanService adapter.
func NewPlanService(appCtx *app.Context) *PlanService {
	return &PlanService{appCtx: appCtx}
}

func (s *PlanService) ListPlans(ctx context.Context, _ *pb.ListPlansRequest) (*pb.ListPlansResponse, error) {
	plans, err := s.appCtx.Repo.ListPlans()
	if err != nil {
		return nil, mapError(err)
	}
	pbPlans := make([]*pb.Plan, len(plans))
	for i := range plans {
		pbPlans[i] = planToProto(&plans[i])
	}
	return &pb.ListPlansResponse{Plans: pbPlans}, nil
}

func (s *PlanService) GetPlan(ctx context.Context, req *pb.GetPlanRequest) (*pb.GetPlanResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "plan id is required")
	}
	plan, err := s.appCtx.Repo.GetPlan(req.Id)
	if err != nil {
		return nil, mapError(err)
	}
	return &pb.GetPlanResponse{Plan: planToProto(plan)}, nil
}

func (s *PlanService) GetActivePlan(ctx context.Context, _ *pb.GetActivePlanRequest) (*pb.GetActivePlanResponse, error) {
	plan, err := s.appCtx.Repo.GetActivePlan()
	if err != nil {
		return nil, mapError(err)
	}
	if plan == nil {
		return &pb.GetActivePlanResponse{}, nil
	}
	return &pb.GetActivePlanResponse{Plan: planToProto(plan)}, nil
}

func (s *PlanService) CreatePlan(ctx context.Context, req *pb.CreatePlanRequest) (*pb.CreatePlanResponse, error) {
	if req.Goal == "" {
		return nil, status.Error(codes.InvalidArgument, "goal is required")
	}
	plan := &task.Plan{
		Goal:         req.Goal,
		EnrichedGoal: req.EnrichedGoal,
	}
	if err := s.appCtx.Repo.CreatePlan(plan); err != nil {
		return nil, mapError(err)
	}
	return &pb.CreatePlanResponse{Plan: planToProto(plan)}, nil
}

func (s *PlanService) UpdatePlan(ctx context.Context, req *pb.UpdatePlanRequest) (*pb.UpdatePlanResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "plan id is required")
	}
	// Get current plan to fill in defaults for fields not specified.
	plan, err := s.appCtx.Repo.GetPlan(req.Id)
	if err != nil {
		return nil, mapError(err)
	}
	goal := plan.Goal
	if req.Goal != "" {
		goal = req.Goal
	}
	enrichedGoal := plan.EnrichedGoal
	if req.EnrichedGoal != "" {
		enrichedGoal = req.EnrichedGoal
	}
	planStatus := plan.Status
	if req.Status != "" {
		planStatus = taskPlanStatusFromString(req.Status)
	}
	if err := s.appCtx.Repo.UpdatePlan(req.Id, goal, enrichedGoal, planStatus); err != nil {
		return nil, mapError(err)
	}
	updated, err := s.appCtx.Repo.GetPlan(req.Id)
	if err != nil {
		return nil, mapError(err)
	}
	return &pb.UpdatePlanResponse{Plan: planToProto(updated)}, nil
}

func (s *PlanService) DeletePlan(ctx context.Context, req *pb.DeletePlanRequest) (*pb.DeletePlanResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "plan id is required")
	}
	if err := s.appCtx.Repo.DeletePlan(req.Id); err != nil {
		return nil, mapError(err)
	}
	return &pb.DeletePlanResponse{}, nil
}

func (s *PlanService) ActivatePlan(ctx context.Context, req *pb.ActivatePlanRequest) (*pb.ActivatePlanResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "plan id is required")
	}
	if err := s.appCtx.Repo.SetActivePlan(req.Id); err != nil {
		return nil, mapError(err)
	}
	plan, err := s.appCtx.Repo.GetPlan(req.Id)
	if err != nil {
		return nil, mapError(err)
	}
	return &pb.ActivatePlanResponse{Plan: planToProto(plan)}, nil
}

func (s *PlanService) Clarify(ctx context.Context, req *pb.ClarifyRequest) (*pb.ClarifyResponse, error) {
	planApp := app.NewPlanApp(s.appCtx)

	answers := make([]app.ClarifyAnswer, len(req.Answers))
	for i, a := range req.Answers {
		answers[i] = app.ClarifyAnswer{Question: a.Question, Answer: a.Answer}
	}

	maxRounds := int(req.MaxRounds)
	if maxRounds == 0 {
		maxRounds = 5
	}

	result, err := planApp.Clarify(ctx, app.ClarifyOptions{
		Goal:             req.Goal,
		ClarifySessionID: req.ClarifySessionId,
		Answers:          answers,
		AutoAnswer:       req.AutoAnswer,
		MaxRounds:        maxRounds,
	})
	if err != nil {
		return nil, mapError(err)
	}

	return &pb.ClarifyResponse{
		Success:          result.Success,
		ClarifySessionId: result.ClarifySessionID,
		Questions:        result.Questions,
		GoalSummary:      result.GoalSummary,
		EnrichedGoal:     result.EnrichedGoal,
		IsReadyToPlan:    result.IsReadyToPlan,
		RoundIndex:       int32(result.RoundIndex),
		MaxRoundsReached: result.MaxRoundsReached,
		ContextUsed:      result.ContextUsed,
		Message:          result.Message,
	}, nil
}

func (s *PlanService) Generate(ctx context.Context, req *pb.GenerateRequest) (*pb.GenerateResponse, error) {
	planApp := app.NewPlanApp(s.appCtx)

	result, err := planApp.Generate(ctx, app.GenerateOptions{
		Goal:             req.Goal,
		ClarifySessionID: req.ClarifySessionId,
		EnrichedGoal:     req.EnrichedGoal,
		Save:             req.Save,
	})
	if err != nil {
		return nil, mapError(err)
	}

	tasks := make([]*pb.Task, len(result.Tasks))
	for i := range result.Tasks {
		tasks[i] = taskToProto(&result.Tasks[i])
	}

	var stats *pb.SemanticValidationStats
	if result.ValidationStats != nil {
		stats = &pb.SemanticValidationStats{
			TotalChecks: int32(result.ValidationStats.TotalTasks),
			Passed:      int32(result.ValidationStats.PathsChecked - result.ValidationStats.PathsMissing),
			Warnings:    int32(result.ValidationStats.CommandsInvalid),
			Errors:      int32(result.ValidationStats.PathsMissing),
		}
	}

	return &pb.GenerateResponse{
		Success:          result.Success,
		Tasks:            tasks,
		PlanId:           result.PlanID,
		Goal:             result.Goal,
		EnrichedGoal:     result.EnrichedGoal,
		Message:          result.Message,
		Hint:             result.Hint,
		SemanticWarnings: result.SemanticWarnings,
		SemanticErrors:   result.SemanticErrors,
		ValidationStats:  stats,
	}, nil
}

func (s *PlanService) GenerateStream(req *pb.GenerateRequest, stream pb.PlanService_GenerateStreamServer) error {
	// Delegate to Generate, then emit tasks incrementally from the unary result.
	// TODO(taskwing#generate-streaming): wire to a true streaming PlanApp.Generate variant.
	resp, err := s.Generate(stream.Context(), req)
	if err != nil {
		return err
	}

	for _, t := range resp.Tasks {
		if err := stream.Send(&pb.GenerateStreamResponse{
			Event: &pb.GenerateStreamResponse_Task{Task: t},
		}); err != nil {
			return err
		}
	}

	return stream.Send(&pb.GenerateStreamResponse{
		Event: &pb.GenerateStreamResponse_Complete{Complete: &pb.GenerateComplete{
			Success:          resp.Success,
			PlanId:           resp.PlanId,
			Goal:             resp.Goal,
			EnrichedGoal:     resp.EnrichedGoal,
			Message:          resp.Message,
			Hint:             resp.Hint,
			SemanticWarnings: resp.SemanticWarnings,
			SemanticErrors:   resp.SemanticErrors,
			ValidationStats:  resp.ValidationStats,
			TotalTasks:       int32(len(resp.Tasks)),
		}},
	})
}

func (s *PlanService) Decompose(ctx context.Context, req *pb.DecomposeRequest) (*pb.DecomposeResponse, error) {
	planApp := app.NewPlanApp(s.appCtx)

	result, err := planApp.Decompose(ctx, app.DecomposeOptions{
		PlanID:       req.PlanId,
		Goal:         req.Goal,
		EnrichedGoal: req.EnrichedGoal,
		Feedback:     req.Feedback,
	})
	if err != nil {
		return nil, mapError(err)
	}

	phases := make([]*pb.Phase, len(result.Phases))
	for i := range result.Phases {
		phases[i] = phaseToProto(&result.Phases[i])
	}

	return &pb.DecomposeResponse{
		Success:   result.Success,
		PlanId:    result.PlanID,
		Phases:    phases,
		Rationale: result.Rationale,
		Message:   result.Message,
		Hint:      result.Hint,
	}, nil
}

func (s *PlanService) Expand(ctx context.Context, req *pb.ExpandRequest) (*pb.ExpandResponse, error) {
	planApp := app.NewPlanApp(s.appCtx)

	result, err := planApp.Expand(ctx, app.ExpandOptions{
		PlanID:     req.PlanId,
		PhaseID:    req.PhaseId,
		PhaseIndex: int(req.PhaseIndex),
		Feedback:   req.Feedback,
	})
	if err != nil {
		return nil, mapError(err)
	}

	tasks := make([]*pb.Task, len(result.Tasks))
	for i := range result.Tasks {
		tasks[i] = taskToProto(&result.Tasks[i])
	}

	return &pb.ExpandResponse{
		Success:         result.Success,
		PlanId:          result.PlanID,
		PhaseId:         result.PhaseID,
		PhaseTitle:      result.PhaseTitle,
		Tasks:           tasks,
		Rationale:       result.Rationale,
		RemainingPhases: int32(result.RemainingPhases),
		NextPhaseTitle:  result.NextPhaseTitle,
		Message:         result.Message,
		Hint:            result.Hint,
	}, nil
}

func (s *PlanService) Finalize(ctx context.Context, req *pb.FinalizeRequest) (*pb.FinalizeResponse, error) {
	if req.PlanId == "" {
		return nil, status.Error(codes.InvalidArgument, "plan_id is required")
	}

	planApp := app.NewPlanApp(s.appCtx)
	result, err := planApp.Finalize(ctx, app.FinalizeOptions{
		PlanID: req.PlanId,
	})
	if err != nil {
		return nil, mapError(err)
	}

	return &pb.FinalizeResponse{
		Success:     result.Success,
		PlanId:      result.PlanID,
		Status:      result.Status,
		TotalPhases: int32(result.TotalPhases),
		TotalTasks:  int32(result.TotalTasks),
		Message:     result.Message,
		Hint:        result.Hint,
	}, nil
}

func (s *PlanService) Audit(ctx context.Context, req *pb.AuditRequest) (*pb.AuditResponse, error) {
	planApp := app.NewPlanApp(s.appCtx)
	result, err := planApp.Audit(ctx, app.AuditOptions{
		PlanID:  req.PlanId,
		AutoFix: req.AutoFix,
	})
	if err != nil {
		return nil, mapError(err)
	}

	return &pb.AuditResponse{
		Success:        result.Success,
		PlanId:         result.PlanID,
		Status:         result.Status,
		PlanStatus:     planStatusToProto(result.PlanStatus),
		BuildPassed:    result.BuildPassed,
		TestsPassed:    result.TestsPassed,
		SemanticIssues: result.SemanticIssues,
		FixesApplied:   result.FixesApplied,
		RetryCount:     int32(result.RetryCount),
		Message:        result.Message,
		Hint:           result.Hint,
	}, nil
}
