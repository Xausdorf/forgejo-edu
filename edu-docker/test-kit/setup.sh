#!/bin/bash
#
# setup.sh — Bootstrap test environment for Forgejo Edu extension
#
# Prerequisites:
#   - Forgejo running at http://localhost:3000
#   - First user (eduadmin) already registered via web UI (becomes site admin)
#
# Usage:
#   bash test-kit/setup.sh
#
# What this script does:
#   1. Creates test user accounts (teacher1, teacher2, student1-3, norole)
#   2. Creates organization test-org owned by teacher1
#   3. Creates template repositories with CI/CD workflow
#   4. Assigns edu roles via /edu/admin API
#   5. Creates a test course and enrolls students
#
set -euo pipefail

BASE_URL="${FORGEJO_URL:-http://localhost:3000}"
ADMIN_USER="${ADMIN_USER:-eduadmin}"
ADMIN_PASS="${ADMIN_PASS:-Password123!}"
DEFAULT_PASS="Password123!"

# ── helpers ──────────────────────────────────────────────────────────

log()  { echo -e "\033[1;32m[+]\033[0m $*"; }
warn() { echo -e "\033[1;33m[!]\033[0m $*"; }
err()  { echo -e "\033[1;31m[-]\033[0m $*"; }

# Get admin API token (create if not exists)
get_admin_token() {
    local token_name="edu-test-setup"

    # Try to create a new token
    local resp
    resp=$(curl -s -w "\n%{http_code}" \
        -u "${ADMIN_USER}:${ADMIN_PASS}" \
        -X POST "${BASE_URL}/api/v1/users/${ADMIN_USER}/tokens" \
        -H "Content-Type: application/json" \
        -d "{\"name\": \"${token_name}-$(date +%s)\", \"scopes\": [\"all\"]}")

    local code
    code=$(echo "$resp" | tail -1)
    local body
    body=$(echo "$resp" | head -n -1)

    if [ "$code" = "201" ]; then
        echo "$body" | python3 -c "import sys,json; print(json.load(sys.stdin)['sha1'])" 2>/dev/null \
            || echo "$body" | grep -o '"sha1":"[^"]*"' | cut -d'"' -f4
        return 0
    fi

    err "Failed to create API token (HTTP $code). Is $ADMIN_USER a site admin?"
    err "Response: $body"
    return 1
}

# API call with token
api() {
    local method="$1" path="$2"
    shift 2
    curl -s -w "\n%{http_code}" \
        -X "$method" \
        -H "Authorization: token ${TOKEN}" \
        -H "Content-Type: application/json" \
        "${BASE_URL}${path}" "$@"
}

# Create user (returns 0 if created, 1 if already exists)
create_user() {
    local username="$1" email="$2" password="${3:-$DEFAULT_PASS}"

    local resp
    resp=$(api POST "/api/v1/admin/users" \
        -d "{
            \"username\": \"${username}\",
            \"email\": \"${email}\",
            \"password\": \"${password}\",
            \"must_change_password\": false,
            \"login_name\": \"${username}\",
            \"visibility\": \"public\"
        }")

    local code
    code=$(echo "$resp" | tail -1)

    if [ "$code" = "201" ]; then
        log "Created user: $username"
        return 0
    elif [ "$code" = "422" ]; then
        warn "User $username already exists"
        return 0
    else
        err "Failed to create user $username (HTTP $code)"
        echo "$resp" | head -n -1
        return 1
    fi
}

# Create organization
create_org() {
    local orgname="$1" owner="$2"

    # Need to use the owner's token to create an org
    local resp
    resp=$(api POST "/api/v1/orgs" \
        -d "{
            \"username\": \"${orgname}\",
            \"visibility\": \"public\",
            \"repo_admin_change_team_access\": true
        }")

    local code
    code=$(echo "$resp" | tail -1)

    if [ "$code" = "201" ]; then
        log "Created org: $orgname"
        return 0
    elif [ "$code" = "422" ]; then
        warn "Org $orgname already exists"
        return 0
    else
        err "Failed to create org $orgname (HTTP $code)"
        echo "$resp" | head -n -1
        return 1
    fi
}

