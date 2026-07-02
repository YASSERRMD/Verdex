package ingestion

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/ocr"
	"github.com/YASSERRMD/verdex/packages/stt"
)

// newTestOrchestrator wires an IngestionOrchestrator using isolated
// (per-test) stt/ocr registries with the deterministic no-op providers
// registered, so extraction is real (goes through
// stt.STTService.Transcribe / ocr.OCRService.Extract) but fully
// deterministic and offline.
func newTestOrchestrator(t *testing.T) *IngestionOrchestrator {
	t.Helper()

	sttRegistry := stt.NewRegistry()
	if err := sttRegistry.Register("noop", stt.DefaultNoOpSTTProvider()); err != nil {
		t.Fatalf("register stt noop provider: %v", err)
	}
	ocrRegistry := ocr.NewRegistry()
	if err := ocrRegistry.Register("noop", ocr.DefaultNoOpOCRProvider()); err != nil {
		t.Fatalf("register ocr noop provider: %v", err)
	}

	return NewIngestionOrchestrator(OrchestratorConfig{
		STT: stt.NewSTTService(sttRegistry, nil, nil),
		OCR: ocr.NewOCRService(ocrRegistry, nil, nil, nil),
	})
}

func TestOrchestrator_Process_EndToEnd_Audio(t *testing.T) {
	o := newTestOrchestrator(t)
	job := newAudioJob("job-e2e-audio", "case-1")

	state := o.Process(context.Background(), job)

	if state.Stage != StageComplete {
		t.Fatalf("Stage = %s, want %s (reason: %s)", state.Stage, StageComplete, state.FailureReason)
	}
	if len(state.Segments) == 0 {
		t.Fatal("expected at least one classified segment")
	}
	for _, cs := range state.Segments {
		if cs.Segment.ID == "" {
			t.Error("classified segment missing Segment.ID")
		}
		if cs.Classification.SegmentID != cs.Segment.ID {
			t.Errorf("Classification.SegmentID = %q, want %q", cs.Classification.SegmentID, cs.Segment.ID)
		}
	}

	// Status API reflects the completed state.
	status, err := NewIngestionStatusAPI(o.Status).GetStatus(job.JobID)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if status.Stage != StageComplete {
		t.Errorf("status.Stage = %s, want %s", status.Stage, StageComplete)
	}

	// Progress reflects 100%.
	prog, ok := o.Progress.Get(job.JobID)
	if !ok || prog.PercentComplete != 100 {
		t.Errorf("progress = %+v, ok=%v; want 100%%", prog, ok)
	}

	// No dead-letter entry for a successful job.
	if _, ok := o.DeadLetters.Get(job.JobID); ok {
		t.Error("unexpected dead-letter entry for a successful job")
	}

	// Recovery checkpoint cleared on success.
	if _, ok := o.Recovery.Load(job.JobID); ok {
		t.Error("expected recovery checkpoint to be cleared on success")
	}
}

func TestOrchestrator_Process_EndToEnd_Image(t *testing.T) {
	o := newTestOrchestrator(t)
	job := newImageJob("job-e2e-image", "case-2")

	state := o.Process(context.Background(), job)

	if state.Stage != StageComplete {
		t.Fatalf("Stage = %s, want %s (reason: %s)", state.Stage, StageComplete, state.FailureReason)
	}
	if len(state.Segments) == 0 {
		t.Fatal("expected at least one classified segment")
	}
}

func TestOrchestrator_Process_InvalidJob_MissingInput(t *testing.T) {
	o := newTestOrchestrator(t)
	job := Job{JobID: "job-invalid", CaseID: "case-1", Kind: InputAudio} // Audio is nil

	state := o.Process(context.Background(), job)

	if state.Stage != StageFailed {
		t.Fatalf("Stage = %s, want %s", state.Stage, StageFailed)
	}
	if state.FailureReason == "" {
		t.Error("expected a non-empty FailureReason")
	}

	dl, ok := o.DeadLetters.Get(job.JobID)
	if !ok {
		t.Fatal("expected job to be dead-lettered")
	}
	if dl.Stage != StageExtraction {
		t.Errorf("dead-letter Stage = %s, want %s", dl.Stage, StageExtraction)
	}
}

