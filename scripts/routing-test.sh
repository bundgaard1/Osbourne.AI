#!/usr/bin/env bash
# Verifies which backend each /api route is proxied to, using stub upstreams
# instead of the real services.
#
# The regex locations in nginx.conf are order-sensitive - nginx tries them top to
# bottom and takes the first match, unlike prefix locations which take the
# longest match. So /api/courses/42/modules only reaches course-content because
# that location is listed before the catalogue's. A reordering that still looks
# correct by eye is exactly the regression this catches, and it is not something
# `nginx -t` can see.
#
# Each stub is an nginx that returns its own name plus the request headers it
# received, which lets the same harness assert both the routing table and the
# cookie -> Authorization promotion.
#
# Usage: ./scripts/routing-test.sh
# Requires: docker. Leaves nothing behind.

set -euo pipefail

cd "$(dirname "$0")/.."

NET="osbourne-routing-test-$$"
GATEWAY_PORT="${GATEWAY_PORT:-18080}"
PASS=0
FAIL=0
CONTAINERS=()

cleanup() {
    for c in "${CONTAINERS[@]:-}"; do
        [ -n "$c" ] && docker rm -f "$c" >/dev/null 2>&1 || true
    done
    docker network rm "$NET" >/dev/null 2>&1 || true
}
trap cleanup EXIT

start_stub() {
    local name="$1"
    local conf
    conf=$(mktemp)
    # The stub reports which upstream it is and what it was sent, so the
    # assertions can check the header translation as well as the route.
    cat >"$conf" <<EOF
events {}
http {
    server {
        listen 8080;
        # The stub is an nginx too, so it would apply the 1 MB default and
        # answer 413 itself, making it look like the gateway was rejecting the
        # upload. Unbounded here leaves the gateway's limit as the only one.
        client_max_body_size 0;
        location / {
            default_type application/json;
            return 200 '{"upstream":"$name","path":"\$uri","query":"\$args","authorization":"\$http_authorization","cookie":"\$http_cookie","request_id":"\$http_x_request_id","body_buffering":"\$request_body"}';
        }
    }
}
EOF
    docker run -d --name "stub-$name-$NET" --network "$NET" --network-alias "$name" \
        -v "$conf:/etc/nginx/nginx.conf:ro" nginx:alpine >/dev/null
    CONTAINERS+=("stub-$name-$NET")
    rm -f "$conf"
}

echo "==> creating network"
docker network create "$NET" >/dev/null

echo "==> starting stubs"
for svc in auth-service profile-service notification-service course-content-service \
           course-catalogue-service assignment-service frontend; do
    start_stub "$svc"
done

echo "==> starting gateway"
docker run -d --name "gateway-$NET" --network "$NET" \
    -p "127.0.0.1:${GATEWAY_PORT}:80" \
    -v "$PWD/nginx.conf:/etc/nginx/nginx.conf:ro" \
    -v "$PWD/nginx/includes:/etc/nginx/includes:ro" \
    nginx:alpine >/dev/null
CONTAINERS+=("gateway-$NET")

# Give the container DNS entries a moment; the gateway resolves lazily per
# request so it will start fine either way, but the first curls would 502.
for _ in $(seq 1 30); do
    if curl -fsS "http://127.0.0.1:${GATEWAY_PORT}/" >/dev/null 2>&1; then break; fi
    sleep 0.5
done

# json_field <json> <key> - extracts a top-level string field without jq.
json_field() {
    printf '%s' "$1" | sed -n "s/.*\"$2\":\"\([^\"]*\)\".*/\1/p"
}

# Same, reading the document from stdin, for use mid-pipeline.
json_field_stdin() {
    sed -n "s/.*\"$1\":\"\([^\"]*\)\".*/\1/p"
}

assert_route() {
    local path="$1" want="$2" desc="${3:-}"
    local body
    body=$(curl -sS "http://127.0.0.1:${GATEWAY_PORT}${path}" || true)
    local got
    got=$(json_field "$body" upstream)

    if [ "$got" = "$want" ]; then
        PASS=$((PASS + 1))
        printf '  ok   %-52s -> %s\n' "$path" "$want"
    else
        FAIL=$((FAIL + 1))
        printf '  FAIL %-52s -> %s (want %s)\n' "$path" "${got:-<no body: $body>}" "$want" $desc
    fi
}

assert_status() {
    local path="$1" want="$2"
    local got
    got=$(curl -sS -o /dev/null -w '%{http_code}' "http://127.0.0.1:${GATEWAY_PORT}${path}" || true)
    if [ "$got" = "$want" ]; then
        PASS=$((PASS + 1))
        printf '  ok   %-52s -> HTTP %s\n' "$path" "$want"
    else
        FAIL=$((FAIL + 1))
        printf '  FAIL %-52s -> HTTP %s (want %s)\n' "$path" "$got" "$want"
    fi
}

