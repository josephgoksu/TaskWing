package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cloudwego/eino/callbacks"
	"github.com/josephgoksu/TaskWing/internal/agents/core"
	"github.com/josephgoksu/TaskWing/internal/agents/planning"
	"github.com/josephgoksu/TaskWing/internal/knowledge"
	"github.com/josephgoksu/TaskWing/internal/memory"
	"github.com/josephgoksu/TaskWing/internal/task"
)

type PlanState int

const (
	StateUninitialized PlanState = iota
	StateInitializing
	StateClarifyingThinking
	StateClarifyingInput
	StateAnsweringQuestion // One-by-one Q&A flow
	StatePlanningThinking
	StatePlanningPulse // Streaming logic
	StateSuccess
	StateError
)

// Layout constants
const (
	DefaultViewportWidth  = 80
	DefaultViewportHeight = 15
	MinViewportHeight     = 10
	DefaultTextareaWidth  = 60
	DefaultTextareaHeight = 6
	MaxTextareaHeight     = 15
	HeaderFooterHeight    = 12 // Space for header + input + footer
	MaxMsgWidth           = 65 // Max width for wrapped messages
	LLMTimeoutSeconds     = 60 // Timeout for LLM operations
)

type PlanModel struct {
	// State
	State          PlanState
	PreviousState  PlanState // For returning from overlays/cancellation
	Err            error
	InitialGoal    string
	EnrichedGoal   string
	PlanID         string
	PlanSummary    string
	ThinkingStatus string // Dynamic status message for spinner

	// Data
	History      string   // Clarification history string
	Msgs         []string // Visible chat log for viewport
	ClarifyTurns int
	KGContext    string // Fetched knowledge graph context

	// Interactive Q&A State
	PendingQuestions []string
	CurrentQIdx      int
	CollectedAnswers []string
	AnswerHistory    [][]string // For undo: history of all answer states
	AutoAnswering    bool       // Is auto-answer generating?

	// Cancellation & Confirmation
	CancelFunc      context.CancelFunc // To cancel LLM operations
	ShowQuitConfirm bool               // Show quit confirmation dialog
	ShowHelp        bool               // Show help overlay
	HasUnsavedWork  bool               // Track if user has entered any content
	LLMStartTime    time.Time          // Track LLM operation start for timeout

	// Input/Output
	Input            core.Input
	ClarifyingResult *core.Output // Last result
	PlanningResult   *core.Output
	Stream           *core.StreamingOutput // Channel for streaming events

	// Components
	Spinner   spinner.Model
	TextInput textarea.Model
	Viewport  viewport.Model

	// Dependencies
	Ctx              context.Context
	ClarifyingAgent  *planning.ClarifyingAgent
	PlanningAgent    *planning.PlanningAgent
	KnowledgeService *knowledge.Service
	Repo             *memory.Repository
}

// Messages
type MsgClarificationResult struct {
	Output      *core.Output
	ContextUsed string
	Err         error
}

type MsgContextFound struct {
	Context  string
	Strategy string // The research strategy explanation
	Err      error
}

type MsgAutoAnswerResult struct {
	Answer string
	Err    error
}

// MsgSingleAutoAnswerResult is for auto-answering one question at a time
type MsgSingleAutoAnswerResult struct {
	Answer      string
	QuestionIdx int
	Err         error
}

type MsgPlanResult struct {
	Output *core.Output
	Err    error
}

type MsgSavedPlan struct {
	ID      string
	Summary string
	Err     error
}

// MsgLLMTimeout signals LLM operation timed out
type MsgLLMTimeout struct {
	Operation string // What operation timed out
}

// MsgLLMCancelled signals LLM operation was cancelled by user
type MsgLLMCancelled struct {
	Operation string
}

// MsgCheckTimeout is sent periodically to check for LLM timeouts
type MsgCheckTimeout struct{}

func NewPlanModel(
	ctx context.Context,
	goal string,
	clarifyingAgent *planning.ClarifyingAgent,
	planningAgent *planning.PlanningAgent,
	ks *knowledge.Service,
	repo *memory.Repository,
	stream *core.StreamingOutput,
) PlanModel {
	// Styled TextArea
	ti := textarea.New()
	ti.Placeholder = "Edit the specification or [Ctrl+S] to approve..."
	ti.Focus()
	ti.CharLimit = 0 // Unlimited
	ti.SetWidth(DefaultTextareaWidth)
	ti.SetHeight(DefaultTextareaHeight)
	ti.ShowLineNumbers = false

	// Spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = StylePrimary

	// Viewport for history
	vp := viewport.New(DefaultViewportWidth, DefaultViewportHeight)

	return PlanModel{
		State:            StateInitializing,
		InitialGoal:      goal,
		EnrichedGoal:     goal, // Start same
		ThinkingStatus:   "Strategizing research & analyzing memory...",
		ClarifyingAgent:  clarifyingAgent,
		PlanningAgent:    planningAgent,
		KnowledgeService: ks,
		Repo:             repo,
		Stream:           stream,
		Ctx:              ctx,
		Spinner:          s,
		TextInput:        ti,
		Viewport:         vp,
		Msgs:             []string{StyleSubtle.Render("◆ Analyzing project memory...")},
	}
}

