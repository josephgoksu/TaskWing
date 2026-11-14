package engine

import (
	"fmt"
	"strings"
)

func leadPrompt(task string, maxCards int, feedback string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `You are the lead agent of a TaskWing run. Read this repository, then split the task below into pieces that separate coding agents build at the same time, each in its own sandbox with its own copy of the repository.

Task: %s

Rules for the split:
- 2 to %d cards. Use fewer cards when the task is small; 1 card is allowed only if the task cannot be split.
- Every card owns its own files, given as path patterns ("billing/retry/**", "api/webhook.go"). No two cards may own the same path. Tests for a card's code belong to that card.
- If one card needs something another card makes (a type, a function name, a route), write it as a seam with an owner, and put the exact contract (names, signatures, formats) in both cards' prompts.
- Each prompt is complete on its own: the agent sees only its card, not the task or the other cards. Say what to build, which files, which existing code to follow, and how to check it works (the exact test command).
- Do not write code yourself. Do not change any file.

Answer with ONLY this JSON, no other text:
{"task": "<the task in one line>",
 "cards": [{"id": "c1", "title": "<3 to 6 words>", "owns": ["<pattern>"], "prompt": "<full instructions>"}],
 "seams": [{"between": ["c1", "c2"], "owner": "c1", "contract": "<exact contract>"}]}
`, task, maxCards)
	if feedback != "" {
		fmt.Fprintf(&b, "\nYour previous plan was refused: %s\nFix it and answer with the JSON again.\n", feedback)
	}
	return b.String()
}

func examPrompt(task string) string {
	return fmt.Sprintf(`You write the acceptance exam for a coding task, before anyone builds it. Other agents will build the task without seeing your exam. The exam runs on their joined work in a clean sandbox.

Task: %s

Do this:
1. Read the repository: its language, its test framework, how tests are run.
2. Write 3 to 6 checks. Each check is one plain sentence a non-programmer understands, plus one shell command, run from the repository root, that exits 0 only when the check holds.
3. Write the tests behind the checks as NEW files whose path contains "taskwing_exam" (for example "billing/taskwing_exam_test.go" or "tests/taskwing_exam.spec.ts"). Never change an existing file.
4. The tests must fail on the current code, because the feature is not built yet, and pass once it is built well. Test behaviour through public interfaces the task implies, not private names an agent could choose differently. When the task leaves a name open, pick the most natural one and say it in the check.
5. One check should confirm that the existing tests still pass.

When the files are written, answer with ONLY this JSON, no other text:
{"checks": [{"text": "<plain sentence>", "command": "<shell command>"}]}
`, task)
}

func workerPrompt(p *Plan, c Card, n int, repoDir string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are agent %02d in a TaskWing run. The repository is checked out at %s on its own branch. Work only there.\n\n", n, repoDir)
	fmt.Fprintf(&b, "Your piece: %s\n\n%s\n\n", c.Title, strings.TrimSpace(c.Prompt))
	var seams []string
	for _, s := range p.Seams {
		for _, id := range s.Between {
			if id == c.ID {
				role := "uses"
				if s.Owner == c.ID {
					role = "owns"
				}
				seams = append(seams, fmt.Sprintf("- You %s this contract: %s", role, s.Contract))
				break
			}
		}
	}
	if len(seams) > 0 {
		b.WriteString("Contracts with the other agents (follow them exactly):\n" + strings.Join(seams, "\n") + "\n\n")
	}
	fmt.Fprintf(&b, `Rules:
- Create or change only files matching: %s. Other agents own every other file, and their work is joined with yours later.
- Build it, then run the relevant tests until they pass.
- Commit your work on the current branch with a short plain message. No trailers, no co-author lines, no mention of AI. Do not push, do not add remotes, do not create branches.
- When you are done, end your answer with one line that starts with "PROOF:" and says in plain words what you showed working, for example "PROOF: retries land at +24h, +72h and +7d; 12 tests pass".
`, strings.Join(c.Owns, ", "))
	return b.String()
}
