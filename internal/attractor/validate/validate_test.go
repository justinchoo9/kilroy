package validate

import (
	"strings"
	"testing"

	"github.com/danshapiro/kilroy/internal/attractor/dot"
	"github.com/danshapiro/kilroy/internal/attractor/model"
	"github.com/danshapiro/kilroy/internal/attractor/modeldb"
)

func TestValidate_StartAndExitNodeRules(t *testing.T) {
	// Missing start node.
	g1, err := dot.Parse([]byte(`digraph G { exit [shape=Msquare] }`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	d1 := Validate(g1)
	assertHasRule(t, d1, "start_node", SeverityError)

	// Missing exit node.
	g2, err := dot.Parse([]byte(`digraph G { start [shape=Mdiamond] }`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	d2 := Validate(g2)
	assertHasRule(t, d2, "terminal_node", SeverityError)
}

func TestValidate_ReachabilityAndEdgeTargets(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  orphan [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  start -> a -> exit
  a -> missing
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	diags := Validate(g)
	assertHasRule(t, diags, "reachability", SeverityError)
	assertHasRule(t, diags, "edge_target_exists", SeverityError)

	// Spec DoD: lint diagnostics include node/edge IDs.
	foundNode := false
	foundEdge := false
	for _, d := range diags {
		if d.Rule == "reachability" && strings.TrimSpace(d.NodeID) != "" {
			foundNode = true
		}
		if d.Rule == "edge_target_exists" && (strings.TrimSpace(d.EdgeFrom) != "" || strings.TrimSpace(d.EdgeTo) != "") {
			foundEdge = true
		}
	}
	if !foundNode {
		t.Fatalf("expected reachability diagnostic to include node_id")
	}
	if !foundEdge {
		t.Fatalf("expected edge_target_exists diagnostic to include edge ids")
	}
}

func TestValidate_StartNoIncomingAndExitNoOutgoing(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  start -> a -> exit
  a -> start
  exit -> a
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	diags := Validate(g)
	assertHasRule(t, diags, "start_no_incoming", SeverityError)
	assertHasRule(t, diags, "exit_no_outgoing", SeverityError)
}

func TestValidate_ConditionAndStylesheetSyntax(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  graph [model_stylesheet="* { llm_provider: openai; } box { llm_model: gpt-5.2; }"]
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  start -> a -> exit
  a -> exit [condition="outcome>success"]
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	diags := Validate(g)
	assertHasRule(t, diags, "condition_syntax", SeverityError)
}

func TestValidate_LLMProviderRequired_Metaspec(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  a [shape=box, llm_model=gpt-5.2]
  start -> a -> exit
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	diags := Validate(g)
	assertHasRule(t, diags, "llm_provider_required", SeverityError)
}

func TestValidate_ToolCommandRequired_ParallelogramWithToolCommand_NoError(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  t [shape=parallelogram, tool_command="echo ok"]
  start -> t -> exit
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	diags := Validate(g)
	assertNoRule(t, diags, "tool_command_required")
}

func TestValidate_ToolCommandRequired_ParallelogramWithCommandOnly_Error(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  t [shape=parallelogram, command="echo ok"]
  start -> t -> exit
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	diags := Validate(g)
	assertHasRule(t, diags, "tool_command_required", SeverityError)
}

func TestValidate_ToolCommandRequired_TypeToolRequiresToolCommand(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  t [shape=box, type=tool, prompt="ignored for tool"]
  start -> t -> exit
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	diags := Validate(g)
	assertHasRule(t, diags, "tool_command_required", SeverityError)
}

func TestValidate_PromptOnCodergenNodes_WarnsWhenMissingPrompt(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  start -> a -> exit
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	diags := Validate(g)
	found := false
	for _, d := range diags {
		if d.Rule == "prompt_on_llm_nodes" && d.Severity == SeverityWarning && d.NodeID == "a" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected prompt_on_llm_nodes WARNING for node a; got %+v", diags)
	}
}

func TestValidate_LoopRestartFailureEdgeRequiresTransientInfraGuard(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="x"]
  check [shape=diamond]
  start -> a -> check
  check -> a [condition="outcome=fail", loop_restart=true]
  check -> exit [condition="outcome=success"]
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	diags := Validate(g)
	assertHasRule(t, diags, "loop_restart_failure_class_guard", SeverityWarning)
}

func TestValidate_LoopRestartFailureEdgeWithTransientInfraGuard_NoWarning(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="x"]
  check [shape=diamond]
  pm [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="postmortem"]
  start -> a -> check
  check -> a [condition="outcome=fail && context.failure_class=transient_infra", loop_restart=true]
  check -> pm [condition="outcome=fail && context.failure_class!=transient_infra"]
  check -> exit [condition="outcome=success"]
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	diags := Validate(g)
	for _, d := range diags {
		if d.Rule == "loop_restart_failure_class_guard" {
			t.Fatalf("unexpected loop_restart_failure_class_guard warning: %+v", d)
		}
	}
}

func TestValidate_LoopRestartOnUnconditionalEdge_Warns(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="x"]
  b [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="y"]
  start -> a -> b
  b -> a [loop_restart=true]
  a -> exit [condition="outcome=success"]
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	diags := Validate(g)
	assertHasRule(t, diags, "loop_restart_failure_class_guard", SeverityWarning)
}

func TestValidate_LoopRestartTransientGuardWithoutDeterministicFallback_Warns(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="x"]
  check [shape=diamond]
  start -> a -> check
  check -> a [condition="outcome=fail && context.failure_class=transient_infra", loop_restart=true]
  check -> exit [condition="outcome=success"]
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	diags := Validate(g)
	assertHasRule(t, diags, "loop_restart_failure_class_guard", SeverityWarning)
}

func TestValidate_LoopRestartTransientCompanionIsNotDeterministicFallback_Warns(t *testing.T) {
	// A non-restart companion edge that is ALSO guarded by transient_infra
	// does not satisfy the "non-restart deterministic fail edge" requirement.
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="x"]
  check [shape=diamond]
  pm [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="postmortem"]
  start -> a -> check
  check -> a [condition="outcome=fail && context.failure_class=transient_infra", loop_restart=true]
  check -> pm [condition="outcome=fail && context.failure_class=transient_infra"]
  check -> exit [condition="outcome=success"]
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	diags := Validate(g)
	assertHasRule(t, diags, "loop_restart_failure_class_guard", SeverityWarning)
}

