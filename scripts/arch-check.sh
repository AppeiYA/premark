#!/bin/sh
set -e

# Rule 1: No struct tags in domain, ports, usecase
if grep -rnE '(json|db|yaml|gorm):"' internal/domain internal/ports internal/usecase 2>/dev/null; then
    echo "ARCH VIOLATION: struct tags found in domain, ports, or usecase"
    exit 1
fi

# Rule 2: domain must not import any premark/ packages
for pkg in $(go list ./internal/domain/... 2>/dev/null || true); do
    imports=$(go list -f '{{join .Imports "\n"}}' "$pkg" 2>/dev/null || true)
    if echo "$imports" | grep -q '^premark/'; then
        echo "ARCH VIOLATION: $pkg imports premark package"
        exit 1
    fi
done

# Rule 3: ports must only import premark/internal/domain
for pkg in $(go list ./internal/ports/... 2>/dev/null || true); do
    imports=$(go list -f '{{join .Imports "\n"}}' "$pkg" 2>/dev/null || true)
    bad=$(echo "$imports" | grep '^premark/' | grep -v '^premark/internal/domain$' || true)
    if [ -n "$bad" ]; then
        echo "ARCH VIOLATION: $pkg imports disallowed premark package: $bad"
        exit 1
    fi
done

# Rule 4: usecase must only import premark/internal/domain and premark/internal/ports
for pkg in $(go list ./internal/usecase/... 2>/dev/null || true); do
    imports=$(go list -f '{{join .Imports "\n"}}' "$pkg" 2>/dev/null || true)
    bad=$(echo "$imports" | grep '^premark/' | grep -v -E '^premark/internal/(domain|ports)$' || true)
    if [ -n "$bad" ]; then
        echo "ARCH VIOLATION: $pkg imports disallowed premark package: $bad"
        exit 1
    fi
done

# Rule 5: adapter must not import premark/internal/usecase
for pkg in $(go list ./internal/adapter/... 2>/dev/null || true); do
    imports=$(go list -f '{{join .Imports "\n"}}' "$pkg" 2>/dev/null || true)
    if echo "$imports" | grep -q '^premark/internal/usecase'; then
        echo "ARCH VIOLATION: $pkg imports usecase"
        exit 1
    fi
done

# Rule 6: adapter/in must not import adapter/out, and vice versa
for pkg in $(go list ./internal/adapter/in/... 2>/dev/null || true); do
    imports=$(go list -f '{{join .Imports "\n"}}' "$pkg" 2>/dev/null || true)
    if echo "$imports" | grep -q '^premark/internal/adapter/out'; then
        echo "ARCH VIOLATION: adapter/in package $pkg imports adapter/out"
        exit 1
    fi
done

for pkg in $(go list ./internal/adapter/out/... 2>/dev/null || true); do
    imports=$(go list -f '{{join .Imports "\n"}}' "$pkg" 2>/dev/null || true)
    if echo "$imports" | grep -q '^premark/internal/adapter/in'; then
        echo "ARCH VIOLATION: adapter/out package $pkg imports adapter/in"
        exit 1
    fi
done

# Rule 7: adapter/out/X must not import a different adapter/out/Y
for pkg in $(go list ./internal/adapter/out/... 2>/dev/null || true); do
    imports=$(go list -f '{{join .Imports "\n"}}' "$pkg" 2>/dev/null || true)
    bad=$(echo "$imports" | grep '^premark/internal/adapter/out/' | grep -v "^$pkg" || true)
    if [ -n "$bad" ]; then
        echo "ARCH VIOLATION: $pkg imports other adapter/out package: $bad"
        exit 1
    fi
done

# Rule 8: Non-test package outside cmd/, internal/testsupport and internal/config must not import testsupport
for pkg in $(go list ./... 2>/dev/null || true); do
    case "$pkg" in
        premark/cmd/*|premark/internal/testsupport|premark/internal/testsupport/*|premark/internal/config|premark/internal/config/*)
            ;;
        *)
            imports=$(go list -f '{{join .Imports "\n"}}' "$pkg" 2>/dev/null || true)
            if echo "$imports" | grep -q '^premark/internal/testsupport'; then
                echo "ARCH VIOLATION: $pkg imports testsupport"
                exit 1
            fi
            ;;
    esac
done

echo "arch-check: PASS"
exit 0
