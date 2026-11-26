# mathlib-agent

The toolchain of The Mathlib Handbook. The command is `mlib`.

The handbook explains the mathematics of Mathlib to a student, at the depth that each declaration needs. `mlib` reads the index of Mathlib that [mathlib-lean](https://github.com/tamnd/mathlib-lean) writes at each bump. It computes the depth of each declaration, the coverage of the handbook and the drift of the library from one commit to the next. It checks the map of [mathlib](https://github.com/tamnd/mathlib), ranks the work of the roles, and builds the site https://mathlib.tamnd.com.

`mlib` decides nothing about mathematics. Lean decides what builds, the checks of spec 14 decide what passes, and people decide what is reviewed.

## Install

You need Go 1.27 or later.

```sh
go install github.com/tamnd/mathlib-agent/cmd/mlib@latest
```

Each release on GitHub also has binaries for Linux, macOS and Windows, with checksums and a build provenance attestation.

## Use

Run `mlib help` to see the commands. A command that is not written yet exits with code 3 and gives the milestone that adds it. The full list of commands is in spec 11.1 of tamnd/mathlib.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | success |
| 1 | a failed check or a failed run |
| 2 | a usage error |
| 3 | the command is not written yet |

## Development

```sh
make check
```

This runs gofmt, go vet, the tests with the race detector and golangci-lint.

## Licence

MIT. See `LICENSE`.
