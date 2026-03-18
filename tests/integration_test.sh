#!/usr/bin/env bash
# Erebus API Integration Tests
# Tests each endpoint with curl against a live API instance
#
# Usage:
#   ./integration_test.sh                          # Run against production
#   API_URL=http://localhost:8080 ./integration_test.sh  # Run against local
#   ./integration_test.sh --verbose                # Show response bodies
#   ./integration_test.sh --section clients        # Run only one section
#
# Environment:
#   API_URL     — Base URL (default: https://api-erebus.nimbo.pro)
#   JWT_TOKEN   — Auth token (reads from credentials file if not set)

set -uo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
API_URL="${API_URL:-https://api-erebus.nimbo.pro}"

TOKEN_FILE="$HOME/.openclaw/workspace/.credentials/erebus_agent_token.txt"
if [[ -z "${JWT_TOKEN:-}" ]] && [[ -f "$TOKEN_FILE" ]]; then
    JWT_TOKEN="$(cat "$TOKEN_FILE" | tr -d '[:space:]')"
fi

VERBOSE=false
RUN_SECTION=""
HAS_JQ=false

if command -v jq &>/dev/null; then
    HAS_JQ=true
fi

for arg in "$@"; do
    case "$arg" in
        --verbose)    VERBOSE=true ;;
        --section)    :;; # next arg handled below
        --help|-h)
            echo "Usage: $0 [--verbose] [--section <name>]"
            echo ""
            echo "Sections: health, auth, clients, projects, tasks, ideas, members, messages, activity"
            exit 0
            ;;
        *)
            # If previous arg was --section, capture this as the section name
            if [[ "${prev_arg:-}" == "--section" ]]; then
                RUN_SECTION="$arg"
            fi
            ;;
    esac
    prev_arg="$arg"
done

# ---------------------------------------------------------------------------
# Colors & Counters
# ---------------------------------------------------------------------------
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
DIM='\033[2m'
BOLD='\033[1m'
NC='\033[0m'

PASSED=0
FAILED=0
SKIPPED=0
TOTAL_TESTS=0

# Track resources for cleanup
declare -a CREATED_IDS=()

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
section() {
    echo ""
    echo -e "${CYAN}${BOLD}--- $1 ---${NC}"
}

pass() {
    PASSED=$((PASSED + 1))
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    echo -e "  ${GREEN}[PASS]${NC} $1"
}

fail() {
    FAILED=$((FAILED + 1))
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    echo -e "  ${RED}[FAIL]${NC} $1"
    if [[ -n "${2:-}" ]]; then
        echo -e "         ${DIM}$2${NC}"
    fi
}

skip() {
    SKIPPED=$((SKIPPED + 1))
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    echo -e "  ${YELLOW}[SKIP]${NC} $1"
}

verbose() {
    if $VERBOSE; then
        echo -e "         ${DIM}$1${NC}"
    fi
}

# curl_api METHOD PATH [BODY]
# Sets: RESP_CODE, RESP_BODY
curl_api() {
    local method="$1"
    local path="$2"
    local body="${3:-}"

    local args=(
        -s -w "\n%{http_code}"
        -X "$method"
        -H "Content-Type: application/json"
        --connect-timeout 10
        --max-time 30
    )

    if [[ -n "${JWT_TOKEN:-}" ]]; then
        args+=(-H "Authorization: Bearer ${JWT_TOKEN}")
    fi

    if [[ -n "$body" ]]; then
        args+=(-d "$body")
    fi

    local raw
    raw=$(curl "${args[@]}" "${API_URL}${path}" 2>/dev/null)
    local curl_exit=$?

    if [[ $curl_exit -ne 0 ]]; then
        RESP_CODE="000"
        RESP_BODY='{"error":"curl failed with exit code '"$curl_exit"'"}'
        return 1
    fi

    RESP_CODE=$(echo "$raw" | tail -1)
    RESP_BODY=$(echo "$raw" | sed '$d')

    verbose "$method ${API_URL}${path} -> HTTP $RESP_CODE"
    if $VERBOSE && [[ -n "$RESP_BODY" ]]; then
        if $HAS_JQ; then
            verbose "$(echo "$RESP_BODY" | jq -C '.' 2>/dev/null | head -10)"
        else
            verbose "$(echo "$RESP_BODY" | head -c 300)"
        fi
    fi
}

