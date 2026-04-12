package workflow

// HabitWorkflow runs an ongoing improvement practice.
var HabitWorkflow = Workflow{
	Name:  "habit",
	Start: "load-habit",
	Inputs: []Input{
		{Name: "habit-name", Required: true},
	},
	Nodes: []Node{
		{
			Name: "load-habit",
			Command: `cat ".incrementum/habits/$(cat $SCRATCHPAD/habit-name).md" \
  > $SCRATCHPAD/habit-instructions`,
		},
		{
			Name: "build-habit-prompt",
			Command: `cat > $SCRATCHPAD/prompt <<'PROMPT'
## Workflow Context

You are part of an incremental code workflow designed to complete large projects
through small, focused changes.

## Review Questions

- Does it do what the change description says?
- Does it move us towards the goal?
- Is it free of defects?
- Does it include proper test coverage?
- Does it keep the relevant specs up to date?

## Review Instructions

Publish your review to the file ./.incrementum-feedback

Write one of the following allcaps words as the first line:
- ACCEPT -- if the changes pass review and should be merged
- ABANDON -- if there's nothing worth doing right now (valid for habits)
- REQUEST_CHANGES -- if some modifications could get the changes into shape

## Phase: Habit Implementation

You are working on the habit described below.
Habits are ongoing improvement work without completion state. Unlike regular
todos, habits are never "done" — they represent continuous practices. Find
something worth improving and implement it.

When you make changes, write a detailed commit message to
./.incrementum-commit-message — describe the what _and_ the why. Keep changes
focused on a single improvement.

If there's nothing worth doing right now, that's fine — make no changes and
write nothing to .incrementum-commit-message.

PROMPT
echo "" >> $SCRATCHPAD/prompt
echo "## Habit Instructions" >> $SCRATCHPAD/prompt
cat $SCRATCHPAD/habit-instructions >> $SCRATCHPAD/prompt
echo "" >> $SCRATCHPAD/prompt
echo "## Series Log" >> $SCRATCHPAD/prompt
jj log --no-graph -r 'fork_point(@|main)::@-' >> $SCRATCHPAD/prompt 2>/dev/null || true`,
		},
		{
			Name: "build-feedback-prompt",
			Command: `cat > $SCRATCHPAD/prompt <<'PROMPT'
## Phase: Habit Implementation (responding to feedback)

You are responding to feedback on your previous change for this habit.
Update the commit message at ./.incrementum-commit-message to describe the
entire change as it stands.

PROMPT
echo "## Habit Instructions" >> $SCRATCHPAD/prompt
cat $SCRATCHPAD/habit-instructions >> $SCRATCHPAD/prompt
echo "" >> $SCRATCHPAD/prompt
echo "## Feedback" >> $SCRATCHPAD/prompt
cat $SCRATCHPAD/feedback-text >> $SCRATCHPAD/prompt 2>/dev/null || true
echo "" >> $SCRATCHPAD/prompt
echo "## Series Log" >> $SCRATCHPAD/prompt
jj log --no-graph -r 'fork_point(@|main)::@-' >> $SCRATCHPAD/prompt 2>/dev/null || true`,
		},
		{
			Name: "implement",
			Command: `claude -p "$(cat $SCRATCHPAD/prompt)" \
  --model claude-sonnet-4-20250514 \
  --permission-mode bypassPermissions`,
		},
		{
			Name:    "check-changes",
			Command: `jj log -r @ -T empty --no-graph > $SCRATCHPAD/empty`,
		},
		{
			Name: "test",
			Command: `go tool run test 2>&1 | tee $SCRATCHPAD/test-output
exit_code=${PIPESTATUS[0]}
if [ $exit_code -ne 0 ]; then
  echo "Tests failed:" > $SCRATCHPAD/feedback-text
  cat $SCRATCHPAD/test-output >> $SCRATCHPAD/feedback-text
fi
exit $exit_code`,
		},
		{
			Name: "build-habit-review-prompt",
			Command: `cat > $SCRATCHPAD/prompt <<'PROMPT'
## Phase: Habit Review

Review the changes in the jujutsu working tree. These changes were made as
part of the habit described below.

Some jj commands that may be useful:
- ` + "`jj show --summary`" + `
- ` + "`jj diff --git`" + `

PROMPT
echo "" >> $SCRATCHPAD/prompt
echo "## Habit Instructions" >> $SCRATCHPAD/prompt
cat $SCRATCHPAD/habit-instructions >> $SCRATCHPAD/prompt
echo "" >> $SCRATCHPAD/prompt
echo "## Change Description" >> $SCRATCHPAD/prompt
cat .incrementum-commit-message >> $SCRATCHPAD/prompt 2>/dev/null || true`,
		},
		{
			Name: "review",
			Command: `rm -f .incrementum-feedback
claude -p "$(cat $SCRATCHPAD/prompt)" \
  --model claude-sonnet-4-20250514 \
  --permission-mode bypassPermissions`,
		},
		{
			Name: "parse-feedback",
			Command: `head -1 .incrementum-feedback > $SCRATCHPAD/feedback
tail -n +3 .incrementum-feedback > $SCRATCHPAD/feedback-text 2>/dev/null || true`,
		},
		{
			Name:    "commit",
			Command: `jj commit -m "$(cat .incrementum-commit-message)"`,
		},
		{
			Name:    "nothing-to-do",
			Command: `echo "Nothing to do for habit $(cat $SCRATCHPAD/habit-name)"`,
		},
	},
	Edges: []Edge{
		{From: "load-habit", To: "build-habit-prompt"},
		{From: "build-habit-prompt", To: "implement"},
		{From: "implement", To: "check-changes"},
		{From: "check-changes", To: "nothing-to-do", Condition: `grep -q "true" $SCRATCHPAD/empty`},
		{From: "check-changes", To: "test"},
		{From: "test", To: "build-feedback-prompt", Condition: `[ $EXIT_CODE -ne 0 ]`},
		{From: "test", To: "build-habit-review-prompt"},
		{From: "build-feedback-prompt", To: "implement"},
		{From: "build-habit-review-prompt", To: "review"},
		{From: "review", To: "parse-feedback"},
		{From: "parse-feedback", To: "commit", Condition: `grep -q "^ACCEPT" $SCRATCHPAD/feedback`},
		{From: "parse-feedback", To: "build-feedback-prompt", Condition: `grep -q "^REQUEST_CHANGES" $SCRATCHPAD/feedback`},
		{From: "parse-feedback", To: "nothing-to-do", Condition: `grep -q "^ABANDON" $SCRATCHPAD/feedback`},
		{From: "commit", To: "build-habit-prompt"},
	},
}