func (m PlanModel) Init() tea.Cmd {
	return tea.Batch(
		m.Spinner.Tick,
		m.searchContext,
	)
}

// searchContext fetches relevant KG nodes
func (m PlanModel) searchContext() tea.Msg {
	// Use shared logic for consistency with Eval system
	result, err := planning.RetrieveContext(m.Ctx, m.KnowledgeService, m.InitialGoal)
	if err != nil {
		// Even if error (unlikely as RetrieveContext handles fallbacks internally), return it
		return MsgContextFound{Context: "", Strategy: "", Err: err}
	}

	return MsgContextFound{Context: result.Context, Strategy: result.Strategy}
}

// RunClarification runs the agent in a goroutine
func runClarification(ctx context.Context, agent *planning.ClarifyingAgent, goal, history string, kgContext string) tea.Cmd {
	return func() tea.Msg {
		out, err := agent.Run(ctx, core.Input{
			ExistingContext: map[string]any{
				"goal":    goal,
				"history": history,
				"context": kgContext,
			},
			Verbose: false,
		})
		return MsgClarificationResult{Output: &out, Err: err}
	}
}

// RunPlanning runs the planning agent with streaming
func runPlanning(ctx context.Context, agent *planning.PlanningAgent, goal, enrichedGoal, kgContext string, stream *core.StreamingOutput) tea.Cmd {
	return func() tea.Msg {
		// Callback Handler
		handler := core.CreateStreamingCallbackHandler(agent.Name(), stream)
		runCtx := callbacks.InitCallbacks(ctx, &callbacks.RunInfo{Name: agent.Name()}, handler.Build())

		// Input
		input := core.Input{
			ExistingContext: map[string]any{
				"enriched_goal": enrichedGoal,
				"goal":          goal, // Fallback
				"context":       kgContext,
			},
			Verbose: false, // TUI handles output
		}

		out, err := agent.Run(runCtx, input)
		return MsgPlanResult{Output: &out, Err: err}
	}
}

// listenForStream listens for Pulse events
func listenForStream(stream *core.StreamingOutput) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-stream.Events
		if !ok {
			return nil
		}
		return event
	}
}

// SavePlan persists the plan
func savePlan(repo *memory.Repository, goal, enrichedGoal string, output *core.Output) tea.Cmd {
	return func() tea.Msg {
		// Extract tasks
		if len(output.Findings) == 0 {
			return MsgSavedPlan{Err: fmt.Errorf("no findings to save")}
		}

		finding := output.Findings[0]
		rawTasks, ok := finding.Metadata["tasks"].([]planning.PlanningTask)
		if !ok {
			return MsgSavedPlan{Err: fmt.Errorf("invalid tasks format")}
		}

		planID := "plan-" + fmt.Sprintf("%d", time.Now().Unix())
		newPlan := &task.Plan{
			ID:           planID,
			Goal:         goal,
			EnrichedGoal: enrichedGoal,
			Status:       "active",
		}

		if err := repo.CreatePlan(newPlan); err != nil {
			return MsgSavedPlan{Err: err}
		}

		taskCount := 0
		for _, tData := range rawTasks {
			newTask := &task.Task{
				PlanID:             planID,
				Title:              tData.Title,
				Description:        tData.Description,
				Priority:           tData.Priority,
				AssignedAgent:      tData.AssignedAgent,
				AcceptanceCriteria: tData.AcceptanceCriteria,
				ValidationSteps:    tData.ValidationSteps,
			}
			if err := repo.CreateTask(newTask); err == nil {
				taskCount++
			}
		}

		summary := fmt.Sprintf("Created plan %s with %d tasks", planID, taskCount)
		return MsgSavedPlan{ID: planID, Summary: summary, Err: nil}
	}
}

// runAutoAnswer triggers the agent to refine the spec with timeout
func runAutoAnswer(ctx context.Context, agent *planning.ClarifyingAgent, currentSpec string, questions []string, kgContext string) tea.Cmd {
	return func() tea.Msg {
		timeoutCtx, cancel := context.WithTimeout(ctx, LLMTimeoutSeconds*time.Second)
		defer cancel()
		ans, err := agent.AutoAnswer(timeoutCtx, currentSpec, questions, kgContext)
		if err != nil && timeoutCtx.Err() == context.DeadlineExceeded {
			return MsgLLMTimeout{Operation: "auto-answer"}
		}
		if err != nil && timeoutCtx.Err() == context.Canceled {
			return MsgLLMCancelled{Operation: "auto-answer"}
		}
		return MsgAutoAnswerResult{Answer: ans, Err: err}
	}
}

