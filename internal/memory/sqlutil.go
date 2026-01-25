package memory

import (
	"database/sql"
	"fmt"
)

// checkRowsErr checks for errors that may have occurred during row iteration.
// This should be called after a for rows.Next() loop to catch any iteration errors
// that rows.Next() doesn't report directly (e.g., network failures mid-scan).
//
// Example usage:
//
//	for rows.Next() {
//	    // scan...
//	}
//	if err := checkRowsErr(rows); err != nil {
//	    return nil, fmt.Errorf("iterate rows: %w", err)
//	}
func checkRowsErr(rows *sql.Rows) error {
	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows iteration error: %w", err)
	}
	return nil
}
