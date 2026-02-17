# Prompt Failures Log

Track prompt-contract failures and close the loop with monthly fixes.

## Entry Format

| Date (UTC) | Command | Failure Type | User Impact | Root Cause | Fix Shipped | Release |
|------------|---------|--------------|-------------|------------|-------------|---------|
| YYYY-MM-DD | /tw-*   | gate bypass / sequencing / ambiguity / missing refusal | low/med/high | short summary | yes/no + PR | vX.Y.Z |

## Failure Taxonomy

- `gate_bypass`: assistant skipped a hard gate.
- `sequence_break`: steps executed out of contract order.
- `weak_refusal`: gate failure detected but refusal was missing or vague.
- `evidence_gap`: completion/debug output lacked required proof.
- `prompt_drift`: generated assistant command content diverged from canonical contract.

## Monthly Review Checklist

1. Aggregate all failures from the month.
2. Rank by repeated occurrence and user impact.
3. Ship fixes to slash content and tests.
4. Add changelog note referencing resolved failure classes.
5. Re-check first-run activation loop completion rate.

