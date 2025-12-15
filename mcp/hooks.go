package mcp

import (
	"fmt"

	"github.com/josephgoksu/TaskWing/llm"
	"github.com/josephgoksu/TaskWing/models"
	"github.com/josephgoksu/TaskWing/store"
	"github.com/josephgoksu/TaskWing/types"
)

type Hooks struct {
	GetCurrentTask       func() string
	GetConfig            func() *types.AppConfig
	LogInfo              func(string)
	LogError             func(error)
	LogToolCall          func(string, interface{})
	ResolveTaskReference func(store.TaskStore, string) (*models.Task, error)
	GetVersion           func() string
	SetCurrentTask       func(string) error
	ClearCurrentTask     func() error
	WriteJSONFile        func(string, interface{}) error
	CreateLLMProvider    func(*types.LLMConfig) (llm.Provider, error)
	EnvPrefix            string
}

var hooks = Hooks{
	GetCurrentTask: func() string { return "" },
	GetConfig:      func() *types.AppConfig { return &types.AppConfig{} },
	LogInfo:        func(string) {},
	LogError:       func(error) {},
	LogToolCall:    func(string, interface{}) {},
	ResolveTaskReference: func(store.TaskStore, string) (*models.Task, error) {
		return nil, fmt.Errorf("resolver not configured")
	},
	GetVersion:        func() string { return "dev" },
	SetCurrentTask:    func(string) error { return fmt.Errorf("set current task not configured") },
	ClearCurrentTask:  func() error { return fmt.Errorf("clear current task not configured") },
	WriteJSONFile:     func(string, interface{}) error { return fmt.Errorf("write json not configured") },
	CreateLLMProvider: func(*types.LLMConfig) (llm.Provider, error) { return nil, fmt.Errorf("llm provider not configured") },
	EnvPrefix:         "TASKWING",
}

// ConfigureHooks allows the CLI layer to inject runtime dependencies needed by MCP helpers.
func ConfigureHooks(h Hooks) {
	if h.GetCurrentTask != nil {
		hooks.GetCurrentTask = h.GetCurrentTask
	}
	if h.GetConfig != nil {
		hooks.GetConfig = h.GetConfig
	}
	if h.LogInfo != nil {
		hooks.LogInfo = h.LogInfo
	}
	if h.LogError != nil {
		hooks.LogError = h.LogError
	}
	if h.LogToolCall != nil {
		hooks.LogToolCall = h.LogToolCall
	}
	if h.ResolveTaskReference != nil {
		hooks.ResolveTaskReference = h.ResolveTaskReference
	}
	if h.GetVersion != nil {
		hooks.GetVersion = h.GetVersion
	}
	if h.SetCurrentTask != nil {
		hooks.SetCurrentTask = h.SetCurrentTask
	}
	if h.ClearCurrentTask != nil {
		hooks.ClearCurrentTask = h.ClearCurrentTask
	}
	if h.WriteJSONFile != nil {
		hooks.WriteJSONFile = h.WriteJSONFile
	}
	if h.CreateLLMProvider != nil {
		hooks.CreateLLMProvider = h.CreateLLMProvider
	}
	if h.EnvPrefix != "" {
		hooks.EnvPrefix = h.EnvPrefix
	}
}

func currentTaskID() string {
	if hooks.GetCurrentTask == nil {
		return ""
	}
	return hooks.GetCurrentTask()
}

func currentConfig() *types.AppConfig {
	if hooks.GetConfig == nil {
		return &types.AppConfig{}
	}
	cfg := hooks.GetConfig()
	if cfg == nil {
		return &types.AppConfig{}
	}
	return cfg
}

func logInfo(msg string) {
	if hooks.LogInfo != nil {
		hooks.LogInfo(msg)
	}
}

func logToolCall(name string, params interface{}) {
	if hooks.LogToolCall != nil {
		hooks.LogToolCall(name, params)
	}
}

func setCurrentTask(id string) error {
	return hooks.SetCurrentTask(id)
}

func currentVersion() string {
	return hooks.GetVersion()
}
