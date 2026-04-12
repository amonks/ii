package workflow

// JobWorkflow automates todo completion via an implement/test/review/commit loop.
var JobWorkflow = Workflow{
	Name:  "job",
	Start: "mark-in-progress",
	Inputs: []Input{
		{Name: "todo-id", Required: true},
	},
	Nodes: []Node{
		{
			Name:    "mark-in-progress",
			Command: `ii todo start "$(cat $SCRATCHPAD/todo-id)"`,
		},
		{
			Name: "build-impl-prompt",
			Command: `cat > $SCRATCHPAD/prompt <<'PROMPT'
## Workflow Context

You are part of an incremental code workflow designed to complete large projects
through small, focused changes. This approach is intentional: attempting large
projects in a single context leads to degraded output quality as context fills
up and costs increase. By breaking work into atomic changes, each phase operates
with fresh context and focused attention.

**The Loop**

1. IMPLEMENTATION — Make the next atomic change toward the todo. If nothing
   remains to be done, make no changes.
2. TESTING — Tests run automatically. If they fail, control returns to
   IMPLEMENTATION with the failure output as feedback.
3. CODE REVIEW — A reviewer evaluates the single change. Outcomes: ACCEPT
   (proceed), REQUEST_CHANGES (return to IMPLEMENTATION with feedback), or
   ABANDON (give up on the todo entirely).
4. COMMIT — The change is finalized and committed. The loop restarts at
   IMPLEMENTATION for the next iteration.

When an IMPLEMENTATION phase makes no changes — indicating the todo is
satisfied — the loop exits and a PROJECT REVIEW evaluates the entire series
before marking the todo complete.

## Review Questions

- Does it do what the change description says?
- Does it move us towards the goal in the todo?
- Is it _necessary_ for moving us towards the goal in the todo?
- Is it free of defects?
- Is the domain modeling coherent and sound?
- Are things in the right place?
- Does it include proper test coverage?
- Does it keep the relevant specs up to date?
- Does the change description describe the _entire_ change?
- Does it conform to the norms of the code areas it modifies?
- Does it work?

## Review Instructions

Publish your review to the file ./.incrementum-feedback

Write one of the following allcaps words as the first line:
- ACCEPT -- if the changes pass review and should be merged
- ABANDON -- if the changes are so off-base as to be a lost cause
- REQUEST_CHANGES -- if some modifications could get the changes into shape

After the outcome, add a blank line and your review comments. For ACCEPT, briefly
note what looks good or any observations. For ABANDON or REQUEST_CHANGES, explain
the issues in detail.

## Phase: Implementation

You are in the IMPLEMENTATION phase, creating the next change towards completing
a todo. Make the highest-priority remaining change toward resolution of the todo.
Dot 'i's, cross 't's: this change should be complete and atomic; tests should
pass, specs should be updated. Don't do additional work that does not move us
towards todo resolution. If the todo is already complete, make no changes.

When you make changes, write a change description (jj's equivalent of a commit
message) to ./.incrementum-commit-message. Describe the what _and_ the why.

PROMPT
echo "" >> $SCRATCHPAD/prompt
echo "## Todo" >> $SCRATCHPAD/prompt
ii todo show "$(cat $SCRATCHPAD/todo-id)" >> $SCRATCHPAD/prompt
echo "" >> $SCRATCHPAD/prompt
echo "## Series Log" >> $SCRATCHPAD/prompt
jj log --no-graph -r 'fork_point(@|main)::@-' >> $SCRATCHPAD/prompt 2>/dev/null || true`,
		},
		{
			Name: "build-feedback-prompt",
			Command: `cat > $SCRATCHPAD/prompt <<'PROMPT'
## Workflow Context

You are part of an incremental code workflow. See the review questions and
review instructions from the previous iteration — they still apply.

## Phase: Implementation (responding to feedback)

You are in the IMPLEMENTATION phase, responding to feedback on your previous
change. The todo may take a series of changes to resolve.
When you make changes, write a change description to ./.incrementum-commit-message.
Update it so it describes the entire change as it stands, not just the
modifications from this round of feedback. This is the commit message for the
whole change. Describe the what _and_ the why.

PROMPT
echo "" >> $SCRATCHPAD/prompt
echo "## Todo" >> $SCRATCHPAD/prompt
ii todo show "$(cat $SCRATCHPAD/todo-id)" >> $SCRATCHPAD/prompt
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
			Name: "build-review-prompt",
			Command: `cat > $SCRATCHPAD/prompt <<'PROMPT'
## Workflow Context

You are part of an incremental code workflow. See the review questions and
review instructions — they still apply.

## Phase: Code Review

You are in the CODE REVIEW phase, evaluating a single change.

Review the change in the jujutsu working tree. This change is one iteration
toward completing the todo and is described below.

Some jj commands that may be useful:
- ` + "`jj show --summary`" + ` to see the files modified in the change
- ` + "`jj diff --git`" + ` to see a diff of the change in standard git format

Review the change. Use the review questions as guidance.

Do not ask for needless refactoring: it's important that new things go in the
right place, but it's also important that changes stay small and focused.

PROMPT
echo "" >> $SCRATCHPAD/prompt
echo "## Change Description" >> $SCRATCHPAD/prompt
cat .incrementum-commit-message >> $SCRATCHPAD/prompt 2>/dev/null || true
echo "" >> $SCRATCHPAD/prompt
echo "## Todo" >> $SCRATCHPAD/prompt
ii todo show "$(cat $SCRATCHPAD/todo-id)" >> $SCRATCHPAD/prompt`,
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
			Name: "build-project-prompt",
			Command: `cat > $SCRATCHPAD/prompt <<'PROMPT'
## Workflow Context

You are part of an incremental code workflow. See the review questions and
review instructions — they still apply.

## Phase: Project Review

You are in the PROJECT REVIEW phase, evaluating the entire series of changes.

The IMPLEMENTATION phase has indicated no further changes are needed. Review the
complete series in the jujutsu working tree. These changes, taken together, are
meant to resolve the todo below.

Some jj commands that may be useful:
- ` + "`jj log --revisions 'fork_point(@|main)..@'`" + ` to see the list of changes in the series
- ` + "`jj diff --git --from 'fork_point(@|main)' --to @`" + ` to see the entire diff from the series
- ` + "`jj show --summary <change_id>`" + ` to see the files modified in a specific change
- ` + "`jj diff --git -r <change_id>`" + ` to see a diff of a specific change

Review the series. Use the review questions as guidance.

PROMPT
echo "" >> $SCRATCHPAD/prompt
echo "## Todo" >> $SCRATCHPAD/prompt
ii todo show "$(cat $SCRATCHPAD/todo-id)" >> $SCRATCHPAD/prompt`,
		},
		{
			Name: "project-review",
			Command: `rm -f .incrementum-feedback
claude -p "$(cat $SCRATCHPAD/prompt)" \
  --model claude-sonnet-4-20250514 \
  --permission-mode bypassPermissions`,
		},
		{
			Name: "parse-project-feedback",
			Command: `head -1 .incrementum-feedback > $SCRATCHPAD/feedback
tail -n +3 .incrementum-feedback > $SCRATCHPAD/feedback-text 2>/dev/null || true`,
		},
		{
			Name:    "mark-finished",
			Command: `ii todo finish "$(cat $SCRATCHPAD/todo-id)"`,
		},
		{
			Name:    "reopen-todo",
			Command: `ii todo reopen "$(cat $SCRATCHPAD/todo-id)"`,
		},
	},
	Edges: []Edge{
		// mark-in-progress -> build-impl-prompt
		{From: "mark-in-progress", To: "build-impl-prompt"},

		// build-impl-prompt -> implement
		{From: "build-impl-prompt", To: "implement"},

		// implement -> check-changes
		{From: "implement", To: "check-changes"},

		// check-changes -> test (tree not empty) or project-review (tree empty)
		{From: "check-changes", To: "test", Condition: `grep -q "false" $SCRATCHPAD/empty`},
		{From: "check-changes", To: "build-project-prompt"},

		// test -> feedback (failure) or review (success)
		{From: "test", To: "build-feedback-prompt", Condition: `[ $EXIT_CODE -ne 0 ]`},
		{From: "test", To: "build-review-prompt"},

		// build-feedback-prompt -> implement
		{From: "build-feedback-prompt", To: "implement"},

		// build-review-prompt -> review
		{From: "build-review-prompt", To: "review"},

		// review -> parse-feedback
		{From: "review", To: "parse-feedback"},

		// parse-feedback -> commit (ACCEPT), implement (REQUEST_CHANGES), reopen (ABANDON)
		{From: "parse-feedback", To: "commit", Condition: `grep -q "^ACCEPT" $SCRATCHPAD/feedback`},
		{From: "parse-feedback", To: "build-feedback-prompt", Condition: `grep -q "^REQUEST_CHANGES" $SCRATCHPAD/feedback`},
		{From: "parse-feedback", To: "reopen-todo", Condition: `grep -q "^ABANDON" $SCRATCHPAD/feedback`},

		// commit -> build-impl-prompt (cycle: next iteration)
		{From: "commit", To: "build-impl-prompt"},

		// build-project-prompt -> project-review
		{From: "build-project-prompt", To: "project-review"},

		// project-review -> parse-project-feedback
		{From: "project-review", To: "parse-project-feedback"},

		// parse-project-feedback -> mark-finished (ACCEPT), implement (REQUEST_CHANGES), reopen (ABANDON)
		{From: "parse-project-feedback", To: "mark-finished", Condition: `grep -q "^ACCEPT" $SCRATCHPAD/feedback`},
		{From: "parse-project-feedback", To: "build-feedback-prompt", Condition: `grep -q "^REQUEST_CHANGES" $SCRATCHPAD/feedback`},
		{From: "parse-project-feedback", To: "reopen-todo", Condition: `grep -q "^ABANDON" $SCRATCHPAD/feedback`},
	},
}