assert_header() {
    local path="$1" header="$2" want="$3" desc="$4"
    shift 4 2>/dev/null || true
    local body got
    body=$(curl -sS "$@" "http://127.0.0.1:${GATEWAY_PORT}${path}" || true)
    got=$(json_field "$body" "$header")
    if [ "$got" = "$want" ]; then
        PASS=$((PASS + 1))
        printf '  ok   %-52s %s = %s\n' "$path" "$header" "$want"
    else
        FAIL=$((FAIL + 1))
        printf '  FAIL %-52s %s = %q (want %q) %s\n' "$path" "$header" "$got" "$want" "$desc"
    fi
}

echo
echo "==> routing table"

# auth
assert_route "/api/auth/login"        auth-service
assert_route "/api/auth/validate"     auth-service
assert_route "/api/auth/logout"       auth-service

# profile
assert_route "/api/profile"           profile-service

# notification
assert_route "/api/notifications"     notification-service
assert_route "/api/notifications/n1/read" notification-service

# course content - the sub-resource that has to beat the catalogue
assert_route "/api/courses/42/modules"     course-content-service "content before catalogue"
assert_route "/api/courses/42/modules/m1"  course-content-service
assert_route "/api/courses/abc-def/modules" course-content-service "hyphenated id"

# assignment under a course - beats the catalogue too
assert_route "/api/courses/42/assignments" assignment-service "assignments before catalogue"

# assignment by id
assert_route "/api/assignments/a1"                  assignment-service
assert_route "/api/assignments/a1/submissions"      assignment-service
assert_route "/api/assignments/a1/submissions/mine" assignment-service
assert_route "/api/submissions/s1"                  assignment-service
assert_route "/api/submissions/s1/grade"            assignment-service

# the two hand-written streaming routes
assert_route "/api/assignments/a1/submissions" assignment-service "upload route"
assert_route "/api/submissions/s1/file"        assignment-service "download route"

# catalogue owns the bare course routes and enrollments
assert_route "/api/courses"        course-catalogue-service
assert_route "/api/courses/42"     course-catalogue-service "course detail is the catalogue's"
assert_route "/api/enrollments"    course-catalogue-service
assert_route "/api/enrollments/me" course-catalogue-service

# frontend and the catch-all
assert_route "/"          frontend
assert_route "/login"     frontend
assert_route "/static/x"  frontend
assert_status "/api/"          404
assert_status "/api/nope"      404
assert_route "/api/courses/"    course-catalogue-service "an empty id is the catalogue's to reject"

echo
echo "==> catch-all body"

body=$(curl -sS "http://127.0.0.1:${GATEWAY_PORT}/api/nope" || true)
if [ "$body" = '{"code":404,"success":false,"message":"unknown API route"}' ]; then
    PASS=$((PASS + 1))
    echo "  ok   catch-all returns the gateway error shape"
else
    FAIL=$((FAIL + 1))
    echo "  FAIL catch-all body = $body"
fi

ctype=$(curl -sS -o /dev/null -w '%{content_type}' "http://127.0.0.1:${GATEWAY_PORT}/api/nope" || true)
if [ "$ctype" = "application/json" ]; then
    PASS=$((PASS + 1))
    echo "  ok   catch-all Content-Type is application/json"
else
    FAIL=$((FAIL + 1))
    echo "  FAIL catch-all Content-Type = $ctype"
fi

echo
echo "==> query string is preserved"

# The catalogue paginates on page/page_size, so a dropped query string would
# silently return the default page rather than failing.
body=$(curl -sS "http://127.0.0.1:${GATEWAY_PORT}/api/courses?page=2&page_size=10" || true)
if [ "$(json_field "$body" query)" = "page=2&page_size=10" ]; then
    PASS=$((PASS + 1))
    echo "  ok   /api/courses?page=2&page_size=10 keeps its query string"
else
    FAIL=$((FAIL + 1))
    echo "  FAIL query string lost: $body"
fi

echo
echo "==> session cookie -> Authorization"

# The browser holds the JWT in an HttpOnly cookie; services read a bearer header.
assert_header "/api/profile" authorization "Bearer jwt-from-cookie" \
    "cookie must be promoted to a bearer header" \
    -H 'Cookie: osbourne_session=jwt-from-cookie'
assert_header "/api/profile" cookie "" \
    "the raw cookie must not reach the service" \
    -H 'Cookie: osbourne_session=jwt-from-cookie'

# A browser that has never logged in still sends the request, so the services
# must see an empty bearer and answer 401 themselves rather than inheriting a
# stale token from somewhere. The trailing space after "Bearer" is trimmed
# before comparing, since that is what survives a shell substitution.
empty=$(curl -sS "http://127.0.0.1:${GATEWAY_PORT}/api/profile" \
    | json_field_stdin authorization | tr -d '[:space:]')
