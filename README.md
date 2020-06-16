# goproc

[![GoDoc](https://godoc.org/github.com/leventeliu/goproc?status.svg)](https://godoc.org/github.com/leventeliu/goproc)

goproc is a golang module containing components for goroutine and timer control.

## Basic Components

### BackgroundController

A simple controller of background goroutines, which can cancel or wait for all under control goroutines to return.

### TimeoutChan

A type representing a channel for Deadliner objects. TimeoutChan accepts Deadliner from TimeoutChan.In and sends Deadliner to Timeout.Out when its deadline is reached.

The underlying implementation of TimeoutChan timer scheduling is similar to the internal [golang timer](https://blog.gopheracademy.com/advent-2016/go-timers/) but with a higher level abstraction and better-controlled behavior.

**Features**:

- Channel-like behavior with limited/unlimited buffer
- Deadliner management and timeout scheduling
- Guaranteed out-order of deadliners in TimeoutChan buffer
  - While working with limited TimeoutChan, the order is only guaranteed in the limited buffer range

See [example test cases](timeout_chan_test.go) for details.