// flakyOnceSTTProvider fails the first N calls then delegates to a real
// NoOpSTTProvider, used to exercise the retry-then-succeed path through
// the full orchestrator (not just RunWithRetry in isolation).
type flakyOnceSTTProvider struct {
	failuresLeft int
	inner        stt.STTProvider
}

func (f *flakyOnceSTTProvider) ID() string { return f.inner.ID() }
func (f *flakyOnceSTTProvider) Capabilities() stt.Capability {
	return f.inner.Capabilities()
}
func (f *flakyOnceSTTProvider) Transcribe(ctx context.Context, input stt.AudioInput) (*stt.Transcript, error) {
	if f.failuresLeft > 0 {
		f.failuresLeft--
		return nil, errors.New("simulated transient provider failure")
	}
	return f.inner.Transcribe(ctx, input)
}

func TestOrchestrator_Process_RetriesTransientExtractionFailure(t *testing.T) {
	sttRegistry := stt.NewRegistry()
	flaky := &flakyOnceSTTProvider{failuresLeft: 1, inner: stt.DefaultNoOpSTTProvider()}
	if err := sttRegistry.Register("noop", flaky); err != nil {
		t.Fatalf("register: %v", err)
	}
	ocrRegistry := ocr.NewRegistry()
	if err := ocrRegistry.Register("noop", ocr.DefaultNoOpOCRProvider()); err != nil {
		t.Fatalf("register: %v", err)
	}

	o := NewIngestionOrchestrator(OrchestratorConfig{
		STT:         stt.NewSTTService(sttRegistry, nil, nil),
		OCR:         ocr.NewOCRService(ocrRegistry, nil, nil, nil),
		RetryPolicy: RetryPolicy{MaxAttempts: 3},
	})

	job := newAudioJob("job-flaky", "case-1")
	state := o.Process(context.Background(), job)

	if state.Stage != StageComplete {
		t.Fatalf("Stage = %s, want %s (reason: %s)", state.Stage, StageComplete, state.FailureReason)
	}

	rec, ok := o.Idempotency.Get(job.JobID, StageExtraction)
	if !ok || !rec.Completed {
		t.Errorf("idempotency record = %+v, ok=%v; want Completed=true", rec, ok)
	}
	if rec.Attempts != 2 {
		t.Errorf("Attempts = %d, want 2 (one failure + one success)", rec.Attempts)
	}
}

// alwaysFailSTTProvider fails every call, used to exercise the
// dead-letter-after-exhausted-retries path.
type alwaysFailSTTProvider struct{ inner stt.STTProvider }

func (f alwaysFailSTTProvider) ID() string                   { return f.inner.ID() }
func (f alwaysFailSTTProvider) Capabilities() stt.Capability { return f.inner.Capabilities() }
func (f alwaysFailSTTProvider) Transcribe(context.Context, stt.AudioInput) (*stt.Transcript, error) {
	return nil, errors.New("permanent provider failure")
}

func TestOrchestrator_Process_DeadLettersAfterExhaustedRetries(t *testing.T) {
	sttRegistry := stt.NewRegistry()
	if err := sttRegistry.Register("noop", alwaysFailSTTProvider{inner: stt.DefaultNoOpSTTProvider()}); err != nil {
		t.Fatalf("register: %v", err)
	}
	ocrRegistry := ocr.NewRegistry()
	if err := ocrRegistry.Register("noop", ocr.DefaultNoOpOCRProvider()); err != nil {
		t.Fatalf("register: %v", err)
	}

	o := NewIngestionOrchestrator(OrchestratorConfig{
		STT:         stt.NewSTTService(sttRegistry, nil, nil),
		OCR:         ocr.NewOCRService(ocrRegistry, nil, nil, nil),
		RetryPolicy: RetryPolicy{MaxAttempts: 2},
	})

	job := newAudioJob("job-doomed", "case-1")
	state := o.Process(context.Background(), job)

	if state.Stage != StageFailed {
		t.Fatalf("Stage = %s, want %s", state.Stage, StageFailed)
	}

	dl, ok := o.DeadLetters.Get(job.JobID)
	if !ok {
		t.Fatal("expected job to be dead-lettered")
	}
	if dl.Stage != StageExtraction {
		t.Errorf("dead-letter Stage = %s, want %s", dl.Stage, StageExtraction)
	}
	if dl.Attempts != 2 {
		t.Errorf("dead-letter Attempts = %d, want 2", dl.Attempts)
	}

	// Recovery checkpoint retained so a manual Resume could be attempted
	// (even though this permanently-failing provider would fail again). The
	// checkpoint records the actual stage that exhausted retries
	// (StageExtraction), not the terminal StageFailed reported via
	// Status/the returned WorkflowState -- that's what lets Resume know
	// which stage to re-attempt.
	checkpoint, ok := o.Recovery.Load(job.JobID)
	if !ok {
		t.Fatal("expected a recovery checkpoint to remain for a dead-lettered job")
	}
	if checkpoint.Stage != StageExtraction {
		t.Errorf("checkpoint.Stage = %s, want %s", checkpoint.Stage, StageExtraction)
	}
}

