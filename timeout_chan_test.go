package goproc

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

type TestDeadliner struct {
	time.Time
}

func (t TestDeadliner) Deadline() time.Time {
	return t.Time
}

type TestDeadlinerFactory struct {
	BaseDuration      time.Duration
	RandDurationRange time.Duration
}

func (f *TestDeadlinerFactory) NewRandTestDeadliner() *TestDeadliner {
	return &TestDeadliner{
		Time: time.Now().Add(f.BaseDuration + time.Duration(rand.Int63())%f.RandDurationRange),
	}
}

func TestUnlimitedTimeoutChan(t *testing.T) {
	Convey("With unlimited timeout chan setup", t, func(c C) {
		const testRounds = 1000
		var (
			tc       = NewTimeoutChan(context.Background(), 100*time.Millisecond, 0)
			readCtrl = NewController(context.Background(), t.Name())
			outList  []Deadliner
		)
		readCtrl.Go(func(ctx context.Context) {
			for item := range tc.Out {
				actual := time.Now()
				outList = append(outList, item)
				fmt.Printf("Received item: %s at %s, diff is %+.3fμs\n",
					item.Deadline().Format(time.RFC3339Nano),
					actual.Format(time.RFC3339Nano),
					float64(actual.Sub(item.Deadline()).Nanoseconds())/1e3)
			}
		})

		factory := &TestDeadlinerFactory{
			BaseDuration:      1 * time.Second,
			RandDurationRange: 30 * time.Second,
		}
		for i := 0; i < testRounds; i++ {
			in := factory.NewRandTestDeadliner()
			tc.Push(in)
			fmt.Printf("Sent item: %s\n", in.Format(time.RFC3339Nano))
		}

		Convey("Test clear timeout chan", func() {
			time.Sleep(3 * time.Second)
			c := tc.Clear()
			tc.Shutdown()
			readCtrl.Wait()

			stat := tc.Stats()
			fmt.Println(stat)
			So(stat.Pushed, ShouldEqual, testRounds)
			So(stat.Popped, ShouldBeLessThanOrEqualTo, testRounds-c)
			So(stat.Cleared, ShouldEqual, c)
		})

		Convey("Test shutdown timeout chan", func() {
			tc.Shutdown()
			readCtrl.Wait()

			stat := tc.Stats()
			fmt.Println(stat)
			So(stat.Pushed, ShouldEqual, testRounds)
			So(stat.Popped, ShouldBeLessThanOrEqualTo, testRounds)
			So(stat.Cleared, ShouldEqual, 0)
		})

		Convey("Test wait exit timeout chan", func() {
			tc.Close()
			readCtrl.Wait()

			stat := tc.Stats()
			fmt.Println(stat)
			So(stat.Pushed, ShouldEqual, testRounds)
			So(stat.Popped, ShouldEqual, testRounds)
			So(stat.Cleared, ShouldEqual, 0)

			for i := 1; i < len(outList); i++ {
				So(outList[i-1].Deadline(), ShouldHappenOnOrBefore, outList[i].Deadline())
			}
		})
	})
}

func TestLimitedTimeoutChan(t *testing.T) {
	Convey("With limited timeout chan setup", t, func(c C) {
		const (
			testLimit  = 10
			testRounds = 100
		)
		var (
			tc       = NewTimeoutChan(context.Background(), 100*time.Millisecond, testLimit)
			readCtrl = NewController(context.Background(), t.Name())
			outList  []Deadliner
		)
		readCtrl.Go(func(ctx context.Context) {
			for item := range tc.Out {
				actual := time.Now()
				outList = append(outList, item)
				fmt.Printf("Received item: %s at %s, diff is %+.3fμs\n",
					item.Deadline().Format(time.RFC3339Nano),
					actual.Format(time.RFC3339Nano),
					float64(actual.Sub(item.Deadline()).Nanoseconds())/1e3)
			}
		})

		factory := &TestDeadlinerFactory{
			BaseDuration:      1 * time.Second,
			RandDurationRange: 10 * time.Second,
		}
		inList := make([]Deadliner, testRounds)
		for i := 0; i < testRounds; i++ {
			in := factory.NewRandTestDeadliner()
			tc.Push(in)
			inList[i] = in
			fmt.Printf("Sent item: %s\n", in.Format(time.RFC3339Nano))
		}

		Convey("Test clear timeout chan", func() {
			time.Sleep(3 * time.Second)
			c := tc.Clear()
			tc.Shutdown()
			readCtrl.Wait()

			stat := tc.Stats()
			fmt.Println(stat)
			So(stat.Pushed, ShouldEqual, testRounds)
			So(stat.Popped, ShouldBeLessThanOrEqualTo, testRounds-c)
			So(stat.Cleared, ShouldEqual, c)
		})

		Convey("Test shutdown timeout chan", func() {
			tc.Shutdown()
			readCtrl.Wait()

			stat := tc.Stats()
			fmt.Println(stat)
			So(stat.Pushed, ShouldEqual, testRounds)
			So(stat.Popped, ShouldBeLessThanOrEqualTo, testRounds)
			So(stat.Cleared, ShouldEqual, 0)
		})

		Convey("Test wait exit timeout chan", func() {
			tc.Close()
			readCtrl.Wait()

			stat := tc.Stats()
			fmt.Println(stat)
			So(stat.Pushed, ShouldEqual, testRounds)
			So(stat.Popped, ShouldEqual, testRounds)
			So(stat.Cleared, ShouldEqual, 0)

			for i := range outList {
				// Get min in buffered range
				min := i
				for j := i + 1; j < i+testLimit && j < testRounds; j++ {
					if inList[j].Deadline().UnixNano() < inList[min].Deadline().UnixNano() {
						min = j
					}
				}
				if min != i {
					inList[i], inList[min] = inList[min], inList[i]
				}
				So(outList[i].Deadline(), ShouldEqual, inList[i].Deadline())
			}
		})
	})
}

