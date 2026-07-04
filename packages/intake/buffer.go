package intake

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// ErrBufferDiscarded is returned when an operation is attempted on a TempBuffer
// that has already been discarded.
var ErrBufferDiscarded = errors.New("intake: temp buffer has already been discarded")

// ErrBufferExpired is returned when the TTL of a TempBuffer has elapsed.
var ErrBufferExpired = errors.New("intake: temp buffer TTL has expired")

// TempBuffer provides a TTL-bounded temporary file that is securely zeroed and
// deleted when Discard() is called or when the TTL elapses.  It is safe for
// concurrent use.
type TempBuffer struct {
	mu        sync.Mutex
	file      *os.File
	ExpiresAt time.Time
	discarded bool
	timer     *time.Timer
}

// Create allocates a new TempBuffer backed by an OS temporary file.  The
// buffer will be automatically discarded when ttl elapses even if Discard is
// never called explicitly.  A ttl of zero disables the automatic discard
// timer (callers must call Discard manually).
func Create(ttl time.Duration) (*TempBuffer, error) {
	f, err := os.CreateTemp("", "verdex-intake-*")
	if err != nil {
		return nil, fmt.Errorf("intake: failed to create temp buffer: %w", err)
	}

	tb := &TempBuffer{
		file:      f,
		ExpiresAt: time.Now().Add(ttl),
	}

	if ttl > 0 {
		tb.mu.Lock()
		tb.timer = time.AfterFunc(ttl, func() {
			_ = tb.Discard()
		})
		tb.mu.Unlock()
	}

	return tb, nil
}

// Write appends p to the underlying temporary file.  Returns
// ErrBufferDiscarded or ErrBufferExpired if the buffer is no longer usable.
func (tb *TempBuffer) Write(p []byte) (n int, err error) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	if tb.discarded {
		return 0, ErrBufferDiscarded
	}
	if time.Now().After(tb.ExpiresAt) {
		return 0, ErrBufferExpired
	}
	return tb.file.Write(p)
}

// Reader returns an io.Reader positioned at the beginning of the buffer.
// Returns ErrBufferDiscarded or ErrBufferExpired if the buffer is no longer
// usable.
func (tb *TempBuffer) Reader() (io.Reader, error) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	if tb.discarded {
		return nil, ErrBufferDiscarded
	}
	if time.Now().After(tb.ExpiresAt) {
		return nil, ErrBufferExpired
	}

	if _, err := tb.file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("intake: failed to seek temp buffer: %w", err)
	}
	return tb.file, nil
}

// Discard zeros the contents of the temporary file and then removes it from
// the filesystem.  Calling Discard on an already-discarded buffer is a no-op.
func (tb *TempBuffer) Discard() error {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	if tb.discarded {
		return nil
	}
	tb.discarded = true

	if tb.timer != nil {
		tb.timer.Stop()
	}

	// Zero the file contents before deletion to reduce forensic recovery risk.
	info, statErr := tb.file.Stat()
	if statErr == nil && info.Size() > 0 {
		zeros := make([]byte, min(info.Size(), 32*1024))
		remaining := info.Size()
		if _, seekErr := tb.file.Seek(0, io.SeekStart); seekErr == nil {
			for remaining > 0 {
				chunk := int64(len(zeros))
				if remaining < chunk {
					chunk = remaining
				}
				if _, writeErr := tb.file.Write(zeros[:chunk]); writeErr != nil {
					break
				}
				remaining -= chunk
			}
		}
	}

	name := tb.file.Name()
	_ = tb.file.Close()

	if err := os.Remove(name); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("intake: failed to remove temp buffer %s: %w", name, err)
	}
	return nil
}

// IsDiscarded reports whether Discard has been called on this buffer.
func (tb *TempBuffer) IsDiscarded() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	return tb.discarded
}

// IsExpired reports whether the buffer's TTL has elapsed.
func (tb *TempBuffer) IsExpired() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	return time.Now().After(tb.ExpiresAt)
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