func TestValidate_LoopRestartPartialSuccessCompanionIsNotDeterministicFallback_Warns(t *testing.T) {
	// A non-restart companion conditioned on outcome=partial_success does not
	// route outcome=fail traffic, so it is not a valid deterministic fallback.
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="x"]
  check [shape=diamond]
  pm [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="postmortem"]
  start -> a -> check
  check -> a [condition="outcome=fail && context.failure_class=transient_infra", loop_restart=true]
  check -> pm [condition="outcome=partial_success"]
  check -> exit [condition="outcome=success"]
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	diags := Validate(g)
	assertHasRule(t, diags, "loop_restart_failure_class_guard", SeverityWarning)
}

func TestValidate_FailLoopFailureClassGuard_WarnsWhenBackEdgeUnguarded(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit [shape=Msquare]
  impl [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="x"]
  check [shape=diamond]
  start -> impl -> check
  check -> impl [condition="outcome=fail"]
  check -> exit [condition="outcome=success"]
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	diags := Validate(g)
	assertHasRule(t, diags, "fail_loop_failure_class_guard", SeverityWarning)
}

func TestValidate_FailLoopFailureClassGuard_NoWarningWhenFailureClassGuarded(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit [shape=Msquare]
  impl [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="x"]
  check [shape=diamond]
  start -> impl -> check
  check -> impl [condition="outcome=fail && context.failure_class=transient_infra"]
  check -> postmortem [condition="outcome=fail && context.failure_class!=transient_infra"]
  postmortem [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="p"]
  check -> exit [condition="outcome=success"]
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	diags := Validate(g)
	for _, d := range diags {
		if d.Rule == "fail_loop_failure_class_guard" {
			t.Fatalf("unexpected fail_loop_failure_class_guard warning: %+v", d)
		}
	}
}

func TestValidate_EscalationModelsSyntax_Valid_NoWarning(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="x", escalation_models="kimi:kimi-k2.5, anthropic:claude-opus-4-6"]
  start -> a -> exit
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	diags := Validate(g)
	for _, d := range diags {
		if d.Rule == "escalation_models_syntax" {
			t.Fatalf("unexpected escalation_models_syntax warning for valid entries: %+v", d)
		}
	}
}

