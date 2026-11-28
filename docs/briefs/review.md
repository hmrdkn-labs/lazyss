# Brief: review (reused for each lazyss v0.3 branch)

You review one branch, read-only. The launch prompt names the branch and its brief
(`docs/briefs/<brief>.md`). You run in that branch's worktree.

## Do

1. Read the brief, then `git diff main...HEAD` (use `git merge-base` if main has moved).
2. Check the diff against the brief's owner decisions and "What to build":
   - argv only, never a shell string; values starting with `-` rejected before they reach argv
   - no Go cloud SDK, no token reads or writes, nothing that prints account object ids or tokens
   - a missing CLI or login degrades only that provider and never panics or blocks
   - every health check claims no more than it proves (a TCP probe ≠ login works) and has a doc comment
   - tests use fixtures and fake runners, assert exact argv, and do not hit the network
   - files touched stay inside the brief's "Files you own"
3. Run `make check` once and note the result.
4. Do not edit, commit, push, merge or tag.

## Report

Write `docs/briefs/review-<brief>.md` (untracked is fine) with findings ordered by severity:
`blocker` / `should-fix` / `nit`, each with `file:line`, what is wrong and the fix you'd suggest. End with
the `make check` result. Reply in at most 15 lines: the counts per severity and the blockers.
