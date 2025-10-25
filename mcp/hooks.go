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
	GetArchiveStore      func() (store.ArchiveStore, error)
	SuggestLessons       func(models.Task) ([]string, bool)
	PolishLessons        func(string) (string, bool)
	ArchiveAndDelete     func(store.TaskStore, store.ArchiveStore, models.Task, string, []string) ([]models.ArchiveEntry, error)
	EncryptFile          func(string, string, string) error
	DecryptFile          func(string, string, string) error
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
	GetArchiveStore: func() (store.ArchiveStore, error) {
		return nil, fmt.Errorf("archive store not configured")
	},
	SuggestLessons: func(models.Task) ([]string, bool) { return nil, false },
	PolishLessons:  func(string) (string, bool) { return "", false },
	ArchiveAndDelete: func(store.TaskStore, store.ArchiveStore, models.Task, string, []string) ([]models.ArchiveEntry, error) {
		return nil, fmt.Errorf("archive helpers not configured")
	},
	EncryptFile: func(string, string, string) error { return fmt.Errorf("encryption not configured") },
	DecryptFile: func(string, string, string) error { return fmt.Errorf("decryption not configured") },
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
	if h.GetArchiveStore != nil {
		hooks.GetArchiveStore = h.GetArchiveStore
	}
	if h.SuggestLessons != nil {
		hooks.SuggestLessons = h.SuggestLessons
	}
	if h.PolishLessons != nil {
		hooks.PolishLessons = h.PolishLessons
	}
	if h.ArchiveAndDelete != nil {
		hooks.ArchiveAndDelete = h.ArchiveAndDelete
	}
	if h.EncryptFile != nil {
		hooks.EncryptFile = h.EncryptFile
	}
	if h.DecryptFile != nil {
		hooks.DecryptFile = h.DecryptFile
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

func logError(err error) {
	if hooks.LogError != nil {
		hooks.LogError(err)
	}
}

func logToolCall(name string, params interface{}) {
	if hooks.LogToolCall != nil {
		hooks.LogToolCall(name, params)
	}
}

func archiveStore() (store.ArchiveStore, error) {
	return hooks.GetArchiveStore()
}

func suggestLessons(task models.Task) ([]string, bool) {
	return hooks.SuggestLessons(task)
}

func polishLessons(text string) (string, bool) {
	return hooks.PolishLessons(text)
}

func archiveAndDelete(taskStore store.TaskStore, arch store.ArchiveStore, root models.Task, lessons string, tags []string) ([]models.ArchiveEntry, error) {
	return hooks.ArchiveAndDelete(taskStore, arch, root, lessons, tags)
}

func encryptFile(in, out, key string) error {
	return hooks.EncryptFile(in, out, key)
}

func decryptFile(in, out, key string) error {
	return hooks.DecryptFile(in, out, key)
}

func resolveTask(taskStore store.TaskStore, reference string) (*models.Task, error) {
	return hooks.ResolveTaskReference(taskStore, reference)
}

func currentVersion() string {
	return hooks.GetVersion()
}

func setCurrentTask(id string) error {
	return hooks.SetCurrentTask(id)
}

func clearCurrentTask() error {
	return hooks.ClearCurrentTask()
}

func writeJSONFile(path string, data interface{}) error {
	return hooks.WriteJSONFile(path, data)
}

func createLLMProvider(cfg *types.LLMConfig) (llm.Provider, error) {
	return hooks.CreateLLMProvider(cfg)
}

func envPrefix() string {
	return hooks.EnvPrefix
}