# check_status EXPECTED TEST_NAME — returns 0 on match, 1 on mismatch
check_status() {
    local expected="$1"
    local name="$2"
    if [[ "$RESP_CODE" == "$expected" ]]; then
        pass "$name"
        return 0
    else
        fail "$name" "Expected HTTP $expected, got HTTP $RESP_CODE"
        return 1
    fi
}

# check_field FIELD EXPECTED TEST_NAME — verify a JSON field value
check_field() {
    local field="$1"
    local expected="$2"
    local name="$3"

    local actual=""
    if $HAS_JQ; then
        actual=$(echo "$RESP_BODY" | jq -r ".$field // empty" 2>/dev/null)
    else
        actual=$(echo "$RESP_BODY" | grep -o "\"$field\"[[:space:]]*:[[:space:]]*\"[^\"]*\"" | head -1 | sed 's/.*:.*"\(.*\)"/\1/')
    fi

    if [[ "$actual" == "$expected" ]]; then
        pass "$name"
        return 0
    else
        fail "$name" "Expected $field='$expected', got '$actual'"
        return 1
    fi
}

# check_has_field FIELD TEST_NAME — verify a field exists in the response
check_has_field() {
    local field="$1"
    local name="$2"

    if echo "$RESP_BODY" | grep -q "\"$field\""; then
        pass "$name"
        return 0
    else
        fail "$name" "Field '$field' not found in response"
        return 1
    fi
}

# get_field FIELD — extract field value (returns via stdout)
get_field() {
    local field="$1"
    if $HAS_JQ; then
        echo "$RESP_BODY" | jq -r ".$field // empty" 2>/dev/null
    else
        echo "$RESP_BODY" | grep -o "\"$field\"[[:space:]]*:[[:space:]]*\"[^\"]*\"" | head -1 | sed 's/.*:.*"\(.*\)"/\1/'
    fi
}

# track_for_cleanup TYPE ID
track_for_cleanup() {
    CREATED_IDS+=("$1:$2")
}

# requires_auth — returns 0 if token is set, skips otherwise
requires_auth() {
    if [[ -z "${JWT_TOKEN:-}" ]]; then
        skip "No JWT_TOKEN — auth required"
        return 1
    fi
    return 0
}

# should_run SECTION — check if this section should run
should_run() {
    if [[ -z "$RUN_SECTION" ]] || [[ "$RUN_SECTION" == "$1" ]]; then
        return 0
    fi
    return 1
}

# ---------------------------------------------------------------------------
# Test: Health & Public Endpoints
# ---------------------------------------------------------------------------
test_health() {
    should_run "health" || return 0
    section "Health & Public Endpoints"

    # Health check
    curl_api GET "/api/health"
    check_status 200 "GET /api/health"
    check_field "status" "ok" "Health status is 'ok'"
    check_has_field "database" "Health response includes database status"
    check_has_field "time" "Health response includes server time"

    # Capabilities
    curl_api GET "/api/capabilities"
    check_status 200 "GET /api/capabilities"
    check_field "service" "erebus-api" "Service name is 'erebus-api'"
    check_has_field "endpoints" "Capabilities lists endpoints"
    check_has_field "features" "Capabilities lists features"

    # Unauthenticated request to protected route should fail
    local saved_token="${JWT_TOKEN:-}"
    JWT_TOKEN=""
    curl_api GET "/api/clients"
    if [[ "$RESP_CODE" == "401" ]]; then
        pass "Protected route rejects unauthenticated request (HTTP 401)"
    elif [[ "$RESP_CODE" == "403" ]]; then
        pass "Protected route rejects unauthenticated request (HTTP 403)"
    else
        fail "Protected route should reject unauthenticated request" "Got HTTP $RESP_CODE"
    fi
    JWT_TOKEN="$saved_token"
}

