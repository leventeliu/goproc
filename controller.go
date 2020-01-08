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

// WithValue returns a copy of c with key->value added to internal context object.
// For good practice of context key-value usage, reference context package docs.
//
// Note that you should not treat it as a child BackgroundController - cancelling the returned BackgroundController
// would actually cancel all goroutines controlled by c - it has the same effect as cancelling c.
// This is just a intended, simplified design to allow internal context manipulation, and I may reconsider this later.
func (c *BackgroundController) WithValue(key interface{}, value interface{}) *BackgroundController {
	if err := c.ctx.Err(); err != nil {
		panic(err)
	}
	return &BackgroundController{
		name:   c.name,
		ctx:    context.WithValue(c.ctx, key, value),
		cancel: c.cancel,
		wg:     c.wg,
	}
}

// WithDeadline returns a copy of c with deadline set to internal context object.
//
// Note that you should not treat it as a child BackgroundController - cancelling the returned BackgroundController
// would actually cancel all goroutines controlled by c - it has the same effect as cancelling c.
// This is just a intended, simplified design to allow internal context manipulation, and I may reconsider this later.
func (c *BackgroundController) WithDeadline(deadline time.Time) *BackgroundController {
	if err := c.ctx.Err(); err != nil {
		panic(err)
	}
	var child, _ = context.WithDeadline(c.ctx, deadline)
	return &BackgroundController{
		name:   c.name,
		ctx:    child,
		cancel: c.cancel,
		wg:     c.wg,
	}
}

// WithTimeout returns a copy of c with timeout set to internal context object.
//
// Note that you should not treat it as a child BackgroundController - cancelling the returned BackgroundController
// would actually cancel all goroutines controlled by c - it has the same effect as cancelling c.
// This is just a intended, simplified design to allow internal context manipulation, and I may reconsider this later.
func (c *BackgroundController) WithTimeout(timeout time.Duration) *BackgroundController {
	if err := c.ctx.Err(); err != nil {
		panic(err)
	}
	var child, _ = context.WithTimeout(c.ctx, timeout)
	return &BackgroundController{
		name:   c.name,
		ctx:    child,
		cancel: c.cancel,
		wg:     c.wg,
	}
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
