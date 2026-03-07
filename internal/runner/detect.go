package runner

import "os/exec"

// cliCandidate maps a binary name to a CLIType.
type cliCandidate struct {
	binary  string
	cliType CLIType
}

// defaultCandidates defines the search order for AI CLI binaries.
// Priority: Claude Code > Gemini CLI > Codex CLI.
var defaultCandidates = []cliCandidate{
	{"claude", CLIClaude},
	{"gemini", CLIGemini},
	{"codex", CLICodex},
}

// DetectCLIs scans PATH for installed AI CLI binaries.
// Returns all detected CLIs in priority order.
func DetectCLIs() []DetectedCLI {
	var detected []DetectedCLI
	for _, c := range defaultCandidates {
		path, err := exec.LookPath(c.binary)
		if err == nil {
			detected = append(detected, DetectedCLI{
				Type:       c.cliType,
				BinaryPath: path,
			})
		}
	}
	return detected
}