# ---------------------------------------------------------------------------
# Test: Auth Endpoints
# ---------------------------------------------------------------------------
test_auth() {
    should_run "auth" || return 0
    section "Authentication"
    requires_auth || return 0

    # Get me
    curl_api GET "/api/auth/me"
    check_status 200 "GET /api/auth/me"
    check_has_field "email" "Response includes email"
    check_has_field "name" "Response includes name"

    # Store email for later tests
    USER_EMAIL=$(get_field "email")
    verbose "Authenticated as: $USER_EMAIL"

    # Refresh token
    curl_api POST "/api/auth/refresh"
    check_status 200 "POST /api/auth/refresh"
    check_has_field "token" "Refresh returns a new token"

    local new_token
    new_token=$(get_field "token")
    if [[ -n "$new_token" ]]; then
        # Use refreshed token going forward
        JWT_TOKEN="$new_token"
        pass "Using refreshed token for remaining tests"
    fi

    # Google connection status
    curl_api GET "/api/auth/google/status"
    check_status 200 "GET /api/auth/google/status"
    check_has_field "connected" "Response includes connected field"

    # Invalid token should be rejected
    local saved_token="$JWT_TOKEN"
    JWT_TOKEN="invalid.jwt.token"
    curl_api GET "/api/auth/me"
    if [[ "$RESP_CODE" == "401" ]] || [[ "$RESP_CODE" == "403" ]]; then
        pass "Invalid token is rejected"
    else
        fail "Invalid token should be rejected" "Got HTTP $RESP_CODE"
    fi
    JWT_TOKEN="$saved_token"
}

# ---------------------------------------------------------------------------
# Test: Clients CRUD
# ---------------------------------------------------------------------------
test_clients() {
    should_run "clients" || return 0
    section "Clients CRUD"
    requires_auth || return 0

    local client_id=""

    # CREATE
    curl_api POST "/api/clients" '{"name":"IntTest Client","contactEmail":"inttest@test.local","company":"IntTest Corp","phone":"+1-555-0100","status":"active"}'
    if check_status 201 "POST /api/clients — create"; then
        client_id=$(get_field "id")
        track_for_cleanup "client" "$client_id"
        check_field "name" "IntTest Client" "Created client name matches"
        check_field "status" "active" "Created client status is active"
        check_has_field "createdAt" "Created client has createdAt"
    fi

    if [[ -z "$client_id" ]]; then
        fail "Cannot continue client tests — create failed"
        return
    fi

    # Validation: missing name should fail
    curl_api POST "/api/clients" '{"contactEmail":"bad@test.local"}'
    check_status 400 "POST /api/clients — reject missing name (400)"

    # READ single
    curl_api GET "/api/clients/$client_id"
    check_status 200 "GET /api/clients/:id — read single"
    check_field "name" "IntTest Client" "Read client name matches"
    check_field "contactEmail" "inttest@test.local" "Read client email matches"

    # READ list
    curl_api GET "/api/clients"
    check_status 200 "GET /api/clients — list"
    check_has_field "data" "List response has data array"
    check_has_field "totalCount" "List response has totalCount"
    check_has_field "page" "List response has page"

    # Pagination
    curl_api GET "/api/clients?page=1&limit=5"
    check_status 200 "GET /api/clients?page=1&limit=5 — paginated list"

    # UPDATE
    curl_api PUT "/api/clients/$client_id" '{"name":"IntTest Client Updated","notes":"Updated via integration test"}'
    if check_status 200 "PUT /api/clients/:id — update"; then
        check_field "name" "IntTest Client Updated" "Updated name matches"
    fi

    # READ not found
    curl_api GET "/api/clients/nonexistent_id_12345"
    check_status 404 "GET /api/clients/:id — not found (404)"

    # Business Units
    local unit_id=""
    curl_api POST "/api/clients/$client_id/units" '{"name":"IntTest Unit","contact":"Unit Contact","email":"unit@test.local"}'
    if check_status 201 "POST /api/clients/:id/units — create unit"; then
        unit_id=$(get_field "id")
        check_field "name" "IntTest Unit" "Created unit name matches"
        check_field "clientId" "$client_id" "Unit linked to correct client"
    fi

    curl_api GET "/api/clients/$client_id/units"
    check_status 200 "GET /api/clients/:id/units — list units"

    if [[ -n "$unit_id" ]]; then
        curl_api PUT "/api/units/$unit_id" '{"name":"IntTest Unit Updated"}'
        check_status 200 "PUT /api/units/:id — update unit"

        curl_api DELETE "/api/units/$unit_id"
        check_status 200 "DELETE /api/units/:id — delete unit"
    fi

    # Validation: create unit without name
    curl_api POST "/api/clients/$client_id/units" '{"contact":"No Name"}'
    check_status 400 "POST /api/clients/:id/units — reject missing name (400)"

    # DELETE
    curl_api DELETE "/api/clients/$client_id"
    check_status 200 "DELETE /api/clients/:id — delete"

    # Verify deleted
    curl_api GET "/api/clients/$client_id"
    check_status 404 "GET /api/clients/:id — verify deleted (404)"

    # Remove from cleanup since already deleted
    local new_ids=()
    for entry in "${CREATED_IDS[@]}"; do
        if [[ "$entry" != "client:$client_id" ]]; then
            new_ids+=("$entry")
        fi
    done
    CREATED_IDS=("${new_ids[@]+"${new_ids[@]}"}")
}

