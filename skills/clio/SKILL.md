---
name: clio
description: Run development commands through Clio to control output volume while retaining failure evidence.
---

Use `clio exec -- <command> [args...]` for development commands such as builds,
tests, lint, package management, compilation, and code generation. Successful
commands emit metadata only. Failed commands return selected raw log content and
metadata.

Clio does not replace or override the coding agent's command safety controls.
Never wrap a command that the agent refused or requires approval for in Clio to
bypass that decision. Follow the agent's normal approval and safety process before
running a command.

Run commands whose output itself is needed, such as `git diff` or searches,
directly. If selected failure output does not contain enough information, run the
command directly under the same approval and safety rules, considering side effects
before repeating it. Clio does not save logs, so output from a past run cannot be
retrieved later.

Include `--context "<purpose>"` before `--` when a concise goal will help Jev
select useful evidence. Give one or two concrete sentences about the goal and what
the command verifies. Do not include credentials or the entire conversation. For
example:

```sh
clio exec --context "Verify that the app changes compile into the debug APK." -- ./gradlew assembleDevelopDebug
```

On failure, Jev selects important raw log chunks and separately decides whether
context from the previous chunk's last ten lines or the next chunk's first ten
lines is needed. These excerpts remain within the same stdout or stderr stream.
Routine progress lines may appear in selected chunks or context; this is expected.
Overlapping ranges are merged. Clio retains whole selected chunks and context
excerpts in relevance order up to 16 KiB of raw content, then renders them in
original order within each stream, stdout first. It does not summarize or rewrite
the logs. `[clio: output omitted]` marks excluded output.

Read the entire returned output, including later stderr sections, before diagnosing
the failure. When rerunning, consider side effects before repeating installation,
deployment, migration, or other mutations.

Authenticate with `clio auth login` when filtering a failed command. Authentication
is only needed for filtering. If filtering fails, Clio returns bounded raw tails
and preserves the child exit code. Use `--timeout 2m` when a child execution
deadline is appropriate.
