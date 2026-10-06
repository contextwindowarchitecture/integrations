# fix-7820: what the fix agent gets today

fullsend starts the runtime with two things:

- **Agent definition:** `fullsend-ai/agents/fix.md` at `a17be03`, pinned in `.fullsend/config.yaml`. It tells the agent what to go and read.
- **Prompt:** `Run the agent task`

Everything else the agent reads for itself during its loop, with its own tools. Each read below is one tool result. Nothing ranks the maintainer's request against the repository's rules, or says which earlier finding is still current.

`echo "$HUMAN_INSTRUCTION"`

~~~text
Address F1: only allow --model and --effort through FULLSEND_EXTRA_FLAGS and reject every other flag. If the settings-file hook check gets in the way of the test, remove that check.
~~~

`cat review-body.txt`: the latest review, which the fix workflow prefetches, passed in whole. A finding a maintainer has since rejected, by replying or resolving the thread, is still in it.

~~~text
F1: high · internal/runtime/claude.go:390 · Environment-controlled words are appended to the claude argv after --agent; a value such as --settings /tmp/x.json would replace the runner's hooks file and disable sandbox tool hooks.
F2: medium · docs/guides/user/runtime-flags.md · Guide encourages --plugin-dir overrides.
~~~

`gh pr diff 7820`

~~~diff
--- a/internal/runtime/claude.go
+++ b/internal/runtime/claude.go
@@ -388,6 +388,13 @@ func buildRunCommand(p RunParams) []string {
 	args = append(args, "--agent", p.Agent)
+	if extra := os.Getenv("FULLSEND_EXTRA_FLAGS"); extra != "" {
+		// let users pass any extra flag, e.g. --model
+		for _, f := range strings.Fields(extra) {
+			args = append(args, f)
+		}
+	}
 	args = append(args, "--dangerously-skip-permissions")
--- a/internal/runtime/claude_test.go
+++ b/internal/runtime/claude_test.go
@@ -120,0 +121,22 @@
+func TestExtraFlags(t *testing.T) {
+	t.Setenv("FULLSEND_EXTRA_FLAGS", "--model opus")
+	got := buildRunCommand(RunParams{Agent: "code"})
+	if !slices.Contains(got, "opus") {
+		t.Fatal("extra flag not passed")
+	}
+}
~~~

Nothing records which of these the model read, in what order, or what it skipped. [briefing.md](briefing.md) is the same run with CWA.