# Create repository in org
create_org_repo() {
    local orgname="$1" reponame="$2"

    local resp
    resp=$(api POST "/api/v1/orgs/${orgname}/repos" \
        -d "{
            \"name\": \"${reponame}\",
            \"auto_init\": true,
            \"default_branch\": \"main\",
            \"description\": \"Template repository for edu testing\"
        }")

    local code
    code=$(echo "$resp" | tail -1)

    if [ "$code" = "201" ]; then
        log "Created repo: ${orgname}/${reponame}"
        return 0
    elif [ "$code" = "409" ]; then
        warn "Repo ${orgname}/${reponame} already exists"
        return 0
    else
        err "Failed to create repo ${orgname}/${reponame} (HTTP $code)"
        echo "$resp" | head -n -1
        return 1
    fi
}

# Create personal repository
create_user_repo() {
    local reponame="$1"

    local resp
    resp=$(api POST "/api/v1/user/repos" \
        -d "{
            \"name\": \"${reponame}\",
            \"auto_init\": true,
            \"default_branch\": \"main\",
            \"description\": \"Template repository for edu testing (no CI)\"
        }")

    local code
    code=$(echo "$resp" | tail -1)

    if [ "$code" = "201" ]; then
        log "Created repo: ${ADMIN_USER}/${reponame}"
        return 0
    elif [ "$code" = "409" ]; then
        warn "Repo ${reponame} already exists"
        return 0
    else
        err "Failed to create repo $reponame (HTTP $code)"
        echo "$resp" | head -n -1
        return 1
    fi
}

# Create/update file in repo via API
create_file() {
    local owner="$1" repo="$2" filepath="$3" content_b64="$4" message="$5"

    # Check if file exists first
    local check
    check=$(api GET "/api/v1/repos/${owner}/${repo}/contents/${filepath}")
    local check_code
    check_code=$(echo "$check" | tail -1)

    if [ "$check_code" = "200" ]; then
        # File exists, get SHA and update
        local sha
        sha=$(echo "$check" | head -n -1 | python3 -c "import sys,json; print(json.load(sys.stdin)['sha'])" 2>/dev/null \
            || echo "$check" | head -n -1 | grep -o '"sha":"[^"]*"' | head -1 | cut -d'"' -f4)

        local resp
        resp=$(api PUT "/api/v1/repos/${owner}/${repo}/contents/${filepath}" \
            -d "{
                \"content\": \"${content_b64}\",
                \"message\": \"${message}\",
                \"sha\": \"${sha}\"
            }")
        local code
        code=$(echo "$resp" | tail -1)
        if [ "$code" = "200" ]; then
            log "Updated file: ${owner}/${repo}/${filepath}"
        else
            err "Failed to update file ${filepath} (HTTP $code)"
        fi
    else
        # Create new file
        local resp
        resp=$(api POST "/api/v1/repos/${owner}/${repo}/contents/${filepath}" \
            -d "{
                \"content\": \"${content_b64}\",
                \"message\": \"${message}\"
            }")
        local code
        code=$(echo "$resp" | tail -1)
        if [ "$code" = "201" ]; then
            log "Created file: ${owner}/${repo}/${filepath}"
        else
            err "Failed to create file ${filepath} (HTTP $code)"
        fi
    fi
}

# ── main ─────────────────────────────────────────────────────────────

log "Forgejo Edu Test Setup"
log "Base URL: ${BASE_URL}"
log ""

# Step 0: Get API token
log "Obtaining admin API token..."
TOKEN=$(get_admin_token)
if [ -z "$TOKEN" ]; then
    err "Could not obtain API token. Aborting."
    exit 1
fi
log "Token obtained."
echo ""

# ── Step 1: Create users ─────────────────────────────────────────────

log "=== Step 1: Creating test users ==="

