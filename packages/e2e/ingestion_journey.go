package e2e

import (
	"context"
	"fmt"

	"github.com/YASSERRMD/verdex/packages/ingestion"
	"github.com/YASSERRMD/verdex/packages/stt"
)

// synthAudioPayload is a small, deterministic byte payload used as the
// synthetic audio ingestion.Job.Audio.Data every journey's intake step
// submits -- mirroring packages/perf/benchmark_ingestion_test.go's own
// benchAudioPayload convention. Its content is irrelevant to the noop
// STT provider beyond being non-empty.
var synthAudioPayload = []byte("verdex e2e suite synthetic audio payload, repeated to pad the buffer out. ")

// runIntakePhase drives jobID through f's real
// ingestion.IngestionOrchestrator.Process end-to-end pipeline
// (StageIntake -> StageExtraction -> StageNormalize -> StageSegment ->
// StageClassify), the same real, unmodified orchestrator
// packages/perf's BenchmarkIngestion_Process benchmarks -- this is a
// real code path, not a synthetic stand-in for one (task 2). language
// selects StageNormalize's target language fallback, letting
// multilingual scenario variants (task 4) drive Arabic/Urdu/Tamil/
// English text through the identical real pipeline.
func (f *journeyFixture) runIntakePhase(ctx context.Context, jobID, text, language string) (ingestion.WorkflowState, error) {
	payload := synthAudioPayload
	if text != "" {
		payload = []byte(text)
	}

	job := ingestion.Job{
		JobID:      jobID,
		CaseID:     f.caseID,
		Kind:       ingestion.InputAudio,
		ProviderID: "noop",
		Audio: &stt.AudioInput{
			Data:         append([]byte(nil), payload...),
			MIMEType:     "audio/wav",
			SampleRateHz: 16000,
			Channels:     1,
		},
		Language: language,
	}

	state := f.ingestionOrch.Process(ctx, job)
	if state.Stage != ingestion.StageComplete {
		return state, fmt.Errorf("e2e: runIntakePhase(%s): ingestion did not complete: reached stage %s (%s)", jobID, state.Stage, state.FailureReason)
	}
	return state, nil
}
