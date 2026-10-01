# Clio

[日本語](README.ja.md)

CLI wrapper for coding agents that captures command output and selects raw failure
evidence using TypeSafe Jev. Logs are never persisted or rewritten into a diagnosis.

Developed by AI and designed by humans.

## Build and install

Requires Go 1.26. Run from the project root on macOS/Linux:

```sh
go build -o ./clio ./cmd/clio
mkdir -p "$HOME/.local/bin"
install -m 755 ./clio "$HOME/.local/bin/clio"
export PATH="$HOME/.local/bin:$PATH"
```

Add the `export PATH` line to your shell configuration to keep it across sessions.
To use the binary without installing it, run `./clio` from the project root.

## Usage

Run `clio auth login` in an interactive terminal. It saves your TypeSafe API token
without verifying it with the API. Authentication is needed only for filtering
failed commands.

```sh
clio auth login
clio exec -- go test ./...
clio exec --mode status --timeout 2m -- go build ./...
clio exec --mode full -- git diff
clio exec --context "Verify that the pendant changes compile into the debug APK." -- ./gradlew assembleDevelopDebug
```

`on-error` is the default: successful commands emit metadata only; failures emit
selected raw log content on stdout. `status` emits metadata only. `full` returns
captured stdout and stderr to their original streams. All modes capture output in
memory and return it after the command finishes; even `full` does not stream
progress. Stdout and stderr are captured separately, so their combined chronological
order is not preserved. Successfully filtered logs are returned on stdout, with
`[stdout]` and `[stderr]` sections. Metadata is always on stderr and includes exit
code, child duration, signal when applicable, timeout and cancellation.
Arguments after `--` are executed directly, with stdin, working directory, and
environment inherited except `TYPESAFE_API_KEY`.

Clio does not interpret shell expressions: use `clio exec -- echo 1`, not
`clio exec -- 'echo 1'`. If shell syntax is needed, explicitly run a shell:

```sh
clio exec -- sh -c 'printf "hello\n" && printf "world\n"'
```

Use optional `--context "<purpose>"` before `--` to describe the agent's goal in
one or two sentences. Filtering sends it as `agent_context` alongside the command,
exit code, and chunks, and asks Jev to consider both failure diagnosis and the next
action toward that goal. Failures blocking the command remain relevant even in
other modules. Omitting context preserves the existing failure-focused question.
The value is not added to child arguments or environment, or echoed in metadata.
It is sent to TypeSafe only when filtering is needed; do not include credentials
or the entire conversation. `status`, `full`, and successful `on-error` runs do not
send it to TypeSafe.

Child exit codes are preserved, including `128 + signal`. Argument errors exit 2;
internal errors exit 1. `--timeout` limits only child execution; omitted means no
deadline, as does `--timeout 0`. Builds taking several minutes can run normally.
Filtering has a separate 30 second deadline for all batches and retries combined.
HTTP 429/529 responses are retried at most twice within that deadline, using backoff
and `Retry-After`.
Filter failures, invalid answers, missing credentials, and empty selections from
nonempty logs emit `filtering failed` with a reason and return each stream's last
4 KiB, retaining the child exit code. No credentials are needed for `status`,
`full`, or successful `on-error` runs.

## Credentials and model

`clio auth login` reads the token without echo using a terminal and stores it under
`os.UserConfigDir()/clio/credentials.json`: on macOS,
`~/Library/Application Support/clio/credentials.json`; on Linux, usually
`~/.config/clio/credentials.json`. The directory is 0700 and file is 0600; updates
use a same-directory temporary file and atomic rename. Symlinks, wrong ownership,
and overly permissive modes are rejected. **Storage is plaintext**: these permissions
restrict other users, but processes running as your user can still read the token.
The token is loaded only when filtering is needed and sent as Bearer authentication.
Set `TYPESAFE_MODEL` to override the default `jev-latest` model.
An existing `TYPESAFE_API_KEY` is not used as Clio's credential source.

## Filtering and data sent to TypeSafe

On a failed `on-error` run, Clio sends normalized stdout and stderr in chunks to
the TypeSafe API. The entire captured log is eligible for transmission, not just
the chunks later returned to the agent. Requests also contain the command, exit
code, optional context, and neighboring chunk excerpts. Clio does not redact
secrets from logs or command arguments. `status`, `full`, and successful `on-error`
runs do not call TypeSafe.

For judgment, Clio removes ANSI escapes and converts carriage returns to newlines.
Chunks contain at most 4 KiB of normalized text, preferring line boundaries;
single lines longer than the limit are split. Raw ranges can be larger because
they retain ANSI escapes and original carriage returns.
Clio evaluates batches of up to eight chunks with Noul relevance and selects
chunks scoring at least 0.5. Jev separately decides whether to include context from
the previous chunk's last ten lines or the next chunk's first ten lines. Selected
context is returned as raw log text. Overlaps are merged; complete selected chunks
and context excerpts are admitted in relevance order up to 16 KiB of raw content.
A group that exceeds the remaining limit is skipped whole. If nothing fits, filtering
fails and returns the bounded tails described above. Output preserves original order
within each stream, stdout first; read later stderr sections as well. Labels and
omission markers are additional bytes. Use `full` only if information needed for
diagnosis is missing.

Clio does not save logs, so obtaining a previous run's full output requires rerunning
the command. Consider side effects before repeating installation, deployment,
or other mutations.

## Agent skill

[skills/clio/SKILL.md](skills/clio/SKILL.md) explains mode selection, task context,
expected omission markers, and when to rerun with `full`. Copy the `skills/clio`
folder into your agent's skills directory, or reference it from project instructions.
The skill guides the agent; the `clio` binary must also be installed and on PATH.

## Limits and development checks

Capture uses memory proportional to output. This version has no disk spill, saved
logs, command-specific heuristics, or Windows process-tree control. A descendant
holding an output pipe open is bounded by a 200 ms drain deadline after child exit.
Output arriving after that deadline may be lost, including in `full`.
macOS/Linux cancellation kills the child process group.

From the project root, with Clio installed:

```sh
clio exec -- go test -race ./...
clio exec -- go vet ./...
clio exec -- go build -o ./clio ./cmd/clio
```

HTTP integration follows the [official TypeSafe API](https://docs.typesafe.ai/api).
Tests use fake filters, fake Jev clients, and local HTTP servers; no live API is needed.
See [tests/smoke/README.md](tests/smoke/README.md) for lightweight commands that
always succeed or intentionally fail.