// resumableSTTProvider fails until Fixed is flipped to true, letting a
// test simulate "the transient problem was fixed out of band" before
// calling Resume.
type resumableSTTProvider struct {
	fail  bool
	inner stt.STTProvider
}

func (f *resumableSTTProvider) ID() string { return f.inner.ID() }
func (f *resumableSTTProvider) Capabilities() stt.Capability {
	return f.inner.Capabilities()
}
func (f *resumableSTTProvider) Transcribe(ctx context.Context, input stt.AudioInput) (*stt.Transcript, error) {
	if f.fail {
		return nil, errors.New("provider still down")
	}
	return f.inner.Transcribe(ctx, input)
}

func TestOrchestrator_Resume_ContinuesFromFailedStage(t *testing.T) {
	sttRegistry := stt.NewRegistry()
	provider := &resumableSTTProvider{fail: true, inner: stt.DefaultNoOpSTTProvider()}
	if err := sttRegistry.Register("noop", provider); err != nil {
		t.Fatalf("register: %v", err)
	}
	ocrRegistry := ocr.NewRegistry()
	if err := ocrRegistry.Register("noop", ocr.DefaultNoOpOCRProvider()); err != nil {
		t.Fatalf("register: %v", err)
	}

	o := NewIngestionOrchestrator(OrchestratorConfig{
		STT:         stt.NewSTTService(sttRegistry, nil, nil),
		OCR:         ocr.NewOCRService(ocrRegistry, nil, nil, nil),
		RetryPolicy: RetryPolicy{MaxAttempts: 1},
	})

	job := newAudioJob("job-resume", "case-1")

	state := o.Process(context.Background(), job)
	if state.Stage != StageFailed {
		t.Fatalf("initial Stage = %s, want %s", state.Stage, StageFailed)
	}
	if _, ok := o.DeadLetters.Get(job.JobID); !ok {
		t.Fatal("expected initial failure to be dead-lettered")
	}

	// Simulate the transient provider issue being fixed, then resume.
	provider.fail = false

	resumed, err := o.Resume(context.Background(), job)
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if resumed.Stage != StageComplete {
		t.Fatalf("resumed Stage = %s, want %s (reason: %s)", resumed.Stage, StageComplete, resumed.FailureReason)
	}
	if len(resumed.Segments) == 0 {
		t.Error("expected classified segments after successful resume")
	}
}

func TestOrchestrator_Resume_NotResumable(t *testing.T) {
	o := newTestOrchestrator(t)
	job := newAudioJob("job-never-run", "case-1")

	_, err := o.Resume(context.Background(), job)
	if !errors.Is(err, ErrNotResumable) {
		t.Errorf("err = %v, want ErrNotResumable", err)
	}
}

func TestOrchestrator_Run_ProcessesQueuedJobs(t *testing.T) {
	o := newTestOrchestrator(t)
	o.Queue = NewInMemoryJobQueue(4)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go o.Run(ctx)

	job := newAudioJob("job-queued", "case-1")
	if err := o.Queue.Enqueue(job); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if st, ok := o.Status.Get(job.JobID); ok && st.Stage.IsTerminal() {
			if st.Stage != StageComplete {
				t.Fatalf("Stage = %s, want %s", st.Stage, StageComplete)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job was not processed by Run within the deadline")
}
