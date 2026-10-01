---
name: clio
description: Run commands through Clio to control output volume while retaining failure evidence.
---

Choose output mode by intent. Use `clio exec --mode on-error -- <command> [args...]`
for development commands such as builds, tests, lint, package management, Docker,
compilation, and code generation. This is also the default mode.


Include `--context "<purpose>"` before `--` to tell the filter what you are trying
to accomplish. Give one or two concrete sentences about the goal and what this
command verifies; English is preferred when practical. Do not include the entire
conversation or credentials. For example:

```sh
clio exec --context "Verify that the pendant changes compile into the debug APK." -- ./gradlew assembleDevelopDebug
```

Clio sends this purpose together with the command, exit code, and log chunks to
Jev when filtering a failure. Context is optional; omit it if no useful goal can
be stated. Command failures in other modules can still be relevant blockers.

Use `--mode status` when only success/failure metadata matters. Use `--mode full`
when the output itself is needed, such as a diff or search results. Trivial one-line
probes may run directly. Preserve argument boundaries; avoid adding a shell unless
the command needs shell syntax.

On failure, Jev selects important raw log chunks and separately decides whether
context from the previous chunk's last ten lines or the next chunk's first ten lines
is needed. Those excerpts remain within the same stdout or stderr stream. Routine
progress lines may appear in selected chunks or context; this is expected, not a
filtering failure. Overlapping ranges are merged. Clio retains whole selected chunks
and context excerpts in relevance order up to 16 KiB of raw content, then renders
them in original order within each stream, stdout first. It does not summarize or
rewrite the logs.
`[clio: output omitted]` marks excluded output and is expected behavior.

Read the entire returned output, including later stderr sections, before diagnosing
the failure. Consider `full` only when information needed for diagnosis is missing,
not merely because routine lines or omission markers appear. When rerunning, consider
side effects before repeating installation, deployment, migration, or other mutations.
Clio does not save logs, so a past run's full output cannot be retrieved.

Authenticate once with `clio auth login`. Authentication is only needed for filtering
failed commands. A filtering failure returns bounded raw tails and retains the child
exit code. Use `--timeout 2m` when a child execution deadline is appropriate.