// runSingleAutoAnswer auto-answers a single question using the LLM with timeout
func runSingleAutoAnswer(ctx context.Context, agent *planning.ClarifyingAgent, question string, kgContext string, qIdx int) tea.Cmd {
	return func() tea.Msg {
		timeoutCtx, cancel := context.WithTimeout(ctx, LLMTimeoutSeconds*time.Second)
		defer cancel()
		ans, err := agent.AutoAnswer(timeoutCtx, "", []string{question}, kgContext)
		if err != nil && timeoutCtx.Err() == context.DeadlineExceeded {
			return MsgLLMTimeout{Operation: "auto-answer question"}
		}
		if err != nil && timeoutCtx.Err() == context.Canceled {
			return MsgLLMCancelled{Operation: "auto-answer question"}
		}
		return MsgSingleAutoAnswerResult{Answer: ans, QuestionIdx: qIdx, Err: err}
	}
}

// checkTimeout sends a timeout check message after a delay
func checkTimeout() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return MsgCheckTimeout{}
	})
}

func (m PlanModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Viewport.Width = msg.Width - 4 // Account for borders
		m.Viewport.Height = msg.Height - HeaderFooterHeight
		if m.Viewport.Height < MinViewportHeight {
			m.Viewport.Height = MinViewportHeight
		}
		m.TextInput.SetWidth(msg.Width - 6)
		return m, nil

	case tea.KeyMsg:
		// === GLOBAL KEY HANDLERS ===

		// Ctrl+C always quits immediately
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}

		// ? toggles help overlay (P2)
		if msg.String() == "?" && !m.ShowQuitConfirm {
			m.ShowHelp = !m.ShowHelp
			return m, nil
		}

		// If help overlay is showing, any key dismisses it
		if m.ShowHelp {
			m.ShowHelp = false
			return m, nil
		}

		// If quit confirmation is showing, handle y/n
		if m.ShowQuitConfirm {
			switch msg.String() {
			case "y", "Y":
				return m, tea.Quit
			case "n", "N", "escape":
				m.ShowQuitConfirm = false
				return m, nil
			}
			return m, nil // Ignore other keys
		}

		// Esc: Cancel LLM operation or show quit confirmation (P0)
		if msg.Type == tea.KeyEscape {
			if m.AutoAnswering {
				// Cancel ongoing LLM operation
				if m.CancelFunc != nil {
					m.CancelFunc()
				}
				m.AutoAnswering = false
				m.addMsg("SYSTEM", "Operation cancelled.")
				return m, nil
			}
			// If user has entered content, show quit confirmation
			if m.HasUnsavedWork {
				m.ShowQuitConfirm = true
				return m, nil
			}
			// Otherwise, go back one state or quit
			switch m.State {
			case StateAnsweringQuestion:
				if m.CurrentQIdx > 0 {
					// Go back to previous question
					m.CurrentQIdx--
					m.TextInput.SetValue(m.CollectedAnswers[m.CurrentQIdx])
					m.addMsg("SYSTEM", fmt.Sprintf("Back to question %d/%d", m.CurrentQIdx+1, len(m.PendingQuestions)))
				}
				return m, nil
			case StateClarifyingInput:
				m.ShowQuitConfirm = true
				return m, nil
			default:
				m.ShowQuitConfirm = true
				return m, nil
			}
		}

		// 'q' to quit from non-input states (with confirmation if work done)
		if msg.String() == "q" && m.State != StateClarifyingInput && m.State != StateAnsweringQuestion {
			if m.HasUnsavedWork {
				m.ShowQuitConfirm = true
				return m, nil
			}
			return m, tea.Quit
		}

		// === STATE-SPECIFIC KEY HANDLERS ===

		// Vim keys for viewport scrolling (P2) - only in non-input states
		if m.State != StateClarifyingInput && m.State != StateAnsweringQuestion {
			switch msg.String() {
			case "j":
				m.Viewport.ScrollDown(1)
				return m, nil
			case "k":
				m.Viewport.ScrollUp(1)
				return m, nil
			case "g":
				m.Viewport.GotoTop()
				return m, nil
			case "G":
				m.Viewport.GotoBottom()
				return m, nil
			}
		}

		// Handle one-by-one question answering
		if m.State == StateAnsweringQuestion {
			if m.AutoAnswering {
				// Only Esc can interrupt (handled above)
				return m, nil
			}

			// Tab: auto-answer current question
			if msg.Type == tea.KeyTab {
				m.AutoAnswering = true
				m.LLMStartTime = time.Now()
				m.HasUnsavedWork = true
				currentQ := m.PendingQuestions[m.CurrentQIdx]
				m.addMsg("SYSTEM", "Auto-generating answer... (Esc to cancel)")
				cmds = append(cmds, runSingleAutoAnswer(m.Ctx, m.ClarifyingAgent, currentQ, m.KGContext, m.CurrentQIdx))
				cmds = append(cmds, checkTimeout())
				return m, tea.Batch(cmds...)
			}

			// Ctrl+N or →: Skip to next question (P1)
			if msg.Type == tea.KeyCtrlN || msg.String() == "right" {
				// Save current answer (even if empty) and move forward
				m.CollectedAnswers[m.CurrentQIdx] = strings.TrimSpace(m.TextInput.Value())
				if m.CurrentQIdx < len(m.PendingQuestions)-1 {
					m.CurrentQIdx++
					m.TextInput.SetValue(m.CollectedAnswers[m.CurrentQIdx])
					m.TextInput.Focus()
				}
				return m, nil
			}

			// Ctrl+P or ←: Go to previous question (P1)
			if msg.Type == tea.KeyCtrlP || msg.String() == "left" {
				// Save current answer and move back
				m.CollectedAnswers[m.CurrentQIdx] = strings.TrimSpace(m.TextInput.Value())
				if m.CurrentQIdx > 0 {
					m.CurrentQIdx--
					m.TextInput.SetValue(m.CollectedAnswers[m.CurrentQIdx])
					m.TextInput.Focus()
				}
				return m, nil
			}

			// Ctrl+Z: Undo last answer (P2)
			if msg.Type == tea.KeyCtrlZ {
				if len(m.AnswerHistory) > 0 {
					// Restore previous state
					lastState := m.AnswerHistory[len(m.AnswerHistory)-1]
					m.AnswerHistory = m.AnswerHistory[:len(m.AnswerHistory)-1]
					copy(m.CollectedAnswers, lastState)
					m.TextInput.SetValue(m.CollectedAnswers[m.CurrentQIdx])
					m.addMsg("SYSTEM", "Undo successful.")
				}
				return m, nil
			}

			// Enter: submit answer and move to next question
			if msg.Type == tea.KeyEnter {
				answer := strings.TrimSpace(m.TextInput.Value())
				if answer == "" {
					return m, nil // Ignore empty
				}

				// Save for undo (P2)
				historyCopy := make([]string, len(m.CollectedAnswers))
				copy(historyCopy, m.CollectedAnswers)
				m.AnswerHistory = append(m.AnswerHistory, historyCopy)

				// Store answer
				m.CollectedAnswers[m.CurrentQIdx] = answer
				m.HasUnsavedWork = true
				m.addMsg("USER", answer)

				// Move to next question or finish
				m.CurrentQIdx++
				m.TextInput.Reset()

				if m.CurrentQIdx >= len(m.PendingQuestions) {
					// All questions answered, transition to final spec review
					m.State = StateClarifyingInput
					m.transitionToFinalReview()
				} else {
					m.TextInput.Placeholder = "Type your answer or press [Tab] to auto-answer..."
					m.TextInput.SetHeight(3)
					m.TextInput.Focus()
				}
				return m, nil
			}

			// Ctrl+S: Skip remaining questions and go to final review
			if msg.Type == tea.KeyCtrlS {
				// Save current answer
				m.CollectedAnswers[m.CurrentQIdx] = strings.TrimSpace(m.TextInput.Value())
				m.State = StateClarifyingInput
				m.transitionToFinalReview()
				return m, nil
			}

			m.TextInput, cmd = m.TextInput.Update(msg)
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		}

		// Pass keys to textInput if waiting for input
		if m.State == StateClarifyingInput {
			// Block input if auto-answering
			if m.AutoAnswering {
				return m, nil
			}

			// Handle Tab for Auto-Refine
			if msg.Type == tea.KeyTab {
				m.AutoAnswering = true
				m.LLMStartTime = time.Now()
				m.addMsg("SYSTEM", "Auto-generating specification... (Esc to cancel)")
				cmds = append(cmds, runAutoAnswer(m.Ctx, m.ClarifyingAgent, m.EnrichedGoal, m.PendingQuestions, m.KGContext))
				cmds = append(cmds, checkTimeout())
				return m, tea.Batch(cmds...)
			}

			// Handle Ctrl+S for submission
			if msg.Type == tea.KeyCtrlS {
				// User submitted (possibly edited) specification
				answer := m.TextInput.Value()
				if strings.TrimSpace(answer) == "" {
					return m, nil // Ignore empty
				}
				m.TextInput.Reset()
				m.TextInput.SetHeight(DefaultTextareaHeight)
				m.addMsg("USER", "Updated Specification:\n"+answer)

				// Use this AS the history for the next turn
				m.History += fmt.Sprintf("\nUser refined specification:\n%s\n", answer)
				m.HasUnsavedWork = true

				m.State = StateClarifyingThinking
				m.ThinkingStatus = "Finalizing specification..."
				m.Msgs = append(m.Msgs, StyleSubtle.Render("Refining goal..."))
				cmds = append(cmds, runClarification(m.Ctx, m.ClarifyingAgent, m.InitialGoal, m.History, m.KGContext))
				return m, tea.Batch(cmds...)
			}

			m.TextInput, cmd = m.TextInput.Update(msg)
			m.HasUnsavedWork = true // Mark as dirty when user types
			cmds = append(cmds, cmd)
		}

	case MsgAutoAnswerResult:
		m.AutoAnswering = false
		if msg.Err == nil {
			m.EnrichedGoal = msg.Answer
			m.TextInput.SetValue(msg.Answer)

			// Dynamic Resizing
			lines := strings.Count(msg.Answer, "\n") + 1
			estimatedLines := len(msg.Answer) / DefaultTextareaWidth
			if estimatedLines > lines {
				lines = estimatedLines
			}

			newHeight := lines + 2
			if newHeight < DefaultTextareaHeight {
				newHeight = DefaultTextareaHeight
			}
			if newHeight > MaxTextareaHeight {
				newHeight = MaxTextareaHeight
			}
			m.TextInput.SetHeight(newHeight)

		} else {
			m.addMsg("SYSTEM", "Auto-refine failed: "+msg.Err.Error())
			m.TextInput.SetValue(m.EnrichedGoal)
		}

	case MsgSingleAutoAnswerResult:
		m.AutoAnswering = false
		if msg.Err != nil {
			m.addMsg("WARN", formatUserFriendlyError("Auto-answer", msg.Err))
			return m, nil
		}

		// Store the auto-generated answer
		answer := strings.TrimSpace(msg.Answer)
		if answer == "" {
			answer = "(No answer generated)"
		}
		m.CollectedAnswers[msg.QuestionIdx] = answer
		m.TextInput.SetValue(answer)
		m.addMsg("ANSWER", answer)

	case MsgLLMTimeout:
		m.AutoAnswering = false
		m.addMsg("WARN", fmt.Sprintf("%s timed out after %ds. Press [Tab] to retry.", msg.Operation, LLMTimeoutSeconds))

	case MsgLLMCancelled:
		m.AutoAnswering = false
		m.addMsg("DONE", fmt.Sprintf("Operation cancelled: %s", msg.Operation))

	case MsgCheckTimeout:
		// Periodic timeout check - only add another check if still auto-answering
		if m.AutoAnswering {
			cmds = append(cmds, checkTimeout())
		}

	case spinner.TickMsg:
		if m.State != StateSuccess && m.State != StateError {
			m.Spinner, cmd = m.Spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
		// Recursive logic for state init is handled by Init() command now.
		if m.State == StateInitializing {
			// Ensure spinner is ticking
		}

	// Context Found -> Start Clarification
	case MsgContextFound:
		if msg.Err != nil {
			// Log error but proceed without context? Or fail?
			// Let's proceed without context but show error
			m.addMsg("WARN", fmt.Sprintf("Memory search failed (%v)", msg.Err))
		} else {
			m.KGContext = msg.Context
			if msg.Context != "" {
				// Display the strategy used
				if msg.Strategy != "" {
					// Use strings.TrimSpace to avoid extra newlines
					m.addMsg("STRATEGY", strings.TrimSpace(msg.Strategy))
				} else {
					m.addMsg("DONE", "Found relevant architectural context.")
				}
			} else {
				m.addMsg("THINKING", "No relevant memory found. Analyzing from scratch...")
			}
		}

		m.State = StateClarifyingThinking
		m.ThinkingStatus = "Agent is clarifying the goal..."
		cmds = append(cmds, runClarification(m.Ctx, m.ClarifyingAgent, m.InitialGoal, m.History, m.KGContext))

	// clarification Result
	case MsgClarificationResult:
		if msg.Err != nil {
			m.Err = msg.Err
			m.State = StateError
			return m, nil
		}

		// Persist the context found in the first run
		if msg.ContextUsed != "" && m.KGContext == "" {
			m.KGContext = msg.ContextUsed
			// m.addMsg("SYSTEM", "Fetched contextual memory...") // Optional feedback
		}

		// Process output
		m.ClarifyingResult = msg.Output

		finding := msg.Output.Findings[0]
		meta := finding.Metadata
		isReady, _ := meta["is_ready_to_plan"].(bool)
		enriched, _ := meta["enriched_goal"].(string)
		if enriched != "" {
			m.EnrichedGoal = enriched
		}
		// Handle questions type assertion carefully as it might be []string or []interface{}
		var questions []string
		if qs, ok := meta["questions"].([]string); ok {
			questions = qs
		} else if qs, ok := meta["questions"].([]interface{}); ok {
			for _, q := range qs {
				questions = append(questions, fmt.Sprintf("%v", q))
			}
		}

		// Enforce at least one review pass (ClarifyTurns > 0) even if agent is ready
		if (isReady && m.ClarifyTurns > 0) || m.ClarifyTurns >= 3 {
			// Ready to plan!
			m.State = StatePlanningPulse
			m.addMsg("DONE", "Goal finalized! Drafting implementation plan...")
			m.addMsg("GOAL", m.EnrichedGoal)

			// Start Planning
			// 1. Get KG context (blocking call ok here? Viewport update won't show. Assume fast enough or do async)
			// For simplicity we do synchronous search here as it's typically fast SQLite/Vector
			// But ideally we wrap this too. Let's do it inline for now.

			rankedNodes, _ := m.KnowledgeService.Search(m.Ctx, m.EnrichedGoal, 5) // Ignore err for simplicity

			var sb strings.Builder
			sb.WriteString("RELEVANT ARCHITECTURAL CONTEXT:\n")
			for _, node := range rankedNodes {
				sb.WriteString(fmt.Sprintf("%s\n%s\n\n", node.Node.Summary, node.Node.Content))
			}
			kgContext := sb.String()

			// Start Pulse listener
			cmds = append(cmds, listenForStream(m.Stream))
			// Start Planning Agent
			cmds = append(cmds, runPlanning(m.Ctx, m.PlanningAgent, m.InitialGoal, m.EnrichedGoal, kgContext, m.Stream))

		} else {
			// Spec-First Refinement
			m.ClarifyTurns++
			m.PendingQuestions = questions
			m.EnrichedGoal = enriched
			m.CurrentQIdx = 0
			m.CollectedAnswers = make([]string, len(questions))
			m.ThinkingStatus = ""

			m.addMsg("AGENT", "I've drafted a technical specification based on your goal and project context.")

			if len(m.PendingQuestions) > 0 {
				// Start one-by-one Q&A flow
				m.State = StateAnsweringQuestion
				m.addMsg("SYSTEM", fmt.Sprintf("I have %d questions to clarify. Answer each one or press [Tab] to auto-answer.", len(m.PendingQuestions)))
				// Set up textarea for answering current question
				m.TextInput.SetValue("")
				m.TextInput.Placeholder = "Type your answer or press [Tab] to auto-answer..."
				m.TextInput.SetHeight(3)
				m.TextInput.Focus()
			} else {
				// No questions, go directly to final review
				m.State = StateClarifyingInput
				m.TextInput.SetValue(m.EnrichedGoal)
				m.TextInput.Placeholder = "Review and edit the specification..."
				m.TextInput.Focus()

				// Calculate initial height based on draft
				lines := strings.Count(m.EnrichedGoal, "\n") + 1
				estimatedLines := len(m.EnrichedGoal) / DefaultTextareaWidth
				if estimatedLines > lines {
					lines = estimatedLines
				}
				newHeight := lines + 2
				if newHeight < DefaultTextareaHeight {
					newHeight = DefaultTextareaHeight
				}
				if newHeight > MaxTextareaHeight {
					newHeight = MaxTextareaHeight
				}
				m.TextInput.SetHeight(newHeight)

				m.addMsg("SYSTEM", "Please review the final specification below. Hit [Ctrl+S] to approve and start planning.")
			}
		}

	// Planning Pulse
	case core.StreamEvent:
		// Pulse Update - only process if still in planning state
		if m.State == StatePlanningPulse {
			switch msg.Type {
			case core.EventNodeStart:
				content := msg.Content // "prompt", "model", "parser"
				niceMsg := content
				if content == "prompt" {
					niceMsg = "Templating..."
				}
				if content == "model" {
					niceMsg = "Thinking..."
				}
				if content == "parser" {
					niceMsg = "Generating tasks..."
				}

				m.ThinkingStatus = niceMsg // Update status line instead of spamming log
				m.addMsg("PULSE", niceMsg)
			}
			// Keep listening only if still in planning state
			if m.State == StatePlanningPulse {
				cmds = append(cmds, listenForStream(m.Stream))
			}
		}

	// Planning Result
	case MsgPlanResult:
		if msg.Err != nil {
			m.Err = msg.Err
			m.State = StateError
			return m, nil
		}
		m.PlanningResult = msg.Output
		m.addMsg("THINKING", "Plan drafted. Saving to memory...")
		cmds = append(cmds, savePlan(m.Repo, m.InitialGoal, m.EnrichedGoal, msg.Output))

	// Saved Plan
	case MsgSavedPlan:
		if msg.Err != nil {
			m.Err = msg.Err
			m.State = StateError
		} else {
			m.State = StateSuccess
			m.PlanID = msg.ID
			m.PlanSummary = msg.Summary
			return m, tea.Quit // We exit and let the main function print the final details if needed
		}
	}

	// Sync Viewport
	m.Viewport.SetContent(strings.Join(m.Msgs, "\n"))
	// m.Viewport.GotoBottom() // REMOVED: Breaks manual scrolling. Moved to addMsg.
	m.Viewport, cmd = m.Viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *PlanModel) addMsg(msgType, content string) {
	var fullMsg string
	maxWidth := MaxMsgWidth

	switch msgType {
	// === MINIMAL PREFIXES (no brackets, just icon) ===
	case "THINKING":
		// Minimal progress - dim, no bracket noise
		fullMsg = StylePrefixThinking.Render("◆ " + content)

	case "PULSE":
		// Very subtle internal progress
		fullMsg = StyleSubtle.Render("  › " + content)

	// === BOXED MESSAGES (special visual treatment) ===
	case "STRATEGY":
		// Research strategy in a distinct box
		header := StylePrefixStrategy.Render("🔍 Research Strategy")
		// Format bullet points
		lines := strings.Split(content, "\n")
		var boxContent strings.Builder
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "•") {
				line = "› " + strings.TrimPrefix(line, "•")
				line = strings.TrimSpace(line)
			}
			if line != "" {
				boxContent.WriteString(line + "\n")
			}
		}
		boxed := StyleStrategyBox.Width(maxWidth).Render(strings.TrimSuffix(boxContent.String(), "\n"))
		fullMsg = header + "\n" + boxed

	case "ANSWER":
		// Auto-generated answer in a blue box
		header := StylePrefixAnswer.Render("💡 Suggested Answer")
		boxed := StyleAnswerBox.Width(maxWidth).Render(content)
		fullMsg = header + "\n" + boxed + "\n" + StyleSubtle.Render("[Enter] Accept  |  Edit below")

	// === ICON PREFIXES (cleaner than brackets) ===
	case "DONE":
		fullMsg = StylePrefixDone.Render("✓ " + content)

	case "WARN":
		fullMsg = StylePrefixWarn.Render("⚠ " + content)

	case "ERROR":
		fullMsg = StylePrefixError.Render("✗ " + content)

	case "AGENT":
		wrapped := StyleText.Width(maxWidth).Render(content)
		fullMsg = StylePrefixAgent.Render("◈ Agent") + " " + wrapped

	case "USER":
		wrapped := StyleText.Width(maxWidth).Render(content)
		fullMsg = StylePrefixUser.Render("› You") + " " + wrapped

	case "GOAL":
		wrapped := StylePrimary.Width(maxWidth).Render(content)
		fullMsg = StyleTitle.Render("◆ Goal") + "\n" + wrapped

	// === FALLBACK: Legacy SYSTEM messages ===
	case "SYSTEM":
		// Simple system messages without brackets
		wrapped := StyleSubtle.Width(maxWidth).Render(content)
		fullMsg = wrapped

	default:
		// Unknown type - just render as-is
		fullMsg = StyleText.Width(maxWidth).Render(content)
	}

	m.Msgs = append(m.Msgs, fullMsg)
	m.Viewport.SetContent(strings.Join(m.Msgs, "\n"))
	m.Viewport.GotoBottom()
}