# ---------------------------------------------------------------------------
# Test: Projects CRUD
# ---------------------------------------------------------------------------
test_projects() {
    should_run "projects" || return 0
    section "Projects CRUD"
    requires_auth || return 0

    local project_id=""

    # CREATE
    curl_api POST "/api/projects" '{"name":"IntTest Project","description":"Integration test project","status":"active","priority":"high","stack":"Go + React","tags":["test","integration"]}'
    if check_status 201 "POST /api/projects — create"; then
        project_id=$(get_field "id")
        track_for_cleanup "project" "$project_id"
        check_field "name" "IntTest Project" "Created project name matches"
        check_field "status" "active" "Created project status is active"
    fi

    if [[ -z "$project_id" ]]; then
        fail "Cannot continue project tests — create failed"
        return
    fi

    # Validation
    curl_api POST "/api/projects" '{"description":"No name"}'
    check_status 400 "POST /api/projects — reject missing name (400)"

    # READ single
    curl_api GET "/api/projects/$project_id"
    check_status 200 "GET /api/projects/:id — read single"
    check_has_field "project" "Response has project field"
    check_has_field "subProjects" "Response has subProjects field"

    # READ list
    curl_api GET "/api/projects"
    check_status 200 "GET /api/projects — list"
    check_has_field "data" "List response has data"
    check_has_field "totalCount" "List response has totalCount"

    # Top-level filter
    curl_api GET "/api/projects?topLevel=true"
    check_status 200 "GET /api/projects?topLevel=true — filtered list"

    # UPDATE
    curl_api PUT "/api/projects/$project_id" '{"name":"IntTest Project Updated","description":"Updated description"}'
    if check_status 200 "PUT /api/projects/:id — update"; then
        check_field "name" "IntTest Project Updated" "Updated name matches"
    fi

    # Not found
    curl_api GET "/api/projects/nonexistent_id_12345"
    check_status 404 "GET /api/projects/:id — not found (404)"

    # CONVERT TO GROUP
    curl_api POST "/api/projects/$project_id/convert-to-group"
    if check_status 200 "POST /api/projects/:id/convert-to-group"; then
        local is_group
        is_group=$(get_field "isGroup")
        if [[ "$is_group" == "true" ]]; then
            pass "Project isGroup is true after conversion"
        else
            fail "isGroup should be true" "Got '$is_group'"
        fi
    fi

    # Create sub-project and move
    local sub_id=""
    curl_api POST "/api/projects" '{"name":"IntTest Sub-Project"}'
    if check_status 201 "POST /api/projects — create sub-project"; then
        sub_id=$(get_field "id")
        track_for_cleanup "project" "$sub_id"

        curl_api POST "/api/projects/$sub_id/move-to/$project_id"
        if check_status 200 "POST /api/projects/:id/move-to/:groupId"; then
            check_field "parentId" "$project_id" "Sub-project parentId matches group"
        fi

        # List sub-projects
        curl_api GET "/api/projects/$project_id/sub-projects"
        check_status 200 "GET /api/projects/:id/sub-projects"

        # Make standalone
        curl_api POST "/api/projects/$sub_id/make-standalone"
        check_status 200 "POST /api/projects/:id/make-standalone"

        # Delete sub-project
        curl_api DELETE "/api/projects/$sub_id"
        check_status 200 "DELETE /api/projects/:id — delete sub-project"
        # Remove from cleanup
        local new_ids=()
        for entry in "${CREATED_IDS[@]}"; do
            if [[ "$entry" != "project:$sub_id" ]]; then
                new_ids+=("$entry")
            fi
        done
        CREATED_IDS=("${new_ids[@]+"${new_ids[@]}"}")
    fi

    # DELETE project
    curl_api DELETE "/api/projects/$project_id"
    check_status 200 "DELETE /api/projects/:id — delete"

    curl_api GET "/api/projects/$project_id"
    check_status 404 "GET /api/projects/:id — verify deleted (404)"

    local new_ids=()
    for entry in "${CREATED_IDS[@]}"; do
        if [[ "$entry" != "project:$project_id" ]]; then
            new_ids+=("$entry")
        fi
    done
    CREATED_IDS=("${new_ids[@]+"${new_ids[@]}"}")
}

