package workflow

// MergeWorkflow rebases a change onto a target bookmark with conflict resolution.
var MergeWorkflow = Workflow{
	Name:  "merge",
	Start: "rebase",
	Inputs: []Input{
		{Name: "change-id", Required: true},
		{Name: "target", Default: "main"},
	},
	Nodes: []Node{
		{
			Name: "rebase",
			Command: `jj rebase -b "$(cat $SCRATCHPAD/change-id)" \
  -d "$(cat $SCRATCHPAD/target)"`,
		},
		{
			Name: "check-conflicts",
			Command: `jj log --no-graph \
  -r "$(cat $SCRATCHPAD/target)::$(cat $SCRATCHPAD/change-id)" \
  -T 'if(conflict, change_id ++ "\n")' \
  > $SCRATCHPAD/conflicts`,
		},
		{
			Name: "build-resolve-prompt",
			Command: `CHANGE=$(head -1 $SCRATCHPAD/conflicts)
jj new "$CHANGE"
cat > $SCRATCHPAD/prompt <<PROMPT
Resolve merge conflicts in the current workspace.

- Locate and fix conflict markers (<<<<<<<, =======, >>>>>>>).
- Keep only the intended final code.
- Do not introduce unrelated changes.
- Ensure no conflict markers remain in the tree.

Resolve merge conflicts for change $CHANGE rebased onto $(cat $SCRATCHPAD/target).
PROMPT`,
		},
		{
			Name: "resolve",
			Command: `claude -p "$(cat $SCRATCHPAD/prompt)" \
  --model claude-sonnet-4-20250514 \
  --permission-mode bypassPermissions`,
		},
		{
			Name:    "squash",
			Command: `jj squash`,
		},
		{
			Name: "advance-bookmark",
			Command: `jj bookmark set "$(cat $SCRATCHPAD/target)" \
  -r "$(cat $SCRATCHPAD/change-id)"`,
		},
		{
			Name: "verify-clean",
			Command: `REMAINING=$(jj log --no-graph \
  -r "$(cat $SCRATCHPAD/target)::$(cat $SCRATCHPAD/change-id)" \
  -T 'if(conflict, change_id ++ "\n")')
if [ -n "$REMAINING" ]; then
  echo "Conflicts remain after merge" >&2
  exit 1
fi`,
		},
	},
	Edges: []Edge{
		// rebase -> check-conflicts
		{From: "rebase", To: "check-conflicts"},

		// check-conflicts -> advance-bookmark (no conflicts) or resolve (conflicts)
		{From: "check-conflicts", To: "advance-bookmark", Condition: `[ ! -s $SCRATCHPAD/conflicts ]`},
		{From: "check-conflicts", To: "build-resolve-prompt"},

		// build-resolve-prompt -> resolve
		{From: "build-resolve-prompt", To: "resolve"},

		// resolve -> squash
		{From: "resolve", To: "squash"},

		// squash -> check-conflicts (cycle: re-check for remaining conflicts)
		{From: "squash", To: "check-conflicts"},

		// advance-bookmark -> verify-clean
		{From: "advance-bookmark", To: "verify-clean"},
	},
}