func TestTimeoutChanReschedule(t *testing.T) {
	Convey("Test TimeoutChan reschedule", t, func(c C) {
		var (
			tc       = NewTimeoutChan(context.Background(), 100*time.Millisecond, 0)
			readCtrl = NewController(context.Background(), t.Name())
			outList  []Deadliner
		)
		readCtrl.Go(func(ctx context.Context) {
			for item := range tc.Out {
				actual := time.Now()
				outList = append(outList, item)
				fmt.Printf("Received item: %s at %s, diff is %+.3fμs\n",
					item.Deadline().Format(time.RFC3339Nano),
					actual.Format(time.RFC3339Nano),
					float64(actual.Sub(item.Deadline()).Nanoseconds())/1e3)
			}
		})

		tc.In <- TestDeadliner{Time: time.Now().Add(10 * time.Second)} // should spin for 5 seconds
		tc.In <- TestDeadliner{Time: time.Now().Add(4 * time.Second)}  // should trigger reschedule to reset spin time to 2 second
		tc.In <- TestDeadliner{Time: time.Now().Add(1 * time.Second)}  // should trigger reschedule to reset spin time to 0.5 second
		tc.Close()
		readCtrl.Wait()

		for i := 1; i < len(outList); i++ {
			So(outList[i-1].Deadline(), ShouldHappenOnOrBefore, outList[i].Deadline())
		}
	})
}

func TestTimeoutChanStarving(t *testing.T) {
	Convey("Test starving", t, func(c C) {
		const testRounds = 10
		var tc = NewTimeoutChan(context.Background(), 100*time.Millisecond, 0)
		defer tc.Close()
		factory := TestDeadlinerFactory{
			BaseDuration:      1 * time.Second,
			RandDurationRange: 5 * time.Second,
		}
		for i := 0; i < testRounds; i++ {
			in := factory.NewRandTestDeadliner()
			tc.In <- in
			fmt.Printf("Sent item: %s\n", in.Format(time.RFC3339Nano))
			out := <-tc.Out
			actual := time.Now()
			fmt.Printf("Received item: %s at %s, diff is %+.3fμs\n",
				out.Deadline().Format(time.RFC3339Nano),
				actual.Format(time.RFC3339Nano),
				float64(actual.Sub(out.Deadline()).Nanoseconds())/1e3)
		}
	})
}

func TestTimeoutChanChaos(t *testing.T) {
	Convey("Test chaos", t, func(c C) {
		const (
			testConcurrency = 10
			testRounds      = 10
		)
		var (
			tc       = NewTimeoutChan(context.Background(), 100*time.Millisecond, 0)
			readCtrl = NewController(context.Background(), t.Name()+"-R")
			outList  []Deadliner
		)
		readCtrl.Go(func(ctx context.Context) {
			for item := range tc.Out {
				actual := time.Now()
				outList = append(outList, item)
				fmt.Printf("Received item: %s at %s, diff is %+.3fμs\n",
					item.Deadline().Format(time.RFC3339Nano),
					actual.Format(time.RFC3339Nano),
					float64(actual.Sub(item.Deadline()).Nanoseconds())/1e3)
			}
		})

		factory := &TestDeadlinerFactory{
			BaseDuration:      1 * time.Second,
			RandDurationRange: 10 * time.Second,
		}
		writeCtrl := NewController(context.Background(), t.Name()+"-W")
		for i := 0; i < testConcurrency; i++ {
			writeCtrl.Go(func(ctx context.Context) {
				for i := 0; i < testRounds; i++ {
					time.Sleep(time.Duration(rand.Int63()) % (1 * time.Second))
					in := factory.NewRandTestDeadliner()
					tc.In <- in
					fmt.Printf("Sent item: %s\n", in.Format(time.RFC3339Nano))
				}
			})
		}
		writeCtrl.Wait()
		tc.Close()
		readCtrl.Wait()

		stat := tc.Stats()
		fmt.Println(stat)
		So(stat.Pushed, ShouldEqual, testConcurrency*testRounds)
		So(stat.Popped, ShouldEqual, testConcurrency*testRounds)
		So(stat.Cleared, ShouldEqual, 0)

		for i := 1; i < len(outList); i++ {
			So(outList[i-1].Deadline(), ShouldHappenOnOrBefore, outList[i].Deadline())
		}
	})
}