if [ "$empty" = "Bearer" ]; then
    PASS=$((PASS + 1))
    echo "  ok   no credentials produce an empty bearer token"
else
    FAIL=$((FAIL + 1))
    echo "  FAIL no credentials produced '$empty' (want an empty token)"
fi

# curl and Postman set the header directly, so it has to win over the cookie.
got=$(curl -sS -H 'Cookie: osbourne_session=from-cookie' \
             -H 'Authorization: Bearer explicit-header' \
             "http://127.0.0.1:${GATEWAY_PORT}/api/profile" \
      | sed -n 's/.*"authorization":"\([^"]*\)".*/\1/p')
if [ "$got" = "Bearer explicit-header" ]; then
    PASS=$((PASS + 1))
    echo "  ok   an explicit Authorization header wins over the cookie"
else
    FAIL=$((FAIL + 1))
    echo "  FAIL explicit header = $got (want Bearer explicit-header)"
fi

# The frontend does read the cookie directly, so it must still arrive there.
fcookie=$(curl -sS -H 'Cookie: osbourne_session=abc' \
    "http://127.0.0.1:${GATEWAY_PORT}/login" \
    | sed -n 's/.*"cookie":"\([^"]*\)".*/\1/p')
if [ "$fcookie" = "osbourne_session=abc" ]; then
    PASS=$((PASS + 1))
    echo "  ok   the frontend still receives the Cookie header"
else
    FAIL=$((FAIL + 1))
    echo "  FAIL frontend cookie = $fcookie (want osbourne_session=abc)"
fi

echo
echo "==> request id is propagated"

back=$(curl -sS "http://127.0.0.1:${GATEWAY_PORT}/api/profile" \
    | sed -n 's/.*"request_id":"\([^"]*\)".*/\1/p')
if [ -n "$back" ]; then
    PASS=$((PASS + 1))
    echo "  ok   X-Request-Id reaches the upstream"
else
    FAIL=$((FAIL + 1))
    echo "  FAIL X-Request-Id not forwarded"
fi

# The id is also echoed back, so it can be read from the browser's network tab
# rather than only from the container's logs. `always` matters here: nginx drops
# add_header from error responses without it, and the 404 below is one.
for probe in "/api/profile:200" "/api/nope:404"; do
    path=${probe%:*}
    want=${probe##*:}
    hdr=$(curl -sS -D - -o /dev/null "http://127.0.0.1:${GATEWAY_PORT}${path}" \
        | sed -n 's/^X-Request-Id: *//Ip' | tr -d '\r')
    if [ -n "$hdr" ]; then
        PASS=$((PASS + 1))
        printf '  ok   %-52s returns X-Request-Id (%s)\n' "$path HTTP $want" "$hdr"
    else
        FAIL=$((FAIL + 1))
        printf '  FAIL %-52s returns no X-Request-Id\n' "$path HTTP $want"
    fi
done

echo
echo "==> 10 MB upload is not rejected by the gateway"

# Nginx's default client_max_body_size is 1m, which would answer 413 before the
# service ever saw the body.
big=$(mktemp)
dd if=/dev/zero of="$big" bs=1M count=10 status=none
ctype=$(curl -sS -o /dev/null -w '%{http_code}' -X POST \
    -H 'Content-Type: application/octet-stream' --data-binary "@$big" \
    "http://127.0.0.1:${GATEWAY_PORT}/api/assignments/a1/submissions" || true)
rm -f "$big"
# The stub always answers 200, so anything other than 413 means the body got
# through the gateway.
if [ "$ctype" = "200" ]; then
    PASS=$((PASS + 1))
    echo "  ok   a 10 MB body reaches the upload route"
elif [ "$ctype" = "413" ]; then
    FAIL=$((FAIL + 1))
    echo "  FAIL 10 MB upload rejected with 413; client_max_body_size is too low"
else
    FAIL=$((FAIL + 1))
    echo "  FAIL unexpected status $ctype for the 10 MB upload"
fi

echo
echo "==> 13 MB body is still rejected"
toobig=$(mktemp)
dd if=/dev/zero of="$toobig" bs=1M count=13 status=none
ctype=$(curl -sS -o /dev/null -w '%{http_code}' -X POST \
    -H 'Content-Type: application/octet-stream' --data-binary "@$toobig" \
    "http://127.0.0.1:${GATEWAY_PORT}/api/assignments/a1/submissions" || true)
rm -f "$toobig"
if [ "$ctype" = "413" ]; then
    PASS=$((PASS + 1))
    echo "  ok   a 13 MB body is rejected with 413"
else
    FAIL=$((FAIL + 1))
    echo "  FAIL 13 MB body gave $ctype (want 413)"
fi

echo
echo "-------------------------------------------"
echo "passed: $PASS   failed: $FAIL"
[ "$FAIL" -eq 0 ] || exit 1