# ---------------------------------------------------------------------------
# Test: Tasks CRUD
# ---------------------------------------------------------------------------
test_tasks() {
    should_run "tasks" || return 0
    section "Tasks CRUD"
    requires_auth || return 0

    # Create a project for tasks
    local project_id=""
    curl_api POST "/api/projects" '{"name":"IntTest Task Project"}'
    if check_status 201 "POST /api/projects — create project for tasks"; then
        project_id=$(get_field "id")
        track_for_cleanup "project" "$project_id"
    else
        fail "Cannot continue task tests — project create failed"
        return
    fi

    # CREATE task under project
    local task_id=""
    curl_api POST "/api/projects/$project_id/tasks" '{"title":"IntTest Task","description":"Task for integration test","priority":"high"}'
    if check_status 201 "POST /api/projects/:id/tasks — create task"; then
        task_id=$(get_field "id")
        track_for_cleanup "task" "$task_id"
        check_field "title" "IntTest Task" "Created task title matches"
        check_field "status" "todo" "Default task status is 'todo'"
        check_field "projectId" "$project_id" "Task linked to correct project"
    fi

    if [[ -z "$task_id" ]]; then
        fail "Cannot continue task tests — create failed"
        return
    fi

    # Validation
    curl_api POST "/api/projects/$project_id/tasks" '{"description":"No title"}'
    check_status 400 "POST /api/projects/:id/tasks — reject missing title (400)"

    # CREATE via flat endpoint
    local flat_task_id=""
    curl_api POST "/api/tasks" "{\"title\":\"IntTest Flat Task\",\"projectId\":\"$project_id\",\"priority\":\"low\"}"
    if check_status 201 "POST /api/tasks — create task (flat endpoint)"; then
        flat_task_id=$(get_field "id")
        track_for_cleanup "task" "$flat_task_id"
        check_field "title" "IntTest Flat Task" "Flat task title matches"
    fi

    # READ single
    curl_api GET "/api/tasks/$task_id"
    check_status 200 "GET /api/tasks/:id — read single"
    check_field "title" "IntTest Task" "Read task title matches"

    # READ list (project-scoped)
    curl_api GET "/api/projects/$project_id/tasks"
    check_status 200 "GET /api/projects/:id/tasks — list project tasks"
    check_has_field "data" "Project task list has data"

    # READ list (global)
    curl_api GET "/api/tasks"
    check_status 200 "GET /api/tasks — list all tasks"
    check_has_field "data" "Global task list has data"
    check_has_field "total" "Global task list has total"

    # Global with projectId filter
    curl_api GET "/api/tasks?projectId=$project_id"
    check_status 200 "GET /api/tasks?projectId=... — filtered list"

    # Not found
    curl_api GET "/api/tasks/nonexistent_id_12345"
    check_status 404 "GET /api/tasks/:id — not found (404)"

    # UPDATE
    curl_api PUT "/api/tasks/$task_id" '{"status":"in_progress","description":"Updated by integration test"}'
    if check_status 200 "PUT /api/tasks/:id — update status"; then
        check_field "status" "in_progress" "Task status updated to in_progress"
    fi

    # CLAIM
    curl_api POST "/api/tasks/$task_id/claim"
    if check_status 200 "POST /api/tasks/:id/claim — claim task"; then
        check_has_field "claimedBy" "Task has claimedBy after claim"
        local claimed_status
        claimed_status=$(get_field "status")
        if [[ "$claimed_status" == "in_progress" ]]; then
            pass "Task status is in_progress after claim"
        else
            fail "Task status after claim" "Expected in_progress, got '$claimed_status'"
        fi
    fi

    # DELETE tasks
    curl_api DELETE "/api/tasks/$task_id"
    check_status 200 "DELETE /api/tasks/:id — delete task"

    curl_api GET "/api/tasks/$task_id"
    check_status 404 "GET /api/tasks/:id — verify deleted (404)"

    if [[ -n "$flat_task_id" ]]; then
        curl_api DELETE "/api/tasks/$flat_task_id"
        check_status 200 "DELETE /api/tasks/:id — delete flat task"
    fi

    # Clean up project
    curl_api DELETE "/api/projects/$project_id"
    check_status 200 "DELETE /api/projects/:id — clean up task project"

    # Remove from cleanup tracking
    local new_ids=()
    for entry in "${CREATED_IDS[@]}"; do
        case "$entry" in
            "task:$task_id"|"task:$flat_task_id"|"project:$project_id") ;;
            *) new_ids+=("$entry") ;;
        esac
    done
    CREATED_IDS=("${new_ids[@]+"${new_ids[@]}"}")
}

