# Go Toolchain and Dependency Maintenance

Keep repositories current without turning updates into unreviewed churn inside feature work.

<!-- Last verified: 2026-10-28 against the go1.27.0 toolchain -->

## Baseline

Services target the current stable Go release; the handbook's minimum language version is Go 1.26. Before changing the baseline, check the official [downloads](https://go.dev/dl/) and [release history](https://go.dev/doc/devel/release) pages and read the release notes, then update the "Last verified" lines in this handbook.

Keep these synchronized:

- the `go` (and optional `toolchain`) directive in `go.mod`;
- CI and release build images;
- local and dev-container toolchains;
- code-generation and lint environments.

The `go` directive is a minimum. A reusable library may deliberately keep an older minimum and test against both it and the current release; do not raise a consumer requirement by accident. `go mod init` does not pick your organization's baseline — set it explicitly.

## Classify the Work

| Kind | Scope |
|---|---|
| Security or compatibility fix | Narrowest dependency set that resolves it |
| Scheduled refresh | The whole build list, as its own reviewed change |
| Toolchain migration | `go.mod`, CI, builders, generators, linters, and compatibility tests |
| Feature work | No broad dependency diff; a targeted bump only if the feature needs it |

## Audit Before Updating

```bash
go version
go env GOTOOLCHAIN
go list -m -u all          # available upgrades, deprecations, retractions
```

Also read `go.mod`, the Makefile, CI workflows, Dockerfiles, `replace` directives, and any dependency bot configuration.

## Update

Prefer the repository's checked-in target (`make deps`) when it exists. Otherwise:

```bash
go get -u ./...            # or go get example.com/mod@vX.Y.Z for a targeted fix
go mod tidy
```

Run broad refreshes on a regular cadence (monthly is typical) so security fixes are cheap. Then:

1. Inspect every `go.mod`, `go.sum`, and generated-file change.
2. Look for unexpected major versions, `go`/`toolchain` edits, `replace` changes, deprecations, and retractions.
3. Read release notes for material direct dependencies — especially gRPC, protobuf, OpenTelemetry (upgrade its modules together), and the database driver.
4. Regenerate protobuf and other artifacts if generator versions changed.

## Security Scanning

```bash
govulncheck ./...
```

`govulncheck` uses call-graph analysis to report vulnerabilities your code can actually reach. Also scan the built container image with your registry's or CI's image scanner: the image contains OS packages and the compiled binary, which source scanning does not cover.

Never weaken scanner policy or add an ignore just to make CI green. A legitimate exception names the advisory, explains why it does not apply, has an owner, and has a review or expiry date.

## Verification Gate

After any dependency or toolchain change:

1. Confirm the intended Go version is selected.
2. Run formatting, generation, `make lint`, and the full test suite.
3. Run `-race` for affected concurrent paths.
4. Run `govulncheck` and scan the image.
5. Review the complete diff for unrelated upgrades and accidental contract changes.
6. Record any check that could not run and the remaining risk.

"It compiles" is not evidence that an update is safe.