// transitionToFinalReview sets up the final spec review state
func (m *PlanModel) transitionToFinalReview() {
	// Build enriched goal with Q&A
	var qaSection strings.Builder
	qaSection.WriteString("\n\n## Clarifications\n")
	for i, q := range m.PendingQuestions {
		ans := m.CollectedAnswers[i]
		if ans == "" {
			ans = "(skipped)"
		}
		qaSection.WriteString(fmt.Sprintf("**Q%d:** %s\n**A:** %s\n\n", i+1, q, ans))
	}
	m.EnrichedGoal += qaSection.String()
	m.History += qaSection.String()

	// Set up for final review
	m.TextInput.SetValue(m.EnrichedGoal)
	m.TextInput.Placeholder = "Review and edit the final specification..."
	lines := strings.Count(m.EnrichedGoal, "\n") + 1
	estimatedLines := len(m.EnrichedGoal) / DefaultTextareaWidth
	if estimatedLines > lines {
		lines = estimatedLines
	}
	newHeight := lines + 2
	if newHeight < DefaultTextareaHeight {
		newHeight = DefaultTextareaHeight
	}
	if newHeight > MaxTextareaHeight {
		newHeight = MaxTextareaHeight
	}
	m.TextInput.SetHeight(newHeight)
	m.TextInput.Focus()

	m.addMsg("SYSTEM", "Review the specification and press [Ctrl+S] to submit.")
}

