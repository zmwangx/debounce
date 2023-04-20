package debounce_test

import (
	"sync/atomic"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	. "github.com/zmwangx/debounce"
)

const ms = time.Millisecond

func TestDebounceWithCustomSignature(t *testing.T) {
	Convey("DebounceWithCustomSignature", t, func() {
		// The following tests were ported from
		// https://github.com/lodash/lodash/blob/master/test/debounce-and-throttle.js.

		Convey("should support cancelling delayed calls", func() {
			callCount := 0

			debounced, control := DebounceWithCustomSignature(func(args ...interface{}) interface{} {
				callCount++
				return nil
			}, 32*ms, WithLeading(false))

			debounced()
			control.Cancel()

			time.Sleep(64 * ms)
			So(callCount, ShouldEqual, 0)
		})

		Convey("should reset `lastCalled` after cancelling", func() {
			callCount := 0

			debounced, control := DebounceWithCustomSignature(func(args ...interface{}) int {
				callCount++
				return callCount
			}, 32*ms, WithLeading(true))

			So(debounced(), ShouldEqual, 1)
			control.Cancel()

			So(debounced(), ShouldEqual, 2)
			debounced()

			time.Sleep(64 * ms)
			So(callCount, ShouldEqual, 3)
		})

		Convey("should support flushing delayed calls", func() {
			callCount := 0

			debounced, control := DebounceWithCustomSignature(func(args ...interface{}) int {
				callCount++
				return callCount
			}, 32*ms, WithLeading(false))

			debounced()
			So(control.Flush(), ShouldEqual, 1)

			time.Sleep(64 * ms)
			So(callCount, ShouldEqual, 1)
		})

		Convey("should noop `cancel` and `flush` when nothing is queued", func() {
			callCount := 0

			_, control := DebounceWithCustomSignature(func(args ...interface{}) interface{} {
				callCount++
				return nil
			}, 32*ms)

			control.Cancel()
			So(control.Flush(), ShouldBeNil)

			time.Sleep(64 * ms)
			So(callCount, ShouldEqual, 0)
		})

		// --------------------------------------------------------------------

		// The following tests were ported from
		// https://github.com/lodash/lodash/blob/master/test/debounce.test.js.

		Convey("should debounce a function", func() {
			callCount := 0

			debounced, _ := DebounceWithCustomSignature(func(args ...string) string {
				value := args[0]
				callCount++
				return value
			}, 32*ms)

			results := []string{debounced("a"), debounced("b"), debounced("c")}
			So(results, ShouldResemble, []string{"", "", ""})
			So(callCount, ShouldEqual, 0)

			time.Sleep(128 * ms)
			So(callCount, ShouldEqual, 1)
			results = []string{debounced("d"), debounced("e"), debounced("f")}
			So(results, ShouldResemble, []string{"c", "c", "c"})
			So(callCount, ShouldEqual, 1)

			time.Sleep(128 * ms)
			So(callCount, ShouldEqual, 2)
		})

		Convey("subsequent debounced calls return the last `func` result", func() {
			debounced, _ := DebounceWithCustomSignature(func(args ...string) string {
				value := args[0]
				return value
			}, 32*ms)
			debounced("a")

			time.Sleep(64 * ms)
			So(debounced("b"), ShouldNotEqual, "b")

			time.Sleep(64 * ms)
			So(debounced("c"), ShouldNotEqual, "c")
		})

		Convey("should apply default options", func() {
			callCount := 0

			debounced, _ := DebounceWithCustomSignature(func(args ...interface{}) interface{} {
				callCount++
				return nil
			}, 32*ms)

			debounced()
			So(callCount, ShouldEqual, 0)

			time.Sleep(64 * ms)
			So(callCount, ShouldEqual, 1)
		})

		Convey("should support a `leading` option", func() {
			callCounts := []int{0, 0}

			withLeading, _ := DebounceWithCustomSignature(func(args ...interface{}) interface{} {
				callCounts[0]++
				return nil
			}, 32*ms, WithLeading(true))

			withLeadingAndTrailing, _ := DebounceWithCustomSignature(func(args ...interface{}) interface{} {
				callCounts[1]++
				return nil
			}, 32*ms, WithLeading(true))

			withLeading()
			So(callCounts[0], ShouldEqual, 1)

			withLeadingAndTrailing()
			withLeadingAndTrailing()
			So(callCounts[1], ShouldEqual, 1)

			time.Sleep(64 * ms)
			So(callCounts, ShouldResemble, []int{1, 2})
			withLeading()
			So(callCounts[0], ShouldEqual, 2)
		})

		Convey("subsequent leading debounced calls return the last `func` result", func() {
			debounced, _ := DebounceWithCustomSignature(func(args ...string) string {
				value := args[0]
				return value
			}, 32*ms, WithLeading(true), WithTrailing(false))

			results := []string{debounced("a"), debounced("b")}
			So(results, ShouldResemble, []string{"a", "a"})

			time.Sleep(64 * ms)
			results = []string{debounced("c"), debounced("d")}
			So(results, ShouldResemble, []string{"c", "c"})
		})

		Convey("should support a `trailing` option", func() {
			withCount := 0
			withoutCount := 0

			withTrailing, _ := DebounceWithCustomSignature(func(args ...interface{}) interface{} {
				withCount++
				return nil
			}, 32*ms, WithTrailing(true))

			withoutTrailing, _ := DebounceWithCustomSignature(func(args ...interface{}) interface{} {
				withoutCount++
				return nil
			}, 32*ms, WithTrailing(false))

			withTrailing()
			So(withCount, ShouldEqual, 0)

			withoutTrailing()
			So(withoutCount, ShouldEqual, 0)

			time.Sleep(64 * ms)
			So(withCount, ShouldEqual, 1)
			So(withoutCount, ShouldEqual, 0)
		})

		Convey("should support a `maxWait` option", func() {
			callCount := 0

			debounced, _ := DebounceWithCustomSignature(func(args ...interface{}) interface{} {
				callCount++
				return nil
			}, 32*ms, WithMaxWait(64*ms))

			debounced()
			debounced()
			So(callCount, ShouldEqual, 0)

			time.Sleep(128 * ms)
			So(callCount, ShouldEqual, 1)
			debounced()
			debounced()
			So(callCount, ShouldEqual, 1)

			time.Sleep(128 * ms)
			So(callCount, ShouldEqual, 2)
		})

		Convey("should support `maxWait` in a tight loop", func() {
			limit := 320 * ms
			var withCount int64
			var withoutCount int64

			withMaxWait, _ := DebounceWithCustomSignature(func(args ...interface{}) interface{} {
				atomic.AddInt64(&withCount, 1)
				return nil
			}, 64*ms, WithMaxWait(128*ms))

			withoutMaxWait, _ := DebounceWithCustomSignature(func(args ...interface{}) interface{} {
				atomic.AddInt64(&withoutCount, 1)
				return nil
			}, 96*ms)

			start := time.Now()
			for time.Since(start) < limit {
				withMaxWait()
				withoutMaxWait()
			}
			So(withCount, ShouldBeGreaterThan, 0)
			So(withoutCount, ShouldEqual, 0)
		})

		Convey("should queue a trailing call for subsequent debounced calls after `maxWait`", func() {
			callCount := 0

			debounced, _ := DebounceWithCustomSignature(func(args ...interface{}) interface{} {
				callCount++
				return nil
			}, 200*ms, WithMaxWait(200*ms))

			debounced()

			time.Sleep(190 * ms)
			debounced()
			time.Sleep(10 * ms)
			debounced()
			time.Sleep(10 * ms)
			debounced()

			time.Sleep(300 * ms)
			So(callCount, ShouldEqual, 2)
		})

		Convey("should cancel `maxDelayed` when `delayed` is invoked", func() {
			callCount := 0

			debounced, _ := DebounceWithCustomSignature(func(args ...interface{}) interface{} {
				callCount++
				return nil
			}, 32*ms, WithMaxWait(64*ms))

			debounced()

			time.Sleep(128 * ms)
			debounced()
			So(callCount, ShouldEqual, 1)

			time.Sleep(64 * ms)
			So(callCount, ShouldEqual, 2)
		})

		Convey("should invoke the trailing call with the correct arguments", func() {
			callCount := 0
			var calledArgs []interface{}

			debounced, _ := DebounceWithCustomSignature(func(args ...interface{}) bool {
				callCount++
				calledArgs = args
				return callCount != 2
			}, 32*ms, WithLeading(true), WithMaxWait(64*ms))

			for {
				if !debounced("a", "b") {
					break
				}
			}
			time.Sleep(64 * ms)
			So(callCount, ShouldEqual, 2)
			So(calledArgs, ShouldResemble, []interface{}{"a", "b"})
		})

		// --------------------------------------------------------------------

		// Additional tests

		Convey("should be thread-safe", func() {
			var callCount int64

			debounced, _ := DebounceWithCustomSignature(func(args ...interface{}) interface{} {
				atomic.AddInt64(&callCount, 1)
				return nil
			}, 32*ms)

			tightLoop := func() {
				start := time.Now()
				for time.Since(start) < 64*ms {
					debounced()
				}
			}

			go tightLoop()
			go tightLoop()
			go tightLoop()

			time.Sleep(128 * ms)
			So(callCount, ShouldEqual, 1)
		})
	})
}
