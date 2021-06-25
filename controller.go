package goproc

import (
	"context"
	"sync"
	"time"
)

// Goroutine defines the function type for Controller.
type Goroutine func(ctx context.Context)

// Recover defines the recover handler function type for Controller.
type Recover func(r interface{})

// Controller implements a simple controller of goroutines, which can cancel
// or wait for all under control goroutines to return.
type Controller struct {
	name   string
	ctx    context.Context
	cancel context.CancelFunc
	wg     *sync.WaitGroup
}

// NewController creates a new Controller.
func NewController(ctx context.Context, name string) *Controller {
	child, cancel := context.WithCancel(ctx)
	return &Controller{
		name:   name,
		ctx:    child,
		cancel: cancel,
		wg:     &sync.WaitGroup{},
	}
}

// Go initiates a new goroutine for f and gains control on the goroutine through
// a context.Context argument.
func (c *Controller) Go(g Goroutine) *Controller {
	if err := c.ctx.Err(); err != nil {
		panic(err)
	}
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		g(c.ctx)
	}()
	return c
}

// GoWithRecover initiates a new goroutine for f and gains control on the goroutine
// through a context.Context argument.
// Any panic from f will be captured and handled by h.
func (c *Controller) GoWithRecover(g Goroutine, rf Recover) *Controller {
	if err := c.ctx.Err(); err != nil {
		panic(err)
	}
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				rf(r)
			}
		}()
		g(c.ctx)
	}()
	return c
}

// WithValue returns a copy of c with key->value added to internal context object, which will be
// passed to the Goroutine functions in subsequent c.Go* calls.
// For good practice of context key-value usage, reference context package docs.
//
// Note that unlike a child context, the returned object still holds the control of c, which means
// cancelling the returned Controller would actually cancel all goroutines started by c.
func (c *Controller) WithValue(key interface{}, value interface{}) *Controller {
	if err := c.ctx.Err(); err != nil {
		panic(err)
	}
	return &Controller{
		name:   c.name,
		ctx:    context.WithValue(c.ctx, key, value),
		cancel: c.cancel,
		wg:     c.wg,
	}
}

// WithDeadline returns a copy of c with deadline set to internal context object, which will be
// passed to the Goroutine functions in subsequent c.Go* calls.
//
// Note that unlike a child context, the returned object still holds the control of c, which means
// cancelling the returned Controller would actually cancel all goroutines started by c.
func (c *Controller) WithDeadline(deadline time.Time) *Controller {
	if err := c.ctx.Err(); err != nil {
		panic(err)
	}
	var child, _ = context.WithDeadline(c.ctx, deadline)
	return &Controller{
		name:   c.name,
		ctx:    child,
		cancel: c.cancel,
		wg:     c.wg,
	}
}

// WithTimeout returns a copy of c with timeout set to internal context object, which will be
// passed to the Goroutine functions in subsequent c.Go* calls.
//
// Note that unlike a child context, the returned object still holds the control of c, which means
// cancelling the returned Controller would actually cancel all goroutines started by c.
func (c *Controller) WithTimeout(timeout time.Duration) *Controller {
	if err := c.ctx.Err(); err != nil {
		panic(err)
	}
	var child, _ = context.WithTimeout(c.ctx, timeout)
	return &Controller{
		name:   c.name,
		ctx:    child,
		cancel: c.cancel,
		wg:     c.wg,
	}
}

// Shutdown cancels and waits for any goroutine under control.
func (c *Controller) Shutdown() {
	c.cancel()
	c.wg.Wait()
}

// Wait waits for any goroutine under control to exit.
func (c *Controller) Wait() {
	defer c.cancel()
	c.wg.Wait()
}

// Die tells whether c is already cancelled - it always returns true after the first time
// c.Shutdown() or c.Wait() is called.
func (c *Controller) Die() bool {
	return c.ctx.Err() != nil
}
