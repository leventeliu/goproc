package goproc

import (
	"container/heap"
	"context"
	"fmt"
	"sync"
	"time"
)

// Deadliner is the interface implemented by an object that can return a deadline time.
type Deadliner interface {
	Deadline() time.Time
}

// TimeoutChanStats contains timeout chan statistics returned from TimeoutChan.Stats().
type TimeoutChanStats struct {
	Pushed  int
	Popped  int
	Cleared int
}

// String implements fmt.Stringer.
func (s TimeoutChanStats) String() string {
	return fmt.Sprintf("TimeoutChanStats: Pushed=%d Popped=%d Cleared=%d", s.Pushed, s.Popped, s.Cleared)
}

// TimeoutChan is a type representing a channel for Deadliner objects.
// TimeoutChan accepts Deadliner from TimeoutChan.In and sends Deadliner to Timeout.Out when its deadline is reached.
type TimeoutChan struct {
	In  chan<- Deadliner
	Out <-chan Deadliner

	ctx        context.Context
	pushCtrl   *BackgroundController
	popCtrl    *BackgroundController
	resolution time.Duration
	limit      int
	in         chan Deadliner
	out        chan Deadliner
	resumePush chan interface{}
	resumePop  chan interface{}
	reschedule chan interface{}
	closePush  chan interface{}

	mu      *sync.RWMutex
	pq      *PriorityQueue
	pushed  int
	popped  int
	cleared int
}

// NewTimeoutChan creates a new TimeoutChan. With 0 limit an unlimited timeout chan will be returned.
func NewTimeoutChan(ctx context.Context, resolution time.Duration, limit int) *TimeoutChan {
	size := limit
	if limit == 0 {
		size = 1024
	}
	in := make(chan Deadliner)
	out := make(chan Deadliner)
	tc := &TimeoutChan{
		In:  in,
		Out: out,

		ctx:        ctx,
		pushCtrl:   NewBackgroundController(ctx, "TimeoutChan Push"),
		popCtrl:    NewBackgroundController(ctx, "TimeoutChan Pop"),
		resolution: resolution,
		limit:      limit,
		in:         in,
		out:        out,
		resumePush: make(chan interface{}),
		resumePop:  make(chan interface{}),
		reschedule: make(chan interface{}),
		closePush:  make(chan interface{}),

		mu:      &sync.RWMutex{},
		pq:      NewPriorityQueue(false, size),
		pushed:  0,
		popped:  0,
		cleared: 0,
	}
	tc.popCtrl.GoBackground(tc.popProcess)
	tc.pushCtrl.GoBackground(tc.pushProcess)
	return tc
}

// Push is an alias of TimeoutChan.In <- (in Deadliner), but bypasses background push process for unlimited TimeoutChan.
func (c *TimeoutChan) Push(in Deadliner) {
	if c.limit == 0 {
		c.push(in)
	} else {
		c.In <- in
	}
}

// Clear clears buffered Deadliners in TimeoutChan.
func (c *TimeoutChan) Clear() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pushCtrl.Shutdown()
	c.popCtrl.Shutdown()
	l := c.pq.Clear()
	if c.limit > 0 && l == c.limit {
		defer func() { c.resumePush <- nil }() // queue is not full, resume
	}
	c.cleared += l
	c.pushCtrl = NewBackgroundController(c.ctx, "TimeoutChan Push")
	c.popCtrl = NewBackgroundController(c.ctx, "TimeoutChan Pop")
	c.popCtrl.GoBackground(c.popProcess)
	c.pushCtrl.GoBackground(c.pushProcess)
	return l
}

// Close closes TimeoutChan and waits until all buffered Deadliners in TimeoutChan to be sent and read in
// TimeoutChan.Out before it returns.
func (c *TimeoutChan) Close() {
	close(c.in)
	c.pushCtrl.WaitExit()
	close(c.closePush)
	c.popCtrl.WaitExit()
	close(c.out)
	close(c.resumePush)
	close(c.resumePop)
	close(c.reschedule)
}

// Close closes TimeoutChan and returns immediately, any buffered Deadliners in TimeoutChan will be ignored.
func (c *TimeoutChan) Shutdown() {
	close(c.in)
	c.pushCtrl.Shutdown()
	close(c.closePush)
	c.popCtrl.Shutdown()
	close(c.out)
	close(c.resumePush)
	close(c.resumePop)
	close(c.reschedule)
}

// Stats returns TimeoutChan statistics.
func (c *TimeoutChan) Stats() TimeoutChanStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return TimeoutChanStats{
		Pushed:  c.pushed,
		Popped:  c.popped,
		Cleared: c.cleared,
	}
}

func (c *TimeoutChan) len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.pq.Len()
}

func (c *TimeoutChan) peek() (<-chan interface{}, time.Duration) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.reschedule, c.pq.Peek().(Deadliner).Deadline().Sub(time.Now())
}

type prioritierWrapper struct {
	Deadliner
}

func (w prioritierWrapper) Priority() int64 {
	return w.Deadline().UnixNano()
}

func (c *TimeoutChan) push(in Deadliner) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pq.Len() == 0 {
		defer func() { c.resumePop <- nil }()
	} else {
		if in.Deadline().Before(c.pq.Peek().(Deadliner).Deadline()) {
			// Most recent deadline changed, send reschedule notice
			defer func() {
				close(c.reschedule)
				c.reschedule = make(chan interface{})
			}()
		}
	}
	heap.Push(c.pq, prioritierWrapper{in})
	c.pushed++
}

func (c *TimeoutChan) pop() Deadliner {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.limit > 0 && c.pq.Len() == c.limit {
		defer func() { c.resumePush <- nil }() // queue is not full, resume
	}
	c.popped++
	return heap.Pop(c.pq).(prioritierWrapper).Deadliner // unwrap
}

func (c *TimeoutChan) pushProcess(ctx context.Context) {
	for {
		// Working phase
		select {
		case in, ok := <-c.in:
			if !ok {
				// End push process because `in` channel is closed and drained
				return
			}
			c.push(in)
			if c.limit == 0 || c.len() < c.limit {
				continue
			} // else queue is full, suspense
		case <-ctx.Done():
			return
		}
		// Suspending phase
		select {
		case <-c.resumePush:
		case <-ctx.Done():
			return
		}
	}
}

func (c *TimeoutChan) popProcess(ctx context.Context) {
	for {
		// Suspending phase
		select {
		case <-c.resumePop:
		case <-c.closePush:
			if c.len() == 0 {
				return
			}
		case <-ctx.Done():
			return
		}
		// Working phase
	outerLoop:
		for {
			// Peeking sub-phase
			reschedule, delta := c.peek()
			if delta <= 0 {
				select {
				case c.out <- c.pop():
					if c.len() == 0 {
						break outerLoop // queue is empty, suspend
					}
				case <-ctx.Done():
					return
				}
			}
			// Spinning sub-phase
			if d := delta / 2; d > c.resolution {
				delta = d
			} else if delta > c.resolution {
				delta = c.resolution
			}
			timer := time.NewTimer(delta)
			select {
			case <-reschedule: // should receive any reschedule after last peek
				if !timer.Stop() {
					<-timer.C
				}
			case <-timer.C:
			case <-ctx.Done():
				return
			}
		}
	}
}
