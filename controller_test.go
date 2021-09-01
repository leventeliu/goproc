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
		ctrl := NewController(context.Background(), t.Name())
		Convey("Test wait exit", func() {
			ctrl.WithTimeout(3 * time.Second).
				Go(hangingAround).
				Wait()
			So(ctrl.Die(), ShouldBeTrue)
		})
		Convey("Test shutdown", func() {
			ctrl.Go(hangingAround).Shutdown()
			So(ctrl.Die(), ShouldBeTrue)
		})
		Convey("Test passing value", func() {
			ctrl.WithDeadline(time.Now().Add(3*time.Second)).
				WithValue(hangingAroundKey1, "Let's play!").
				Go(hangingAround).
				Wait()
			So(ctrl.Die(), ShouldBeTrue)
		})
		Convey("Test passing value with anonymous key", func() {
			ctrl.WithTimeout(3*time.Second).
				WithValue(&testKeyType{}, "panic").
				Go(hangingAround).
				Wait() // should not receive key
			So(ctrl.Die(), ShouldBeTrue)
		})
	})
}