create_user "teacher1" "teacher1@localhost.local"
create_user "teacher2" "teacher2@localhost.local"
create_user "student1" "student1@localhost.local"
create_user "student2" "student2@localhost.local"
create_user "student3" "student3@localhost.local"
create_user "norole"   "norole@localhost.local"
echo ""

# ── Step 2: Create org ───────────────────────────────────────────────

log "=== Step 2: Creating organization ==="

# We need teacher1's token to own the org. Use admin to create it first,
# then transfer or add teacher1 as owner.
# Simpler approach: admin creates org, adds teacher1 as owner.
create_org "test-org" "teacher1"

# Add teacher1 to the owners team
log "Adding teacher1 to test-org Owners team..."
# First, find the Owners team ID
TEAMS_RESP=$(api GET "/api/v1/orgs/test-org/teams")
TEAMS_CODE=$(echo "$TEAMS_RESP" | tail -1)
if [ "$TEAMS_CODE" = "200" ]; then
    OWNERS_TEAM_ID=$(echo "$TEAMS_RESP" | head -n -1 | python3 -c "
import sys, json
teams = json.load(sys.stdin)
for t in teams:
    if t['name'] == 'Owners':
        print(t['id'])
        break
" 2>/dev/null || echo "")

    if [ -n "$OWNERS_TEAM_ID" ]; then
        api PUT "/api/v1/teams/${OWNERS_TEAM_ID}/members/teacher1" > /dev/null 2>&1
        log "teacher1 added to Owners team (ID: $OWNERS_TEAM_ID)"
    fi
fi
echo ""

# ── Step 3: Create tasks-master repo and seed it ─────────────────────

log "=== Step 3: Creating tasks-master repository ==="

create_org_repo "test-org" "tasks-master"

# Seed tasks-master from local template-tasks-master/ folder.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEMPLATE_DIR="${SCRIPT_DIR}/template-tasks-master"

if [ ! -d "$TEMPLATE_DIR" ]; then
    err "Template folder not found: $TEMPLATE_DIR"
    exit 1
fi

# Helper: PUT/POST file to repo via API
push_file_from_disk() {
    local repo_owner="$1" repo_name="$2" rel_path="$3" abs_path="$4" message="$5"
    local content_b64
    content_b64=$(base64 -w0 < "$abs_path" 2>/dev/null || base64 < "$abs_path" | tr -d '\n')
    create_file "$repo_owner" "$repo_name" "$rel_path" "$content_b64" "$message"
}

# Walk the template directory and push each file.
log "Seeding tasks-master from $TEMPLATE_DIR ..."
while IFS= read -r -d '' f; do
    rel="${f#$TEMPLATE_DIR/}"
    rel="${rel//\\//}"
    push_file_from_disk "test-org" "tasks-master" "$rel" "$f" "Seed: $rel"
done < <(find "$TEMPLATE_DIR" -type f -print0)

# Create the submits/sample branch starting from main, so distribute can succeed for the sample assignment.
log "Creating branch submits/sample on tasks-master..."
SHA=$(api GET "/api/v1/repos/test-org/tasks-master/branches/main" | head -n -1 \
    | python3 -c "import sys,json; print(json.load(sys.stdin)['commit']['id'])" 2>/dev/null || echo "")

if [ -n "$SHA" ]; then
    api POST "/api/v1/repos/test-org/tasks-master/branches" \
        -d "{\"new_branch_name\": \"submits/sample\", \"old_branch_name\": \"main\"}" \
        > /dev/null
    log "Branch submits/sample created (from $SHA)"
else
    warn "Could not resolve main commit SHA — create submits/sample manually if needed"
fi

echo ""

# ── Step 4: Assign edu roles ─────────────────────────────────────────

log "=== Step 4: Assigning edu roles ==="
log "(Roles are assigned via the /edu/admin web UI.)"
log ""
log "Manual steps required:"
log "  1. Open ${BASE_URL}/edu/admin as eduadmin"
log "  2. Set teacher1 → teacher"
log "  3. Set teacher2 → teacher"
log "  4. Set student1 → student"
log "  5. Set student2 → student"
log "  6. Set student3 → student"
log "  7. Leave norole without a role"
log ""
log "Alternatively, use the Forgejo API to call /edu/admin/roles POST"
log "(requires CSRF token from a browser session)."
echo ""

# Try to assign roles via curl with session cookie
log "Attempting to assign roles via web session..."

# Login as admin to get session cookie
LOGIN_RESP=$(curl -s -c /tmp/edu_cookies.txt -b /tmp/edu_cookies.txt \
    -L "${BASE_URL}/user/login")

CSRF_TOKEN=$(echo "$LOGIN_RESP" | grep -o 'name="_csrf" content="[^"]*"' | head -1 | cut -d'"' -f4)
if [ -z "$CSRF_TOKEN" ]; then
    CSRF_TOKEN=$(echo "$LOGIN_RESP" | grep -o '_csrf.*value="[^"]*"' | head -1 | grep -o 'value="[^"]*"' | cut -d'"' -f2)
fi

if [ -n "$CSRF_TOKEN" ]; then
    # Perform login
    curl -s -c /tmp/edu_cookies.txt -b /tmp/edu_cookies.txt \
        -L -X POST "${BASE_URL}/user/login" \
        -d "_csrf=${CSRF_TOKEN}&user_name=${ADMIN_USER}&password=${ADMIN_PASS}" \
        -o /dev/null

    # Get fresh CSRF from admin page
    ADMIN_PAGE=$(curl -s -c /tmp/edu_cookies.txt -b /tmp/edu_cookies.txt \
        "${BASE_URL}/edu/admin")

    CSRF_TOKEN=$(echo "$ADMIN_PAGE" | grep -o 'name="_csrf" content="[^"]*"' | head -1 | cut -d'"' -f4)
    if [ -z "$CSRF_TOKEN" ]; then
        CSRF_TOKEN=$(echo "$ADMIN_PAGE" | grep -o '_csrf.*value="[^"]*"' | head -1 | grep -o 'value="[^"]*"' | cut -d'"' -f2)
    fi

    if [ -n "$CSRF_TOKEN" ]; then
        assign_role() {
            local username="$1" role="$2"
            curl -s -c /tmp/edu_cookies.txt -b /tmp/edu_cookies.txt \
                -X POST "${BASE_URL}/edu/admin/roles" \
                -d "_csrf=${CSRF_TOKEN}&username=${username}&role=${role}" \
                -o /dev/null -w "%{http_code}"
        }

        for pair in "teacher1:teacher" "teacher2:teacher" "student1:student" "student2:student" "student3:student"; do
            user="${pair%%:*}"
            role="${pair##*:}"
            code=$(assign_role "$user" "$role")
            if [ "$code" = "302" ] || [ "$code" = "200" ]; then
                log "Assigned role: $user → $role"
            else
                warn "Could not assign role for $user (HTTP $code) — do it manually"
            fi
        done
    else
        warn "Could not extract CSRF token from /edu/admin page. Assign roles manually."
    fi

    rm -f /tmp/edu_cookies.txt
else
    warn "Could not get CSRF token from login page. Assign roles manually."
fi
echo ""

# ── Step 5: Summary ──────────────────────────────────────────────────

log "=== Setup Complete ==="
log ""
log "Test accounts (password: $DEFAULT_PASS):"
log "  eduadmin  — site admin + edu admin"
log "  teacher1  — teacher, owns test-org"
log "  teacher2  — teacher (isolation testing)"
log "  student1  — student"
log "  student2  — student"
log "  student3  — student (not enrolled)"
log "  norole    — no edu role"
log ""
log "Repositories:"
log "  test-org/tasks-master  — single course repo with sample task and CI workflow"
log "                         branch submits/sample exists (so distribute will succeed)"
log ""
log "Next steps:"
log "  1. Verify roles at ${BASE_URL}/edu/admin"
log "  2. Follow TESTING_PLAN.md to run through all test scenarios"
log ""
