package reasoningtrace_test

import (
	"context"
	"time"

	"github.com/YASSERRMD/verdex/packages/agentframework"
	"github.com/YASSERRMD/verdex/packages/evidenceweighing"
	"github.com/YASSERRMD/verdex/packages/firstpartyagent"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/issueagent"
	"github.com/YASSERRMD/verdex/packages/lawapplication"
	"github.com/YASSERRMD/verdex/packages/provider"
	"github.com/YASSERRMD/verdex/packages/reasoningorchestration"
	"github.com/YASSERRMD/verdex/packages/secondpartyagent"
	"github.com/YASSERRMD/verdex/packages/synthesisagent"
)

const testCaseID = "case-reasoningtrace"

func newTestUser(roles ...identity.Role) *identity.User {
	return &identity.User{
		Email:     "test@example.com",
		Name:      "Test User",
		Roles:     roles,
		Status:    identity.UserStatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func authedContext() context.Context {
	return identity.WithUser(context.Background(), newTestUser(identity.RoleAdvocate))
}

func unauthedContext() context.Context {
	return context.Background()
}

// buildAgentResult constructs a minimal, realistic agentframework.Result
// for testCaseID/agentName with one step that called toolName and
// concluded with finalText. Every field a StageStep/RetrievalEvent reads
// is populated so tests exercise the real extraction logic rather than
// zero values.
func buildAgentResult(t interface {
	Fatalf(string, ...any)
}, agentName, toolName string, args map[string]any, resultContent, finalText string,
) agentframework.Result {
	pad, err := agentframework.NewScratchpad(testCaseID, "tenant-1")
	if err != nil {
		t.Fatalf("NewScratchpad: %v", err)
	}
	started := time.Now()
	pad.AppendStep(agentframework.Step{
		Index:   0,
		Request: provider.ChatRequest{Messages: []provider.Message{{Role: "user", Content: "analyze"}}},
		Response: &provider.ChatResponse{
			ID:      "resp-0",
			Content: "tool call",
		},
		Decision: agentframework.Decision{
			ToolCalls: []agentframework.ToolCall{{Name: toolName, Args: args}},
		},
		Observations: []agentframework.Observation{
			{
				Call:   agentframework.ToolCall{Name: toolName, Args: args},
				Result: agentframework.ToolResult{Content: resultContent},
			},
		},
		StartedAt: started,
		EndedAt:   started.Add(50 * time.Millisecond),
	})
	pad.AppendStep(agentframework.Step{
		Index:     1,
		Request:   provider.ChatRequest{Messages: []provider.Message{{Role: "user", Content: "conclude"}}},
		Response:  &provider.ChatResponse{ID: "resp-1", Content: finalText},
		Decision:  agentframework.Decision{Conclude: true, FinalText: finalText},
		StartedAt: started.Add(50 * time.Millisecond),
		EndedAt:   started.Add(80 * time.Millisecond),
	})

	return agentframework.Result{
		CaseID:      testCaseID,
		AgentName:   agentName,
		Termination: agentframework.TerminationConcluded,
		FinalText:   finalText,
		Scratchpad:  pad,
		Telemetry:   agentframework.Stats{StepsTaken: 2, ToolCallsMade: 1, ModelCalls: 2, Termination: agentframework.TerminationConcluded},
	}
}

// seededStore returns an InMemoryCheckpointStore pre-populated with a
// complete, realistic run for testCaseID: every stage from issue framing
// through the guardrail check, with the four LLM-backed stages carrying
// a populated agentframework.Result whose Scratchpad records a
// resolve_citation/get_node tool call.
func seededStore(t interface {
	Fatalf(string, ...any)
}) reasoningorchestration.CheckpointStore {
	ctx := context.Background()
	store := reasoningorchestration.NewInMemoryCheckpointStore()

	issueRun := buildAgentResult(t, "issue-agent", agentframework.ToolGetNode,
		map[string]any{"node_id": "issue-1"}, "issue-1: contract formation", "Framed 1 issue.")
	issueCP := reasoningorchestration.Checkpoint{
		Stage: reasoningorchestration.StageIssueFraming,
		IssueAnalysis: issueagent.IssueAnalysisResult{
			CaseID: testCaseID,
			Issues: []issueagent.FramedIssue{
				{SourceIssueNodeID: "issue-1", Question: "Was the contract validly formed?", MaterialityRank: 1},
			},
		},
		IssueFramingRun: issueRun,
	}

	fpRun := buildAgentResult(t, "first-party-agent", agentframework.ToolSearchCaseKnowledge,
		map[string]any{"query": "written memorandum"}, "fact-1: signed memorandum", "Constructed 1 argument.")
	fpCP := reasoningorchestration.Checkpoint{
		Stage: reasoningorchestration.StageFirstPartyArguments,
		FirstPartyArguments: firstpartyagent.ArgumentSet{
			CaseID:  testCaseID,
			PartyID: "plaintiff",
			Arguments: []firstpartyagent.Argument{
				{ID: "arg-1", IssueNodeID: "issue-1", PartyID: "plaintiff", Claim: "The contract was validly formed.", SupportingFactIDs: []string{"fact-1"}},
			},
		},
		FirstPartyRun: fpRun,
	}

	spRun := buildAgentResult(t, "second-party-agent", agentframework.ToolResolveCitation,
		map[string]any{"node_id": "rule-1"}, "rule-1: Statute of Frauds", "Constructed 1 rebuttal.")
	spCP := reasoningorchestration.Checkpoint{
		Stage: reasoningorchestration.StageSecondPartyArguments,
		SecondPartyArguments: secondpartyagent.ArgumentSet{
			CaseID:  testCaseID,
			PartyID: "defendant",
			Arguments: []secondpartyagent.Argument{
				{ID: "arg-2", IssueNodeID: "issue-1", PartyID: "defendant", Claim: "No signed writing exists."},
			},
		},
		SecondPartyRun: spRun,
	}

	evCP := reasoningorchestration.Checkpoint{
		Stage: reasoningorchestration.StageEvidenceWeighing,
		Evidence: evidenceweighing.Result{
			CaseID:      testCaseID,
			FactWeights: []evidenceweighing.FactWeight{{FactNodeID: "fact-1", Weight: 0.75}},
		},
	}

	lawCP := reasoningorchestration.Checkpoint{
		Stage: reasoningorchestration.StageLawApplication,
		Law: lawapplication.Result{
			CaseID: testCaseID,
			IssueApplications: []lawapplication.IssueApplication{
				{
					IssueNodeID:        "issue-1",
					ControllingRuleIDs: []string{"rule-1"},
					Citations: []lawapplication.AppliedCitation{
						{RuleID: "rule-1", Citation: "Cal. Civ. Code § 1624", Resolved: true, Verified: true},
					},
				},
			},
		},
	}

	synthRun := buildAgentResult(t, "synthesis-agent", agentframework.ToolValidationStatus,
		map[string]any{}, "validated", "Synthesized 1 conclusion.")
	synthCP := reasoningorchestration.Checkpoint{
		Stage: reasoningorchestration.StageSynthesis,
		Opinion: synthesisagent.Opinion{
			CaseID: testCaseID,
			Conclusions: []synthesisagent.TentativeConclusion{
				{
					IssueNodeID:       "issue-1",
					Text:              "The evidence favors the plaintiff.",
					FavoredParty:      "plaintiff",
					Confidence:        0.7,
					SupportingFactIDs: []string{"fact-1"},
					SupportingRuleIDs: []string{"rule-1"},
					Grounded:          true,
				},
			},
			GeneratedAt: time.Now(),
		},
		SynthesisRun: synthRun,
	}

	uncCP := reasoningorchestration.Checkpoint{Stage: reasoningorchestration.StageUncertaintySurfacing}
	guardCP := reasoningorchestration.Checkpoint{Stage: reasoningorchestration.StageGuardrailCheck, GuardrailApproved: true}

	for _, cp := range []reasoningorchestration.Checkpoint{issueCP, fpCP, spCP, evCP, lawCP, synthCP, uncCP, guardCP} {
		if err := store.SaveCheckpoint(ctx, testCaseID, cp); err != nil {
			t.Fatalf("SaveCheckpoint(%q): %v", cp.Stage, err)
		}
	}

	state := reasoningorchestration.RunState{
		CaseID:       testCaseID,
		CurrentStage: reasoningorchestration.StageComplete,
		CompletedStages: []reasoningorchestration.Stage{
			reasoningorchestration.StageIssueFraming,
			reasoningorchestration.StageFirstPartyArguments,
			reasoningorchestration.StageSecondPartyArguments,
			reasoningorchestration.StageEvidenceWeighing,
			reasoningorchestration.StageLawApplication,
			reasoningorchestration.StageSynthesis,
			reasoningorchestration.StageUncertaintySurfacing,
			reasoningorchestration.StageGuardrailCheck,
		},
		Termination: reasoningorchestration.TerminationComplete,
		StartedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := store.SaveRunState(ctx, state); err != nil {
		t.Fatalf("SaveRunState: %v", err)
	}

	return store
}
