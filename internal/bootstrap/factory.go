package bootstrap

import (
	"os"
	"path/filepath"

	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/agents/impl"
	"github.com/josephgoksu/TaskWing/internal/llm"
)

// NewDefaultAgents returns the standard set of agents for a bootstrap run.
// If snap is non-nil, agents are adaptively selected based on project state:
//   - deps is skipped if no dependency files exist
//   - code is skipped if no source files exist
//   - git is skipped if the project is not a git repo or has 0 commits
//
// Pass nil for snap to include all agents (safe default).
func NewDefaultAgents(cfg llm.Config, projectPath string, snap *Snapshot) []core.Agent {
	var agents []core.Agent

	// Doc agent is always included (markdown almost always exists)
	agents = append(agents, impl.NewDocAgent(cfg))

	// Deps agent: skip if no dependency manifests found
	if snap == nil || hasDependencyFiles(projectPath) {
		agents = append(agents, impl.NewDepsAgent(cfg))
	}

	// Code agent: skip if snapshot says zero source files
	if snap == nil || snap.FileCount > 0 {
		agents = append(agents, impl.NewCodeAgent(cfg, projectPath))
	}

	// Git agent: skip if not a git repo
	if snap == nil || snap.IsGitRepo {
		agents = append(agents, impl.NewGitAgent(cfg))
	}

	return agents
}

// dependencyManifests lists common dependency file names across ecosystems.
var dependencyManifests = []string{
	"package.json",
	"go.mod",
	"Cargo.toml",
	"requirements.txt",
	"Pipfile",
	"pyproject.toml",
	"pom.xml",
	"build.gradle",
	"build.gradle.kts",
	"Gemfile",
	"composer.json",
	"pubspec.yaml",
}

// hasDependencyFiles checks if any common dependency manifest exists at the project root.
func hasDependencyFiles(basePath string) bool {
	for _, name := range dependencyManifests {
		if _, err := os.Stat(filepath.Join(basePath, name)); err == nil {
			return true
		}
	}
	// Also check for go.mod in subdirectories (monorepo)
	if entries, err := os.ReadDir(basePath); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			for _, name := range dependencyManifests {
				if _, err := os.Stat(filepath.Join(basePath, e.Name(), name)); err == nil {
					return true
				}
			}
		}
	}
	return false
}