// formatUserFriendlyError converts technical errors to user-friendly messages (P2)
func formatUserFriendlyError(operation string, err error) string {
	errStr := err.Error()

	// Common error patterns
	switch {
	case strings.Contains(errStr, "context deadline exceeded"):
		return fmt.Sprintf("⏱ %s timed out. Check your network or try again.", operation)
	case strings.Contains(errStr, "context canceled"):
		return fmt.Sprintf("%s was cancelled.", operation)
	case strings.Contains(errStr, "connection refused"):
		return "⚠ Cannot connect to LLM service. Is the API available?"
	case strings.Contains(errStr, "rate limit"):
		return "⚠ Rate limited by API. Wait a moment and try again."
	case strings.Contains(errStr, "401"), strings.Contains(errStr, "unauthorized"):
		return "⚠ API authentication failed. Check your API key."
	case strings.Contains(errStr, "500"), strings.Contains(errStr, "internal server"):
		return "⚠ LLM service error. Try again in a moment."
	default:
		// Truncate long errors
		if len(errStr) > 100 {
			errStr = errStr[:97] + "..."
		}
		return fmt.Sprintf("⚠ %s failed: %s", operation, errStr)
	}
}

func (m PlanModel) View() string {
	var s strings.Builder

	// === OVERLAYS (render on top of everything) ===

	// Help Overlay (P2)
	if m.ShowHelp {
		s.WriteString(StyleHeader.Render("◆ Keyboard Shortcuts") + "\n\n")
		s.WriteString(StyleTitle.Render("Global:") + "\n")
		s.WriteString("  ?         Toggle this help\n")
		s.WriteString("  Esc       Cancel operation / Go back\n")
		s.WriteString("  Ctrl+C    Quit immediately\n")
		s.WriteString("  q         Quit (with confirmation)\n\n")
		s.WriteString(StyleTitle.Render("Question Mode:") + "\n")
		s.WriteString("  Tab       Auto-answer current question\n")
		s.WriteString("  Enter     Submit answer, next question\n")
		s.WriteString("  ←/Ctrl+P  Previous question\n")
		s.WriteString("  →/Ctrl+N  Next question (skip)\n")
		s.WriteString("  Ctrl+S    Skip to final review\n")
		s.WriteString("  Ctrl+Z    Undo last answer\n\n")
		s.WriteString(StyleTitle.Render("Spec Review Mode:") + "\n")
		s.WriteString("  Tab       Auto-refine specification\n")
		s.WriteString("  Ctrl+S    Submit specification\n\n")
		s.WriteString(StyleTitle.Render("Viewport (non-input):") + "\n")
		s.WriteString("  j/k       Scroll down/up\n")
		s.WriteString("  g/G       Go to top/bottom\n\n")
		s.WriteString(StyleSubtle.Render("Press any key to close"))
		return s.String()
	}

	// Quit Confirmation (P0)
	if m.ShowQuitConfirm {
		s.WriteString("\n\n")
		s.WriteString(StyleWarning.Render("⚠ Quit and lose progress?") + "\n\n")
		s.WriteString("  [y] Yes, quit\n")
		s.WriteString("  [n] No, continue\n")
		s.WriteString("  [Esc] Cancel\n")
		return s.String()
	}

	// === NORMAL VIEW ===

	// Header (compact)
	s.WriteString(StyleHeader.Render("◆ TaskWing Planning Session"))
	goalPreview := m.InitialGoal
	if len(goalPreview) > 60 {
		goalPreview = goalPreview[:57] + "..."
	}
	s.WriteString(" " + StyleSubtle.Render("Goal: "+goalPreview) + " " + StyleSubtle.Render("[?] Help") + "\n")

	// Separator
	sepWidth := m.Viewport.Width
	if sepWidth < 40 {
		sepWidth = 40
	}
	s.WriteString(StyleSubtle.Render(strings.Repeat("─", sepWidth)) + "\n")

	// Viewport (Chat History)
	s.WriteString(m.Viewport.View() + "\n")

	// Separator before input
	s.WriteString(StyleSubtle.Render(strings.Repeat("─", sepWidth)) + "\n")

	// Status Line / Input Area
	switch m.State {
	case StateInitializing:
		s.WriteString(m.Spinner.View() + " " + m.ThinkingStatus)

	case StateAnsweringQuestion:
		// Display current question with progress
		progress := fmt.Sprintf("Question %d/%d", m.CurrentQIdx+1, len(m.PendingQuestions))
		s.WriteString(StyleWarning.Render("? "+progress) + "\n")
		currentQ := m.PendingQuestions[m.CurrentQIdx]
		// Wrap question text to fit viewport width
		qWidth := sepWidth - 4 // Account for indent
		if qWidth < 40 {
			qWidth = 40
		}
		wrappedQ := StyleText.Width(qWidth).Render(currentQ)
		// Indent each line
		for _, line := range strings.Split(wrappedQ, "\n") {
			s.WriteString("  " + line + "\n")
		}
		s.WriteString(StyleInputBox.Render(m.TextInput.View()) + "\n")
		if m.AutoAnswering {
			s.WriteString(m.Spinner.View() + StylePrimary.Render(" Generating answer... (Esc to cancel)"))
		} else {
			s.WriteString(StyleSubtle.Render("[Tab] Auto | [Enter] Submit | [←/→] Nav | [?] Help"))
		}

	case StateClarifyingInput:
		boxStyle := StyleInputBox
		stateLabel := StyleWarning.Render("✎ Editing")
		if len(m.PendingQuestions) == 0 || m.CurrentQIdx >= len(m.PendingQuestions) {
			boxStyle = StyleReadyBox
			stateLabel = StyleSuccess.Render("✓ Ready to Submit")
		}
		s.WriteString(stateLabel + "\n")
		s.WriteString(boxStyle.Render(m.TextInput.View()) + "\n")
		if m.AutoAnswering {
			s.WriteString(m.Spinner.View() + StylePrimary.Render(" Auto-generating specification..."))
		} else {
			s.WriteString(StyleSubtle.Render("[Tab] Auto-Answer | [Ctrl+S] Submit | [q] Quit"))
		}

	case StateClarifyingThinking, StatePlanningThinking, StatePlanningPulse:
		s.WriteString(m.Spinner.View() + " " + m.ThinkingStatus)

	case StateError:
		s.WriteString(StyleError.Render("✗ Error: "+m.Err.Error()) + "\n")
		s.WriteString(StyleSubtle.Render("Press [q] or [Ctrl+C] to exit"))

	case StateSuccess:
		s.WriteString(StyleSuccess.Render("✓ Plan saved. Rendering plan view..."))
	}

	s.WriteString("\n")
	return s.String()
}