func TestValidate_EscalationModelsSyntax_MissingColon(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="x", escalation_models="kimi-kimi-k2.5"]
  start -> a -> exit
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	diags := Validate(g)
	assertHasRule(t, diags, "escalation_models_syntax", SeverityWarning)
}

func TestValidate_EscalationModelsSyntax_EmptyProvider(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="x", escalation_models=":some-model"]
  start -> a -> exit
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	diags := Validate(g)
	assertHasRule(t, diags, "escalation_models_syntax", SeverityWarning)
}

func TestValidate_ShapeAliases_DownstreamLintsFireForCircleAndDoublecircle(t *testing.T) {
	// circle=start, doublecircle=exit aliases should be recognized by downstream lints
	// (start_no_incoming, exit_no_outgoing, reachability) not just lintStartNode/lintExitNode.
	g, err := dot.Parse([]byte(`
digraph G {
  s [shape=circle]
  e [shape=doublecircle]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="x"]
  s -> a -> e
  a -> s
  e -> a
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	diags := Validate(g)
	// Should fire start_no_incoming (a->s) and exit_no_outgoing (e->a).
	assertHasRule(t, diags, "start_no_incoming", SeverityError)
	assertHasRule(t, diags, "exit_no_outgoing", SeverityError)

	// Reachability should also work — all nodes reachable, so no reachability errors.
	for _, d := range diags {
		if d.Rule == "reachability" {
			t.Fatalf("unexpected reachability error for fully connected alias-shaped graph: %+v", d)
		}
	}
}

func TestValidate_GoalGateExitStatusContract_ErrorAndPromptWarning(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  graph [retry_target=implement]
  start [shape=Mdiamond]
  exit [shape=Msquare]
  implement [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="x"]
  review_consensus [
    shape=box,
    goal_gate=true,
    llm_provider=openai,
    llm_model=gpt-5.2,
    prompt="Review and decide outcome.\nWrite status JSON with outcome=pass when approved."
  ]
  start -> review_consensus
  review_consensus -> exit [condition="outcome=pass"]
  review_consensus -> implement [condition="outcome=retry"]
  implement -> exit [condition="outcome=success"]
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	diags := Validate(g)
	assertHasRule(t, diags, "goal_gate_exit_status_contract", SeverityError)
	assertHasRule(t, diags, "goal_gate_prompt_status_hint", SeverityWarning)
}

func TestValidate_GoalGateExitStatusContract_AllowsCanonicalSuccessOutcomes(t *testing.T) {
	tests := []struct {
		name         string
		exitOutcome  string
		promptStatus string
	}{
		{
			name:         "success",
			exitOutcome:  "success",
			promptStatus: "success",
		},
		{
			name:         "partial_success",
			exitOutcome:  "partial_success",
			promptStatus: "partial_success",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dotSrc := `
digraph G {
  graph [retry_target=implement]
  start [shape=Mdiamond]
  exit [shape=Msquare]
  implement [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="x"]
  review_consensus [
    shape=box,
    goal_gate=true,
    llm_provider=openai,
    llm_model=gpt-5.2,
    prompt="Review and decide outcome.\nWrite status JSON with outcome=` + tc.promptStatus + ` when approved."
  ]
  start -> review_consensus
  review_consensus -> exit [condition="outcome=` + tc.exitOutcome + `"]
  review_consensus -> implement [condition="outcome=retry"]
  implement -> exit [condition="outcome=success"]
}
`
			g, err := dot.Parse([]byte(dotSrc))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			diags := Validate(g)
			assertNoRule(t, diags, "goal_gate_exit_status_contract")
			assertNoRule(t, diags, "goal_gate_prompt_status_hint")
		})
	}
}

func TestValidate_GoalGateExitStatusContract_NoTerminalMismatchNoError(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  graph [retry_target=implement]
  start [shape=Mdiamond]
  exit [shape=Msquare]
  implement [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="x"]
  review_consensus [
    shape=box,
    goal_gate=true,
    llm_provider=openai,
    llm_model=gpt-5.2,
    prompt="Review and decide outcome.\nWrite status JSON with outcome=success when approved."
  ]
  review_router [shape=diamond]
  start -> review_consensus
  review_consensus -> review_router [condition="outcome=pass"]
  review_router -> exit [condition="outcome=success"]
  review_consensus -> implement [condition="outcome=retry"]
  implement -> exit [condition="outcome=success"]
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	diags := Validate(g)
	assertNoRule(t, diags, "goal_gate_exit_status_contract")
	assertNoRule(t, diags, "goal_gate_prompt_status_hint")
}

func TestValidate_TemplateProvenancePostmortemRouting_WarnsOnNeedsReplanToNonPlanningEntry(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  graph [provenance_version="1"]
  start [shape=Mdiamond]
  exit [shape=Msquare]
  postmortem [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="pm"]
  implement [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="impl"]
  debate_consolidate [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="debate"]
  start -> postmortem
  postmortem -> debate_consolidate [condition="outcome=needs_replan"]
  postmortem -> implement
  implement -> exit [condition="outcome=success"]
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	diags := Validate(g)
	assertHasRule(t, diags, "template_postmortem_replan_entry", SeverityWarning)
}

func TestValidate_TemplateProvenancePostmortemRouting_NoWarningWithoutProvenance(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit [shape=Msquare]
  postmortem [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="pm"]
  implement [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="impl"]
  debate_consolidate [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="debate"]
  check_toolchain [shape=parallelogram, tool_command="echo ok"]
  start -> postmortem
  postmortem -> debate_consolidate [condition="outcome=needs_replan"]
  postmortem -> check_toolchain
  postmortem -> implement
  implement -> exit [condition="outcome=success"]
  check_toolchain -> implement
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	diags := Validate(g)
	assertNoRule(t, diags, "template_postmortem_replan_entry")
	assertNoRule(t, diags, "template_postmortem_broad_rollback")
}

func TestValidate_TemplateProvenancePostmortemRouting_WarnsOnUnconditionalToolchainRollback(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  graph [provenance_version="1"]
  start [shape=Mdiamond]
  exit [shape=Msquare]
  postmortem [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="pm"]
  implement [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="impl"]
  plan_fanout [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="plan"]
  check_toolchain [shape=parallelogram, tool_command="echo ok"]
  start -> postmortem
  postmortem -> plan_fanout [condition="outcome=needs_replan"]
  postmortem -> check_toolchain
  postmortem -> implement
  implement -> exit [condition="outcome=success"]
  plan_fanout -> implement
  check_toolchain -> implement
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	diags := Validate(g)
	assertHasRule(t, diags, "template_postmortem_broad_rollback", SeverityWarning)
	assertNoRule(t, diags, "template_postmortem_replan_entry")
}

func TestValidate_TemplateProvenancePostmortemRouting_WarnsWhenMissingNeedsReplanRoute(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  graph [provenance_version="1"]
  start [shape=Mdiamond]
  exit [shape=Msquare]
  postmortem [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="pm"]
  implement [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="impl"]
  start -> postmortem
  postmortem -> implement [condition="outcome=impl_repair"]
  postmortem -> implement
  implement -> exit [condition="outcome=success"]
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	diags := Validate(g)
	assertHasRule(t, diags, "template_postmortem_replan_entry", SeverityWarning)
}

func assertHasRule(t *testing.T, diags []Diagnostic, rule string, sev Severity) {
	t.Helper()
	for _, d := range diags {
		if d.Rule == rule && d.Severity == sev {
			return
		}
	}
	var got []string
	for _, d := range diags {
		got = append(got, string(d.Severity)+":"+d.Rule)
	}
	t.Fatalf("expected %s:%s; got %s", sev, rule, strings.Join(got, ", "))
}

func assertNoRule(t *testing.T, diags []Diagnostic, rule string) {
	t.Helper()
	for _, d := range diags {
		if d.Rule == rule {
			t.Fatalf("unexpected diagnostic %s:%s (%s)", d.Severity, d.Rule, d.Message)
		}
	}
}

// --- Tests for V7.2: type_known lint rule ---

func TestValidate_TypeKnownRule_RecognizedType_NoWarning(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit [shape=Msquare]
  a [shape=box, type=codergen, llm_provider=openai, llm_model=gpt-5.2, prompt="x"]
  start -> a -> exit
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	rule := NewTypeKnownRule([]string{"codergen", "conditional", "start", "exit"})
	diags := Validate(g, rule)
	assertNoRule(t, diags, "type_known")
}

func TestValidate_TypeKnownRule_UnrecognizedType_Warning(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit [shape=Msquare]
  a [shape=box, type=unknown_handler, llm_provider=openai, llm_model=gpt-5.2, prompt="x"]
  start -> a -> exit
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	rule := NewTypeKnownRule([]string{"codergen", "conditional", "start", "exit"})
	diags := Validate(g, rule)
	assertHasRule(t, diags, "type_known", SeverityWarning)
}

func TestValidate_TypeKnownRule_NoTypeOverride_NoWarning(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="x"]
  start -> a -> exit
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	rule := NewTypeKnownRule([]string{"codergen"})
	diags := Validate(g, rule)
	assertNoRule(t, diags, "type_known")
}

// --- Tests for V7.3/V7.4: LintRule interface and extra_rules ---

type testLintRule struct {
	name string
	diag Diagnostic
}

func (r *testLintRule) Name() string                      { return r.name }
func (r *testLintRule) Apply(g *model.Graph) []Diagnostic { return []Diagnostic{r.diag} }

func TestValidate_ExtraRules_AreAppendedToBuiltInRules(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="x"]
  start -> a -> exit
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	custom := &testLintRule{
		name: "custom_test_rule",
		diag: Diagnostic{Rule: "custom_test_rule", Severity: SeverityInfo, Message: "test"},
	}
	diags := Validate(g, custom)
	assertHasRule(t, diags, "custom_test_rule", SeverityInfo)
}

func TestValidate_ExtraRules_NilRulesIgnored(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="x"]
  start -> a -> exit
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Should not panic on nil rules.
	_ = Validate(g, nil)
}

// --- Tests for V7.5: ValidateOrError collects all errors ---

func TestValidateOrError_CollectsAllErrors(t *testing.T) {
	// Graph with multiple validation errors: no start, no exit, edge to missing node.
	g, err := dot.Parse([]byte(`
digraph G {
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  a -> missing
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	vErr := ValidateOrError(g)
	if vErr == nil {
		t.Fatal("expected validation error")
	}
	msg := vErr.Error()
	// Should contain multiple errors, not just the first one.
	if !strings.Contains(msg, "start_node") {
		t.Fatalf("expected start_node error in message: %s", msg)
	}
	if !strings.Contains(msg, "terminal_node") {
		t.Fatalf("expected terminal_node error in message: %s", msg)
	}
	if !strings.Contains(msg, "edge_target_exists") {
		t.Fatalf("expected edge_target_exists error in message: %s", msg)
	}
}

// --- Tests for V7.6: at-least-one exit node ---

func TestValidate_MultipleExitNodes_NoError(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  success_exit [shape=Msquare]
  error_exit [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="x"]
  start -> a
  a -> success_exit [condition="outcome=success"]
  a -> error_exit [condition="outcome=fail"]
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	diags := Validate(g)
	assertNoRule(t, diags, "terminal_node")
}

func TestValidate_ZeroExitNodes_Error(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  start -> a
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	diags := Validate(g)
	assertHasRule(t, diags, "terminal_node", SeverityError)
}

func TestValidate_MultipleExitNodes_ExitNoOutgoingChecksAll(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit1 [shape=Msquare]
  exit2 [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="x"]
  start -> a
  a -> exit1 [condition="outcome=success"]
  a -> exit2 [condition="outcome=fail"]
  exit2 -> a
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	diags := Validate(g)
	// exit2 has an outgoing edge, so exit_no_outgoing should fire.
	assertHasRule(t, diags, "exit_no_outgoing", SeverityError)
	// Verify the diagnostic points at exit2, not exit1.
	for _, d := range diags {
		if d.Rule == "exit_no_outgoing" && d.NodeID == "exit2" {
			return
		}
	}
	t.Fatal("expected exit_no_outgoing diagnostic for exit2")
}

// --- Tests for G12: stylesheet model ID catalog validation ---

// buildTestCatalog creates a minimal in-memory catalog for testing.
func buildTestCatalog() *modeldb.Catalog {
	return &modeldb.Catalog{
		Models: map[string]modeldb.ModelEntry{
			"anthropic/claude-opus-4.6": {Provider: "anthropic"},
			"anthropic/claude-sonnet-4.5": {Provider: "anthropic"},
			"openai/gpt-5.2":              {Provider: "openai"},
		},
		CoveredProviders: map[string]bool{
			"anthropic": true,
			"openai":    true,
		},
	}
}

// minimalGraph returns a valid minimal graph with a model_stylesheet attribute set.
func minimalGraphWithStylesheet(stylesheet string) []byte {
	return []byte(`digraph G {
  graph [model_stylesheet="` + stylesheet + `"]
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="x"]
  start -> a -> exit
}`)
}

// TestValidate_G12_UnknownModelID_EmitsWarning checks that a completely unknown
// model ID in the stylesheet produces a stylesheet_unknown_model WARNING.
func TestValidate_G12_UnknownModelID_EmitsWarning(t *testing.T) {
	g, err := dot.Parse(minimalGraphWithStylesheet(`* { llm_provider: anthropic; llm_model: claude-opus-999-nonexistent; }`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	catalog := buildTestCatalog()
	diags := ValidateWithOptions(g, ValidateOptions{Catalog: catalog})
	assertHasRule(t, diags, "stylesheet_unknown_model", SeverityWarning)
}

// TestValidate_G12_DashedAnthropicModelID_EmitsNonCanonicalWarning checks that
// claude-opus-4-6 (dashed version number) produces a stylesheet_noncanonical_model_id
// WARNING because the catalog canonical form is claude-opus-4.6 (dotted).
func TestValidate_G12_DashedAnthropicModelID_EmitsNonCanonicalWarning(t *testing.T) {
	g, err := dot.Parse(minimalGraphWithStylesheet(`* { llm_provider: anthropic; llm_model: claude-opus-4-6; }`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	catalog := buildTestCatalog()
	diags := ValidateWithOptions(g, ValidateOptions{Catalog: catalog})
	assertHasRule(t, diags, "stylesheet_noncanonical_model_id", SeverityWarning)
	assertNoRule(t, diags, "stylesheet_unknown_model")
}

// TestValidate_G12_CanonicalModelID_NoWarning checks that a valid canonical model
// ID in the stylesheet produces no stylesheet_unknown_model or
// stylesheet_noncanonical_model_id warnings.
func TestValidate_G12_CanonicalModelID_NoWarning(t *testing.T) {
	g, err := dot.Parse(minimalGraphWithStylesheet(`* { llm_provider: anthropic; llm_model: claude-opus-4.6; }`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	catalog := buildTestCatalog()
	diags := ValidateWithOptions(g, ValidateOptions{Catalog: catalog})
	assertNoRule(t, diags, "stylesheet_unknown_model")
	assertNoRule(t, diags, "stylesheet_noncanonical_model_id")
}

// TestValidate_G12_NoStylesheet_NoWarning checks that when no model_stylesheet is
// present the catalog check produces no diagnostics.
func TestValidate_G12_NoStylesheet_NoWarning(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="x"]
  start -> a -> exit
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	catalog := buildTestCatalog()
	diags := ValidateWithOptions(g, ValidateOptions{Catalog: catalog})
	assertNoRule(t, diags, "stylesheet_unknown_model")
	assertNoRule(t, diags, "stylesheet_noncanonical_model_id")
}

// TestValidate_G12_NilCatalog_NoWarning checks that when no catalog is provided
// the model ID checks are silently skipped.
func TestValidate_G12_NilCatalog_NoWarning(t *testing.T) {
	g, err := dot.Parse(minimalGraphWithStylesheet(`* { llm_provider: anthropic; llm_model: claude-totally-bogus; }`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	diags := ValidateWithOptions(g, ValidateOptions{Catalog: nil})
	assertNoRule(t, diags, "stylesheet_unknown_model")
	assertNoRule(t, diags, "stylesheet_noncanonical_model_id")
}

// TestValidate_G12_UnknownProvider_NoWarning checks that when the catalog does not
// cover the provider, no unknown-model warning is emitted (the catalog has no opinion).
func TestValidate_G12_UnknownProvider_NoWarning(t *testing.T) {
	g, err := dot.Parse(minimalGraphWithStylesheet(`* { llm_provider: cerebras; llm_model: llama-4-scout; }`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	catalog := buildTestCatalog() // cerebras not covered
	diags := ValidateWithOptions(g, ValidateOptions{Catalog: catalog})
	assertNoRule(t, diags, "stylesheet_unknown_model")
	assertNoRule(t, diags, "stylesheet_noncanonical_model_id")
}
