# Clio smoke tests

These tests use shell builtins only and have no side effects. `pass.sh` always
exits 0; `fail.sh` intentionally always exits 1. They are separate from `go test`.

From the project root:

```sh
clio exec --mode on-error -- /bin/sh tests/smoke/pass.sh
clio exec --mode on-error -- /bin/sh tests/smoke/fail.sh
```

The passing test emits only Clio metadata. The failing test exercises filtering
and preserves exit code 1. If filtering fails, Clio prints the bounded raw tail.
Use `--mode full` to see the complete test output without calling TypeSafe.
