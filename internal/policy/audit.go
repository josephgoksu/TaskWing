package policy

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AuditStore persists policy decisions for compliance and audit trail.
// It uses the same SQLite database as the main memory store.
type AuditStore struct {
	db *sql.DB
}

// NewAuditStore creates a new audit store using an existing database connection.
func NewAuditStore(db *sql.DB) *AuditStore {
	return &AuditStore{db: db}
}

// SaveDecision persists a policy decision to the database.
// If DecisionID is empty, a new UUID will be generated.
func (s *AuditStore) SaveDecision(decision *PolicyDecision) error {
	if decision == nil {
		return fmt.Errorf("decision is nil")
	}

	// Generate decision ID if not set
	if decision.DecisionID == "" {
		decision.DecisionID = uuid.New().String()
	}

	// Set evaluated time if not set
	if decision.EvaluatedAt.IsZero() {
		decision.EvaluatedAt = time.Now().UTC()
	}

	query := `
		INSERT INTO policy_decisions (
			decision_id, policy_path, result, violations, input_json, task_id, session_id, evaluated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(query,
		decision.DecisionID,
		decision.PolicyPath,
		decision.Result,
		decision.ViolationsJSON(),
		decision.InputJSON(),
		nullString(decision.TaskID),
		nullString(decision.SessionID),
		decision.EvaluatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert policy decision: %w", err)
	}

	return nil
}

// GetDecision retrieves a policy decision by its UUID.
func (s *AuditStore) GetDecision(decisionID string) (*PolicyDecision, error) {
	query := `
		SELECT id, decision_id, policy_path, result, violations, input_json, task_id, session_id, evaluated_at
		FROM policy_decisions
		WHERE decision_id = ?
	`

	row := s.db.QueryRow(query, decisionID)
	return s.scanDecision(row)
}

// ListDecisions retrieves policy decisions with optional filters.
func (s *AuditStore) ListDecisions(opts ListDecisionsOptions) ([]*PolicyDecision, error) {
	query := `
		SELECT id, decision_id, policy_path, result, violations, input_json, task_id, session_id, evaluated_at
		FROM policy_decisions
		WHERE 1=1
	`
	args := []any{}

	if opts.TaskID != "" {
		query += " AND task_id = ?"
		args = append(args, opts.TaskID)
	}

	if opts.SessionID != "" {
		query += " AND session_id = ?"
		args = append(args, opts.SessionID)
	}

	if opts.Result != "" {
		query += " AND result = ?"
		args = append(args, opts.Result)
	}

	if !opts.Since.IsZero() {
		query += " AND evaluated_at >= ?"
		args = append(args, opts.Since.Format(time.RFC3339))
	}

	query += " ORDER BY evaluated_at DESC"

	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", opts.Limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query policy decisions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var decisions []*PolicyDecision
	for rows.Next() {
		d, err := s.scanDecisionRows(rows)
		if err != nil {
			return nil, err
		}
		decisions = append(decisions, d)
	}

	return decisions, rows.Err()
}

// CountViolations returns the number of deny decisions in a time range.
func (s *AuditStore) CountViolations(since time.Time) (int, error) {
	query := `SELECT COUNT(*) FROM policy_decisions WHERE result = 'deny' AND evaluated_at >= ?`
	var count int
	err := s.db.QueryRow(query, since.Format(time.RFC3339)).Scan(&count)
	return count, err
}

// DeleteDecision removes a policy decision by its UUID.
func (s *AuditStore) DeleteDecision(decisionID string) error {
	result, err := s.db.Exec("DELETE FROM policy_decisions WHERE decision_id = ?", decisionID)
	if err != nil {
		return fmt.Errorf("delete policy decision: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("decision not found: %s", decisionID)
	}

	return nil
}

// PruneOldDecisions removes decisions older than the specified duration.
func (s *AuditStore) PruneOldDecisions(olderThan time.Duration) (int64, error) {
	cutoff := time.Now().UTC().Add(-olderThan)
	result, err := s.db.Exec(
		"DELETE FROM policy_decisions WHERE evaluated_at < ?",
		cutoff.Format(time.RFC3339),
	)
	if err != nil {
		return 0, fmt.Errorf("prune old decisions: %w", err)
	}
	return result.RowsAffected()
}

// ListDecisionsOptions provides filtering options for ListDecisions.
type ListDecisionsOptions struct {
	TaskID    string    // Filter by task ID
	SessionID string    // Filter by session ID
	Result    string    // Filter by result ("allow" or "deny")
	Since     time.Time // Filter by evaluated_at >= since
	Limit     int       // Maximum number of results (0 = no limit)
}

// scanDecision scans a single row into a PolicyDecision.
func (s *AuditStore) scanDecision(row *sql.Row) (*PolicyDecision, error) {
	var d PolicyDecision
	var violationsJSON, inputJSON string
	var taskID, sessionID sql.NullString
	var evaluatedAtStr string

	err := row.Scan(
		&d.ID,
		&d.DecisionID,
		&d.PolicyPath,
		&d.Result,
		&violationsJSON,
		&inputJSON,
		&taskID,
		&sessionID,
		&evaluatedAtStr,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("decision not found")
	}
	if err != nil {
		return nil, fmt.Errorf("scan policy decision: %w", err)
	}

	d.Violations = ParseViolations(violationsJSON)
	if inputJSON != "" && inputJSON != "{}" {
		var input any
		if err := json.Unmarshal([]byte(inputJSON), &input); err == nil {
			d.Input = input
		}
	}
	d.TaskID = taskID.String
	d.SessionID = sessionID.String
	d.EvaluatedAt, _ = time.Parse(time.RFC3339, evaluatedAtStr)

	return &d, nil
}

// scanDecisionRows scans a row from Rows into a PolicyDecision.
func (s *AuditStore) scanDecisionRows(rows *sql.Rows) (*PolicyDecision, error) {
	var d PolicyDecision
	var violationsJSON, inputJSON string
	var taskID, sessionID sql.NullString
	var evaluatedAtStr string

	err := rows.Scan(
		&d.ID,
		&d.DecisionID,
		&d.PolicyPath,
		&d.Result,
		&violationsJSON,
		&inputJSON,
		&taskID,
		&sessionID,
		&evaluatedAtStr,
	)
	if err != nil {
		return nil, fmt.Errorf("scan policy decision: %w", err)
	}

	d.Violations = ParseViolations(violationsJSON)
	if inputJSON != "" && inputJSON != "{}" {
		var input any
		if err := json.Unmarshal([]byte(inputJSON), &input); err == nil {
			d.Input = input
		}
	}
	d.TaskID = taskID.String
	d.SessionID = sessionID.String
	d.EvaluatedAt, _ = time.Parse(time.RFC3339, evaluatedAtStr)

	return &d, nil
}

// nullString converts an empty string to sql.NullString.
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
