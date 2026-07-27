# Cross-Station Workflow (Windows + macOS)

You develop kd-server from **two machines** (a Windows station and a MacBook).
The only way that stays painless is discipline about **sync** and **branches**.
These are the rules. They exist because we already hit every failure they prevent.

## The two rules that matter most

1. **Start of every session → pull latest.** Before writing a line of code:
   ```bash
   bash scripts/sync-start.sh
   ```
2. **End of every session → push everything.** Before you walk away:
   ```bash
   bash scripts/sync-end.sh
   ```

If the remote always has your latest, the other station always starts current.
Every cross-station problem we've had came from skipping one of these.

## Branch rule: one branch per feature, both stations on it

The mess we just cleaned up happened because the **MacBook pushed the TUI to
`main`** while the **Windows station worked on `feat/admin-htmx`** — two
stations, two branches, silent divergence.

- Pick **one** working branch for a feature and use it from **both** machines.
- Don't commit feature work straight to `main` from one station while the other
  is on a feature branch.
- `sync-start.sh` also refreshes your local `main` so it never goes stale, even
  while you work on a feature branch.

## Line endings: LF everywhere (already enforced)

`.gitattributes` pins every text file to **LF** with `eol=lf`. That overrides a
station's `core.autocrlf`, so Windows and macOS check out **byte-identical**
files — no phantom "modified" files, no `gofmt`/`gofumpt` CRLF failures.

One-time per Windows checkout (already done on this machine):
```bash
git config core.autocrlf false
git config core.eol lf
git rm -r --cached -q . && git reset --hard   # re-checkout as LF
```

## Daily loop

```
sit down ─► sync-start.sh ─► work ─► test ─► commit ─► sync-end.sh ─► leave
```

- **Commit only after you've tested** — that rule is unchanged.
- `sync-start.sh` refuses to run over uncommitted changes (it won't clobber work).
- `sync-end.sh` never commits for you; it warns about uncommitted work and
  pushes what's already committed.

## If the branch has diverged anyway

`sync-start.sh` will stop and tell you (`Diverged: N local / M remote`). Then:
```bash
git fetch origin
git log --oneline HEAD..@{u}     # what the other station added
git merge @{u}                   # or: git rebase @{u}
go build ./... && go test ./...  # always verify after a merge
```
