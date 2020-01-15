package goproc

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

type testKeyType struct{ int }

var (
	hangingAroundKey1 = &testKeyType{}
)

func hangingAround(ctx context.Context) {
	fmt.Printf("Start hanging around...\n")
	if value := ctx.Value(hangingAroundKey1); value != nil {
		fmt.Printf("Found a passed value through context: %+v\n", value)
		if value == "panic" {
			panic(value)
		}
	}
	<-ctx.Done()
	fmt.Printf("Stop hanging around because... %+v!\n", ctx.Err())
}

func TestBackgroundController(t *testing.T) {
	Convey("With test controller created", t, func(c C) {
		ctrl := NewBackgroundController(context.Background(), t.Name())
		Convey("Test wait exit", func() {
			defer ctrl.WaitExit()
			ctrl.WithTimeout(3 * time.Second).GoBackground(hangingAround)
		})
		Convey("Test shutdown", func() {
			ctrl.GoBackground(hangingAround)
			ctrl.Shutdown()
		})
		Convey("Test passing value", func() {
			defer ctrl.WaitExit()
			ctrl.WithDeadline(time.Now().Add(3*time.Second)).
				WithValue(hangingAroundKey1, "Let's play!").
				GoBackground(hangingAround)
		})
		Convey("Test passing value with anonymous key", func() {
			defer ctrl.WaitExit()
			ctrl.WithTimeout(3*time.Second).WithValue(&testKeyType{}, "panic").GoBackground(hangingAround) // should not receive key
		})
	})
}
