#!/bin/sh
# Intentionally failing smoke test; run explicitly, outside the normal test suite.
if [ 1 -eq 2 ]; then
    printf 'PASS: unexpected assertion success\n'
    exit 0
fi
printf 'FAIL: intentional assertion failure: expected 1 to equal 2\n' >&2
exit 1
