package goproc

import (
	"context"
	"sync"
	"time"
)

// BackgroundFunc defines the background function type for BackgroundController.
type BackgroundFunc func(ctx context.Context)

// BackgroundFunc defines the recover handler function type for BackgroundController.
type RecoverHandleFunc func(r interface{})

// BackgroundController implements a simple controller of background goroutines, which can cancel or wait for all
// under control goroutines to return.
type BackgroundController struct {
	name   string
	ctx    context.Context
	cancel context.CancelFunc
	wg     *sync.WaitGroup
}

// NewBackgroundController creates a new BackgroundController.
func NewBackgroundController(ctx context.Context, name string) *BackgroundController {
	child, cancel := context.WithCancel(ctx)
	return &BackgroundController{
		name:   name,
		ctx:    child,
		cancel: cancel,
		wg:     &sync.WaitGroup{},
	}
}

// GoBackground initiates a new goroutine for f and gains control on the goroutine through a context.Context argument.
func (c *BackgroundController) GoBackground(f BackgroundFunc) {
	if err := c.ctx.Err(); err != nil {
		panic(err)
	}
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		f(c.ctx)
	}()
}

// GoRecoverableBackground initiates a new goroutine for f and gains control on the goroutine through a context.Context
// argument.
// Any panic from f will be captured and handled by h.
func (c *BackgroundController) GoRecoverableBackground(f BackgroundFunc, h RecoverHandleFunc) {
	if err := c.ctx.Err(); err != nil {
		panic(err)
	}
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				h(r)
			}
		}()
		f(c.ctx)
	}()
}

// GoBackgroundWithTimeout initiates a new goroutine for f and gains control on the goroutine through a context.Context
// argument.
// BackgroundController cancels the goroutine after at least a timeout duration.
func (c *BackgroundController) GoBackgroundWithTimeout(f BackgroundFunc, timeout time.Duration) {
	if err := c.ctx.Err(); err != nil {
		panic(err)
	}
	c.wg.Add(1)
	go func() {
		var child, cancel = context.WithTimeout(c.ctx, timeout)
		defer func() {
			c.wg.Done()
			cancel()
		}()
		f(child)
	}()
}

// GoBackgroundWithTimeout initiates a new goroutine for f and gains control on the goroutine through a context.Context
// argument.
// BackgroundController cancels the goroutine after at least a timeout duration.
// Any panic from f will be captured and handled by h.
func (c *BackgroundController) GoRecoverableBackgroundWithTimeout(f BackgroundFunc, h RecoverHandleFunc, timeout time.Duration) {
	if err := c.ctx.Err(); err != nil {
		panic(err)
	}
	c.wg.Add(1)
	go func() {
		var child, cancel = context.WithTimeout(c.ctx, timeout)
		defer func() {
			c.wg.Done()
			cancel()
		}()
		defer func() {
			if r := recover(); r != nil {
				h(r)
			}
		}()
		f(child)
	}()
}

// Shutdown cancels and waits for any goroutine under control.
func (c *BackgroundController) Shutdown() {
	c.cancel()
	c.wg.Wait()
}

// Shutdown waits for any goroutine under control.
func (c *BackgroundController) WaitExit() {
	defer c.cancel()
	c.wg.Wait()
}

// Die tells whether the BackgroundController is already cancelled - it always returns true after the first time
// BackgroundController.Shutdown() or BackgroundController.WaitExit() is called.
func (c *BackgroundController) Die() bool {
	return c.ctx.Err() != nil
}