# ---------------------------------------------------------------------------
# Test: Ideas CRUD
# ---------------------------------------------------------------------------
test_ideas() {
    should_run "ideas" || return 0
    section "Ideas CRUD"
    requires_auth || return 0

    local idea_id=""

    # CREATE
    curl_api POST "/api/ideas" '{"title":"IntTest Idea","description":"An idea from integration testing","category":"testing","priority":"medium","tags":["integration","test"]}'
    if check_status 201 "POST /api/ideas — create"; then
        idea_id=$(get_field "id")
        track_for_cleanup "idea" "$idea_id"
        check_field "title" "IntTest Idea" "Created idea title matches"
        check_field "status" "new" "Default idea status is 'new'"
    fi

    if [[ -z "$idea_id" ]]; then
        fail "Cannot continue idea tests — create failed"
        return
    fi

    # Validation
    curl_api POST "/api/ideas" '{"description":"No title"}'
    check_status 400 "POST /api/ideas — reject missing title (400)"

    # READ single
    curl_api GET "/api/ideas/$idea_id"
    check_status 200 "GET /api/ideas/:id — read single"
    check_field "title" "IntTest Idea" "Read idea title matches"

    # READ list
    curl_api GET "/api/ideas"
    check_status 200 "GET /api/ideas — list"
    check_has_field "data" "Ideas list has data"
    check_has_field "totalCount" "Ideas list has totalCount"

    # Filter by status
    curl_api GET "/api/ideas?status=new"
    check_status 200 "GET /api/ideas?status=new — filtered list"

    # UPDATE
    curl_api PUT "/api/ideas/$idea_id" '{"status":"researching","description":"Updated via integration test"}'
    if check_status 200 "PUT /api/ideas/:id — update"; then
        check_field "status" "researching" "Idea status updated to researching"
    fi

    # ADD RESEARCH
    curl_api POST "/api/ideas/$idea_id/research" '{"type":"note","title":"IntTest Research","content":"Research content from integration test","source":"integration-test"}'
    if check_status 201 "POST /api/ideas/:id/research — add research"; then
        check_has_field "content" "Research entry has content"
        check_has_field "timestamp" "Research entry has timestamp"
    fi

    # Validation for research
    curl_api POST "/api/ideas/$idea_id/research" '{"title":"No content"}'
    check_status 400 "POST /api/ideas/:id/research — reject missing content (400)"

    # Verify research persisted
    curl_api GET "/api/ideas/$idea_id"
    if check_status 200 "GET /api/ideas/:id — verify research persisted"; then
        if echo "$RESP_BODY" | grep -q "IntTest Research"; then
            pass "Research entry found on idea"
        else
            fail "Research entry not found on idea"
        fi
    fi

    # CONVERT TO PROJECT
    curl_api POST "/api/ideas/$idea_id/convert-to-project"
    if check_status 201 "POST /api/ideas/:id/convert-to-project — convert"; then
        local converted_project_id
        if $HAS_JQ; then
            converted_project_id=$(echo "$RESP_BODY" | jq -r '.project.id // empty' 2>/dev/null)
        else
            converted_project_id=""
        fi
        if [[ -n "$converted_project_id" ]]; then
            track_for_cleanup "project" "$converted_project_id"
            pass "Idea converted to project: $converted_project_id"
        fi
    fi

    # Verify idea status changed to converted
    curl_api GET "/api/ideas/$idea_id"
    if check_status 200 "GET /api/ideas/:id — verify conversion status"; then
        check_field "status" "converted" "Idea status is 'converted'"
    fi

    # DELETE
    curl_api DELETE "/api/ideas/$idea_id"
    check_status 200 "DELETE /api/ideas/:id — delete"

    curl_api GET "/api/ideas/$idea_id"
    check_status 404 "GET /api/ideas/:id — verify deleted (404)"

    # Remove from cleanup
    local new_ids=()
    for entry in "${CREATED_IDS[@]}"; do
        if [[ "$entry" != "idea:$idea_id" ]]; then
            new_ids+=("$entry")
        fi
    done
    CREATED_IDS=("${new_ids[@]+"${new_ids[@]}"}")
}

