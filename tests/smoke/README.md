# Clio smoke tests

These tests use shell builtins only and have no side effects. `pass.sh` always
exits 0; `fail.sh` intentionally always exits 1. They are separate from `go test`.

From the project root:

```sh
clio exec -- /bin/sh tests/smoke/pass.sh
clio exec -- /bin/sh tests/smoke/fail.sh
```

The passing test emits only Clio metadata. The failing test exercises filtering
and preserves exit code 1. If filtering is unavailable, Clio prints the bounded
raw tail. These smoke commands require no live API when credentials are absent.
