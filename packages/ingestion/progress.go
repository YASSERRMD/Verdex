package ingestion

import "sync"

// stagePercent maps each pipeline stage to the cumulative percent-complete
// once that stage has finished, used to compute Progress.PercentComplete.
var stagePercent = map[Stage]int{
	StageIntake:     20,
	StageExtraction: 40,
	StageNormalize:  60,
	StageSegment:    80,
	StageClassify:   95,
	StageComplete:   100,
	StageFailed:     0,
}

// PercentCompleteForStage returns the cumulative percent-complete
// associated with a job currently sitting at stage. Unrecognized stages
// return 0.
func PercentCompleteForStage(stage Stage) int {
	return stagePercent[stage]
}

// Progress is a point-in-time snapshot of a job's advancement through the
// pipeline.
type Progress struct {
	JobID           string
	Stage           Stage
	PercentComplete int
}

// ProgressReporter tracks per-job progress and lets callers subscribe to
// updates as they happen.
//
// Implementations must be safe for concurrent use.
type ProgressReporter interface {
	// Report records a new Progress snapshot for a job and notifies any
	// active subscribers for that job.
	Report(p Progress)

	// Get returns the latest reported Progress for jobID, and ok=false if
	// no progress has been reported yet.
	Get(jobID string) (p Progress, ok bool)

	// Subscribe returns a channel that receives every subsequent Progress
	// reported for jobID, and an unsubscribe function the caller must call
	// to release resources once it stops reading. The channel is buffered
	// so Report never blocks on a slow/absent subscriber; if the buffer
	// fills, the oldest unread update is dropped in favor of the newest.
	Subscribe(jobID string) (ch <-chan Progress, unsubscribe func())
}

// subscriberBuffer is the channel capacity given to each Subscribe caller.
const subscriberBuffer = 16

// InMemoryProgressReporter is a map-backed ProgressReporter.
type InMemoryProgressReporter struct {
	mu          sync.Mutex
	latest      map[string]Progress
	subscribers map[string]map[int]chan Progress
	nextSubID   int
}

// NewInMemoryProgressReporter constructs an empty InMemoryProgressReporter.
func NewInMemoryProgressReporter() *InMemoryProgressReporter {
	return &InMemoryProgressReporter{
		latest:      make(map[string]Progress),
		subscribers: make(map[string]map[int]chan Progress),
	}
}

// Report implements ProgressReporter.
func (r *InMemoryProgressReporter) Report(p Progress) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.latest[p.JobID] = p
	for _, ch := range r.subscribers[p.JobID] {
		select {
		case ch <- p:
		default:
			// Drop the oldest queued update to make room, then retry once.
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- p:
			default:
			}
		}
	}
}

// Get implements ProgressReporter.
func (r *InMemoryProgressReporter) Get(jobID string) (Progress, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.latest[jobID]
	return p, ok
}

// Subscribe implements ProgressReporter.
func (r *InMemoryProgressReporter) Subscribe(jobID string) (<-chan Progress, func()) {
	r.mu.Lock()
	defer r.mu.Unlock()

	ch := make(chan Progress, subscriberBuffer)
	if r.subscribers[jobID] == nil {
		r.subscribers[jobID] = make(map[int]chan Progress)
	}
	id := r.nextSubID
	r.nextSubID++
	r.subscribers[jobID][id] = ch

	unsubscribe := func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		if subs, ok := r.subscribers[jobID]; ok {
			delete(subs, id)
			if len(subs) == 0 {
				delete(r.subscribers, jobID)
			}
		}
	}
	return ch, unsubscribe
}
