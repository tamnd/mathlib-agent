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

### The roles

`mlib queue -role <role>` prints the work queue of a role, with the rank and the reason of each item. The rank is the formula of spec 11.5: the weight of the volume in the current milestone, the planned sections that wait on the item, the level factor of a stale section, the failed runs and the age factor. The roles are planner, informalizer, family, writer, exampler, exerciser, checker, refresher and porter.

`mlib agent run <role>` takes the top of the queue, or the item of `-item`, and writes a run record to `records/runs/<id>/run.json` with the prompt in `prompt.md`. The prompts are in `prompts/<role>.md`. Text from Mathlib goes in marked data blocks. The budget comes from `agent.yaml` of the map and the spend of the day in `agent/ledger.jsonl`. With no `agent.yaml`, the budget is zero and no call goes out. The environment variable `MLIB_RUNNER` names the command that runs the prompt. It reads the prompt on stdin. Use `-dry-run` to see the record and the prompt and to write nothing. mlib sets the status `ran` or `failed`. It never sets `passed`, because Lean, the checks of spec 14 and people decide that.

## Use as a library

The packages are at the root of the module, so other Go programs can import them. The API can change before v1.0.0.

| Package | Purpose |
|---|---|
| `agent` | rank the work of each role, apply the budget, write the run records and start the runner (spec 11.3 to 11.6) |
| `blueprint` | read and check `blueprint/volumes.yaml` and `blueprint/milestones.yaml` of the map |
| `index` | read the index of Mathlib that mathlib-lean writes |
| `depth` | compute the depth E0 to E3 of each declaration (spec 04.4) |
| `coverage` | compute the coverage numbers (spec 04.5) and the module records (spec 09.1) |
| `drift` | compare two indexes and write the drift items (spec 05.4) |
| `family` | read `blueprint/families.yaml` and give each declaration at most one E0 family (spec 09.4) |
| `edition` | read and write the edition log `records/editions.json` (spec 05.7) |
| `informal` | read and check the E1 records `informal/<vol>.jsonl`, find the stale ones and take a sample (spec 09.3, 14.3) |
| `prompts` | the prompt of each role, with its version (spec 11.4) |
| `plan` | make the draft section plan of a volume (spec 09.1, 07.7) |
| `site` | build and check the site mathlib.tamnd.com |

```go
import "github.com/tamnd/mathlib-agent/index"

ix, err := index.Load("index")
```

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
