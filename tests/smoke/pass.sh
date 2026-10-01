#!/bin/sh
# Deterministic smoke test: shell builtins only, no files or network access.
if [ 1 -eq 1 ]; then
    printf 'PASS: 1 equals 1\n'
    exit 0
fi
printf 'FAIL: unexpected assertion failure\n' >&2
exit 1