# ---------------------------------------------------------------------------
# Test: Members
# ---------------------------------------------------------------------------
test_members() {
    should_run "members" || return 0
    section "Members"
    requires_auth || return 0

    # List users
    curl_api GET "/api/users"
    check_status 200 "GET /api/users — list users"
    check_has_field "data" "Users response has data array"

    # List agents
    curl_api GET "/api/agents"
    check_status 200 "GET /api/agents — list agents"
    check_has_field "data" "Agents response has data array"

    # List all members (unified)
    curl_api GET "/api/members"
    check_status 200 "GET /api/members — list all members"
    check_has_field "data" "Members response has data array"

    # Verify member objects have expected fields
    if $HAS_JQ; then
        local first_member
        first_member=$(echo "$RESP_BODY" | jq -r '.data[0].type // empty' 2>/dev/null)
        if [[ "$first_member" == "user" ]] || [[ "$first_member" == "agent" ]]; then
            pass "Member objects have type field (user/agent)"
        elif [[ -z "$first_member" ]]; then
            skip "No members returned to verify structure"
        else
            fail "Unexpected member type: $first_member"
        fi
    fi
}

# ---------------------------------------------------------------------------
# Test: Messages
# ---------------------------------------------------------------------------
test_messages() {
    should_run "messages" || return 0
    section "Messages"
    requires_auth || return 0

    # Get current user email
    curl_api GET "/api/auth/me"
    local my_email
    my_email=$(get_field "email")

    if [[ -z "$my_email" ]]; then
        skip "Could not determine user email for message tests"
        return
    fi

    local msg_id=""

    # SEND message (to self)
    local ts
    ts=$(date +%s)
    curl_api POST "/api/messages" "{\"toId\":\"${my_email}\",\"content\":\"IntTest message $ts\"}"
    if check_status 201 "POST /api/messages — send message"; then
        msg_id=$(get_field "id")
        track_for_cleanup "message" "$msg_id"
        check_field "toId" "$my_email" "Message toId matches"
        check_has_field "fromId" "Message has fromId"
        check_has_field "createdAt" "Message has createdAt"
    fi

    # Validation
    curl_api POST "/api/messages" '{"toId":"someone@test.local"}'
    check_status 400 "POST /api/messages — reject missing content (400)"

    curl_api POST "/api/messages" '{"content":"No recipient"}'
    check_status 400 "POST /api/messages — reject missing toId (400)"

    # LIST conversations
    curl_api GET "/api/messages/conversations"
    check_status 200 "GET /api/messages/conversations — list"
    check_has_field "data" "Conversations response has data"

    # GET unread count
    curl_api GET "/api/messages/unread"
    check_status 200 "GET /api/messages/unread — count"
    check_has_field "count" "Unread response has count"

    # GET conversation with self
    curl_api GET "/api/messages?with=${my_email}"
    check_status 200 "GET /api/messages?with=... — get conversation"
    check_has_field "data" "Conversation response has data"

    # Validation: missing with param
    curl_api GET "/api/messages"
    check_status 400 "GET /api/messages — reject missing 'with' param (400)"

    # MARK READ
    if [[ -n "$msg_id" ]]; then
        curl_api PUT "/api/messages/$msg_id/read"
        check_status 200 "PUT /api/messages/:id/read — mark as read"
    fi

    # DELETE conversation
    curl_api DELETE "/api/messages/conversation?with=${my_email}"
    if check_status 200 "DELETE /api/messages/conversation — delete conversation"; then
        check_has_field "deleted" "Delete response has deleted count"
    fi

    # Remove from cleanup (already deleted)
    local new_ids=()
    for entry in "${CREATED_IDS[@]}"; do
        if [[ ! "$entry" =~ ^message: ]]; then
            new_ids+=("$entry")
        fi
    done
    CREATED_IDS=("${new_ids[@]+"${new_ids[@]}"}")
}

