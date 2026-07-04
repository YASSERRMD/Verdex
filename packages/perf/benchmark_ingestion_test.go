package perf

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/YASSERRMD/verdex/packages/ingestion"
	"github.com/YASSERRMD/verdex/packages/stt"
)

// benchAudioPayload is a small, deterministic byte payload used as the
// synthetic audio Job.Audio.Data every ingestion benchmark iteration
// submits. Its content is irrelevant to the noop STT provider beyond
// being non-empty (stt.NoOpSTTProvider.Transcribe rejects empty Data).
var benchAudioPayload = []byte("verdex perf benchmark synthetic audio payload, repeated to pad length. ")

// newBenchIngestionOrchestrator builds an IngestionOrchestrator wired with
// an isolated (per-call) stt.Registry carrying the deterministic
// stt.DefaultNoOpSTTProvider registered under "noop", mirroring
// packages/ingestion's own newTestOrchestrator helper
// (orchestrator_test.go): extraction is real (goes through
// stt.STTService.Transcribe), but fully deterministic and offline.
//
// A registry must be built explicitly here rather than relying on
// stt.DefaultRegistry (which NewIngestionOrchestrator falls back to when
// OrchestratorConfig.STT is left nil): DefaultRegistry starts empty in this
// package's own test binary -- nothing self-registers a "noop" provider
// into it at init time -- so a benchmark job with ProviderID "noop" would
// otherwise fail extraction with "provider not found".
func newBenchIngestionOrchestrator(tb testing.TB) *ingestion.IngestionOrchestrator {
	tb.Helper()

	sttRegistry := stt.NewRegistry()
	if err := sttRegistry.Register("noop", stt.DefaultNoOpSTTProvider()); err != nil {
		tb.Fatalf("register stt noop provider: %v", err)
	}

	return ingestion.NewIngestionOrchestrator(ingestion.OrchestratorConfig{
		STT: stt.NewSTTService(sttRegistry, nil, nil),
	})
}

// benchAudioJob builds a real, valid audio ingestion.Job with the given
// jobID.
func benchAudioJob(jobID string) ingestion.Job {
	return ingestion.Job{
		JobID:      jobID,
		CaseID:     benchCaseID,
		Kind:       ingestion.InputAudio,
		ProviderID: "noop",
		Audio: &stt.AudioInput{
			Data:         append([]byte(nil), benchAudioPayload...),
			MIMEType:     "audio/wav",
			SampleRateHz: 16000,
			Channels:     1,
		},
		Language: "en",
	}
}

// BenchmarkIngestion_Process benchmarks the real, unmodified
// ingestion.IngestionOrchestrator.Process end-to-end pipeline (StageIntake
// -> StageExtraction -> StageNormalize -> StageSegment -> StageClassify)
// over a synthetic audio Job. Every stage genuinely executes against
// packages/intake/packages/stt/packages/multilingual/packages/segmentation/
// packages/evidence's real (no-op-provider-backed) service
// implementations -- this is a real code path, not a synthetic stand-in.
//
// Each b.N iteration uses a fresh JobID (idempotency/recovery state is
// keyed by JobID) and a fresh IngestionOrchestrator (Process's internal
// per-job scratch state is cleared after StageComplete, but a fresh
// orchestrator per iteration avoids any possibility of cross-iteration
// state leakage skewing the measurement).
func BenchmarkIngestion_Process(b *testing.B) {
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		orch := newBenchIngestionOrchestrator(b)
		job := benchAudioJob(fmt.Sprintf("bench-job-%d", i))

		state := orch.Process(ctx, job)
		if state.Stage != ingestion.StageComplete {
			b.Fatalf("expected StageComplete, got %s (failure: %s)", state.Stage, state.FailureReason)
		}
	}
}

// BenchmarkIngestion_ProcessParallel runs BenchmarkIngestion_Process's same
// real Process() call under b.RunParallel, giving each parallel goroutine
// its own IngestionOrchestrator (Process's scratch map is guarded by its
// own mutex and is safe for concurrent use across jobs, but a distinct
// orchestrator per goroutine here isolates each goroutine's Idempotency/
// Recovery/Status default in-memory stores, avoiding any lock contention
// between them from skewing the measurement).
func BenchmarkIngestion_ProcessParallel(b *testing.B) {
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()

	var goroutineSeq int64
	b.RunParallel(func(pb *testing.PB) {
		goroutineID := atomic.AddInt64(&goroutineSeq, 1)
		orch := newBenchIngestionOrchestrator(b)

		var localCounter int
		for pb.Next() {
			localCounter++
			job := benchAudioJob(fmt.Sprintf("bench-parallel-job-%d-%d", goroutineID, localCounter))

			state := orch.Process(ctx, job)
			if state.Stage != ingestion.StageComplete {
				b.Fatalf("expected StageComplete, got %s (failure: %s)", state.Stage, state.FailureReason)
			}
		}
	})
}
