#!/usr/bin/env bash
#
# run-eval-comparison.sh
# Benchmark TaskWing context vs no-context baseline (parallel execution)
#
# Usage:
#   ./run-eval-comparison.sh [project-path]
#
# Example:
#   ./run-eval-comparison.sh /path/to/markwise-app
#   ./run-eval-comparison.sh   # uses current directory
#

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TW="${SCRIPT_DIR}/tw"
MODELS=("gpt-5-mini-2025-08-07" "gpt-5-nano-2025-08-07")
PROJECT_PATH=""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Parse arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            -h|--help)
                echo "Usage: $0 [project-path]"
                echo ""
                echo "Runs eval comparison with and without TaskWing context."
                echo "All model evaluations run in parallel for speed."
                echo ""
                exit 0
                ;;
            *)
                PROJECT_PATH="$1"
                shift
                ;;
        esac
    done

    # Default to current directory if not specified
    PROJECT_PATH="${PROJECT_PATH:-$(pwd)}"
}

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

log_parallel() {
    echo -e "${CYAN}[PARALLEL]${NC} $1"
}

# Validate prerequisites
check_prerequisites() {
    if [[ ! -x "${TW}" ]]; then
        log_error "TaskWing CLI not found at ${TW}"
        log_info "Run: cd ${SCRIPT_DIR} && go build -o tw ."
        exit 1
    fi

    if [[ ! -d "${PROJECT_PATH}" ]]; then
        log_error "Project path does not exist: ${PROJECT_PATH}"
        exit 1
    fi

    # Initialize eval harness if missing
    if [[ ! -d "${PROJECT_PATH}/.taskwing/eval" ]]; then
        log_warn "No eval harness found. Initializing..."
        (cd "${PROJECT_PATH}" && "${TW}" eval init)
    fi

    # Validate tasks.yaml exists
    if [[ ! -f "${PROJECT_PATH}/.taskwing/eval/tasks.yaml" ]]; then
        log_error "tasks.yaml not found. Run 'tw eval init' first."
        exit 1
    fi

    # Check if bootstrap is needed for context-based eval
    if [[ ! -d "${PROJECT_PATH}/.taskwing/memory" ]]; then
        log_warn "No project memory found. Running bootstrap for context-based eval..."
        (cd "${PROJECT_PATH}" && "${TW}" bootstrap --quiet) || {
            log_warn "Bootstrap failed. Context-based evals may have limited context."
        }
    fi

    # Create logs directory
    mkdir -p "${PROJECT_PATH}/.taskwing/eval/logs"
}

# Run evaluation with context (output to log file only)
run_with_context() {
    local model="$1"
    local log_file="${PROJECT_PATH}/.taskwing/eval/logs/with-context-${model//\//-}.log"

    if (cd "${PROJECT_PATH}" && "${TW}" eval run \
        -m "${model}" \
        --label "with-taskwing" > "${log_file}" 2>&1); then
        echo "with-context:${model}:success"
    else
        echo "with-context:${model}:failed"
    fi
}

# Run evaluation without context (output to log file only)
run_without_context() {
    local model="$1"
    local log_file="${PROJECT_PATH}/.taskwing/eval/logs/no-context-${model//\//-}.log"

    if (cd "${PROJECT_PATH}" && "${TW}" eval run \
        -m "${model}" \
        --no-context \
        --label "no-context" > "${log_file}" 2>&1); then
        echo "no-context:${model}:success"
    else
        echo "no-context:${model}:failed"
    fi
}

# Run all evaluations in parallel
run_evals() {
    local -a pids=()
    local -a tmpfiles=()

    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "  Phase 1: WITH TaskWing Context (parallel)"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

    for model in "${MODELS[@]}"; do
        log_parallel "Starting: ${model}"
        local tmpfile
        tmpfile=$(mktemp)
        tmpfiles+=("${tmpfile}")
        run_with_context "${model}" > "${tmpfile}" &
        pids+=($!)
    done

    # Wait and collect results
    local failed=0
    for i in "${!pids[@]}"; do
        wait "${pids[i]}" || true
        local result
        result=$(cat "${tmpfiles[i]}")
        rm -f "${tmpfiles[i]}"

        local model="${MODELS[i]}"
        if [[ "${result}" == *":success" ]]; then
            log_success "Completed: ${model}"
        else
            log_error "Failed: ${model} (see logs)"
            ((failed++)) || true
        fi
    done

    if [[ ${failed} -gt 0 ]]; then
        log_warn "${failed} job(s) failed in Phase 1"
    fi

    pids=()
    tmpfiles=()

    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "  Phase 2: WITHOUT Context - Baseline (parallel)"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

    for model in "${MODELS[@]}"; do
        log_parallel "Starting: ${model}"
        local tmpfile
        tmpfile=$(mktemp)
        tmpfiles+=("${tmpfile}")
        run_without_context "${model}" > "${tmpfile}" &
        pids+=($!)
    done

    # Wait and collect results
    for i in "${!pids[@]}"; do
        wait "${pids[i]}" || true
        local result
        result=$(cat "${tmpfiles[i]}")
        rm -f "${tmpfiles[i]}"

        local model="${MODELS[i]}"
        if [[ "${result}" == *":success" ]]; then
            log_success "Completed: ${model}"
        else
            log_error "Failed: ${model} (see logs)"
            ((failed++)) || true
        fi
    done

    if [[ ${failed} -gt 0 ]]; then
        log_warn "${failed} total job(s) failed"
    fi
}

# Generate benchmark report
generate_report() {
    log_info "Generating benchmark report..."
    (cd "${PROJECT_PATH}" && "${TW}" eval benchmark)
}

# Main execution
main() {
    parse_args "$@"

    echo ""
    echo "╔══════════════════════════════════════════════════════════════╗"
    echo "║           TaskWing Eval Comparison                           ║"
    echo "╚══════════════════════════════════════════════════════════════╝"
    echo ""
    log_info "Project: ${PROJECT_PATH}"
    log_info "Models: ${MODELS[*]}"
    echo ""

    check_prerequisites
    run_evals

    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "  Phase 3: Benchmark Report"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    generate_report

    echo ""
    log_success "Evaluation complete!"
    log_info "Results: ${PROJECT_PATH}/.taskwing/eval/runs/"
    log_info "Logs: ${PROJECT_PATH}/.taskwing/eval/logs/"
}

main "$@"