# ---------------------------------------------------------------------------
# Test: Activity & Knowledge Search
# ---------------------------------------------------------------------------
test_activity() {
    should_run "activity" || return 0
    section "Activity & Knowledge Search"
    requires_auth || return 0

    # Activity feed
    curl_api GET "/api/activity"
    check_status 200 "GET /api/activity — get activity feed"
    check_has_field "data" "Activity response has data"

    # Pagination
    curl_api GET "/api/activity?page=1&limit=5"
    check_status 200 "GET /api/activity?page=1&limit=5 — paginated"

    # Knowledge search
    curl_api POST "/api/knowledge/search" '{"query":"test"}'
    check_status 200 "POST /api/knowledge/search — search"
    check_has_field "results" "Search response has results"
    check_has_field "query" "Search response echoes query"

    # Validation
    curl_api POST "/api/knowledge/search" '{"query":""}'
    check_status 400 "POST /api/knowledge/search — reject empty query (400)"
}

# ---------------------------------------------------------------------------
# Cleanup
# ---------------------------------------------------------------------------
do_cleanup() {
    if [[ ${#CREATED_IDS[@]} -eq 0 ]]; then
        return
    fi

    section "Cleanup"

    # Process in reverse order
    for ((i=${#CREATED_IDS[@]}-1; i>=0; i--)); do
        local entry="${CREATED_IDS[$i]}"
        local type="${entry%%:*}"
        local id="${entry#*:}"

        case "$type" in
            client)  curl_api DELETE "/api/clients/$id" ;;
            project) curl_api DELETE "/api/projects/$id" ;;
            task)    curl_api DELETE "/api/tasks/$id" ;;
            idea)    curl_api DELETE "/api/ideas/$id" ;;
            message) ;; # messages cleaned up via conversation delete
        esac

        if [[ "$RESP_CODE" == "200" ]] || [[ "$RESP_CODE" == "404" ]]; then
            echo -e "  ${GREEN}Cleaned${NC} $type: ${DIM}$id${NC}"
        else
            echo -e "  ${YELLOW}Warning${NC} Failed to clean $type $id (HTTP $RESP_CODE)"
        fi
    done
}

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
print_summary() {
    echo ""
    echo -e "${BOLD}========================================${NC}"
    echo -e "${BOLD}  Erebus API Integration Test Results${NC}"
    echo -e "${BOLD}========================================${NC}"
    echo -e "  ${GREEN}Passed:${NC}  $PASSED"
    echo -e "  ${RED}Failed:${NC}  $FAILED"
    echo -e "  ${YELLOW}Skipped:${NC} $SKIPPED"
    echo -e "  Total:   $TOTAL_TESTS"
    echo -e "  Target:  ${DIM}$API_URL${NC}"
    echo -e "${BOLD}========================================${NC}"

    if [[ $FAILED -gt 0 ]]; then
        echo -e "  ${RED}${BOLD}RESULT: FAIL${NC}"
    elif [[ $PASSED -gt 0 ]]; then
        echo -e "  ${GREEN}${BOLD}RESULT: PASS${NC}"
    else
        echo -e "  ${YELLOW}${BOLD}RESULT: ALL SKIPPED${NC}"
    fi
    echo ""
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
    echo -e "${BOLD}Erebus API Integration Tests${NC}"
    echo -e "  Target:  ${CYAN}$API_URL${NC}"
    echo -e "  Token:   ${CYAN}${JWT_TOKEN:+set (${#JWT_TOKEN} chars)}${JWT_TOKEN:-NOT SET}${NC}"
    echo -e "  jq:      ${CYAN}${HAS_JQ}${NC}"
    echo -e "  Date:    $(date -u '+%Y-%m-%d %H:%M:%S UTC')"
    if [[ -n "$RUN_SECTION" ]]; then
        echo -e "  Section: ${CYAN}$RUN_SECTION${NC}"
    fi

    test_health
    test_auth
    test_clients
    test_projects
    test_tasks
    test_ideas
    test_members
    test_messages
    test_activity

    do_cleanup
    print_summary

    if [[ $FAILED -gt 0 ]]; then
        exit 1
    fi
    exit 0
}

main
