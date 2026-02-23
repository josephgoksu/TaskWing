package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/josephgoksu/TaskWing/internal/app"
	"github.com/josephgoksu/TaskWing/internal/task"

	pb "github.com/josephgoksu/TaskWing/gen/go/taskwing/v1"
)

// TaskService adapts internal/app.TaskApp to the gRPC TaskService.
type TaskService struct {
	pb.UnimplementedTaskServiceServer
	appCtx *app.Context
}

// NewTaskService creates a TaskService adapter.
func NewTaskService(appCtx *app.Context) *TaskService {
	return &TaskService{appCtx: appCtx}
}

func (s *TaskService) ListTasks(ctx context.Context, req *pb.ListTasksRequest) (*pb.ListTasksResponse, error) {
	taskApp := app.NewTaskApp(s.appCtx)
	tasks, err := taskApp.List(ctx, req.PlanId)
	if err != nil {
		return nil, mapError(err)
	}

	pbTasks := make([]*pb.Task, len(tasks))
	for i := range tasks {
		pbTasks[i] = taskToProto(&tasks[i])
	}
	return &pb.ListTasksResponse{Tasks: pbTasks}, nil
}

func (s *TaskService) GetTask(ctx context.Context, req *pb.GetTaskRequest) (*pb.GetTaskResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "task id is required")
	}
	task, err := s.appCtx.Repo.GetTask(req.Id)
	if err != nil {
		return nil, mapError(err)
	}
	return &pb.GetTaskResponse{Task: taskToProto(task)}, nil
}

func (s *TaskService) CreateTask(ctx context.Context, req *pb.CreateTaskRequest) (*pb.CreateTaskResponse, error) {
	if req.PlanId == "" {
		return nil, status.Error(codes.InvalidArgument, "plan_id is required")
	}
	if req.Title == "" {
		return nil, status.Error(codes.InvalidArgument, "title is required")
	}
	t := &task.Task{
		PlanID:      req.PlanId,
		Title:       req.Title,
		Description: req.Description,
		Priority:    int(req.Priority),
	}
	if err := s.appCtx.Repo.CreateTask(t); err != nil {
		return nil, mapError(err)
	}
	return &pb.CreateTaskResponse{Task: taskToProto(t)}, nil
}

func (s *TaskService) DeleteTask(ctx context.Context, req *pb.DeleteTaskRequest) (*pb.DeleteTaskResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "task id is required")
	}
	if err := s.appCtx.Repo.DeleteTask(req.Id); err != nil {
		return nil, mapError(err)
	}
	return &pb.DeleteTaskResponse{}, nil
}

func (s *TaskService) Next(ctx context.Context, req *pb.NextTaskRequest) (*pb.TaskResponse, error) {
	taskApp := app.NewTaskApp(s.appCtx)

	result, err := taskApp.Next(ctx, app.TaskNextOptions{
		PlanID:            req.PlanId,
		SessionID:         req.SessionId,
		AutoStart:         req.AutoStart,
		CreateBranch:      req.CreateBranch,
		SkipUnpushedCheck: req.SkipUnpushedCheck,
	})
	if err != nil {
		return nil, mapError(err)
	}

	return taskResultToProto(result), nil
}

func (s *TaskService) Current(ctx context.Context, req *pb.CurrentTaskRequest) (*pb.TaskResponse, error) {
	if req.SessionId == "" {
		return nil, status.Error(codes.InvalidArgument, "session_id is required")
	}

	taskApp := app.NewTaskApp(s.appCtx)
	result, err := taskApp.Current(ctx, req.SessionId, req.PlanId)
	if err != nil {
		return nil, mapError(err)
	}

	return taskResultToProto(result), nil
}

func (s *TaskService) Start(ctx context.Context, req *pb.StartTaskRequest) (*pb.TaskResponse, error) {
	if req.TaskId == "" {
		return nil, status.Error(codes.InvalidArgument, "task_id is required")
	}
	if req.SessionId == "" {
		return nil, status.Error(codes.InvalidArgument, "session_id is required")
	}

	taskApp := app.NewTaskApp(s.appCtx)
	result, err := taskApp.Start(ctx, app.TaskStartOptions{
		TaskID:    req.TaskId,
		SessionID: req.SessionId,
	})
	if err != nil {
		return nil, mapError(err)
	}

	return taskResultToProto(result), nil
}

func (s *TaskService) Complete(ctx context.Context, req *pb.CompleteTaskRequest) (*pb.TaskResponse, error) {
	if req.TaskId == "" {
		return nil, status.Error(codes.InvalidArgument, "task_id is required")
	}

	taskApp := app.NewTaskApp(s.appCtx)
	result, err := taskApp.Complete(ctx, app.TaskCompleteOptions{
		TaskID:        req.TaskId,
		Summary:       req.Summary,
		FilesModified: req.FilesModified,
	})
	if err != nil {
		return nil, mapError(err)
	}

	return taskResultToProto(result), nil
}
