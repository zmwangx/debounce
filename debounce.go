// Package debounce is complete and thread-safe port of [lodash's debounce] for
// Go, with support for leading/trailing/both edges, max wait, arguments, return
// values, flush, cancel, etc. See this [CSS-Tricks article] for detailed
// explanation of debouncing behavior.
//
// [lodash's debounce]: https://lodash.com/docs/#debounce
// [CSS-Tricks article]: https://css-tricks.com/debouncing-throttling-explained-examples/
package debounce

import (
	"sync"
	"time"
)

type options struct {
	leading  bool
	trailing bool
	maxWait  time.Duration
}

type Option func(*options)

// WithLeading returns an Option that sets whether the function is invoked on
// the leading edge. The default is false.
func WithLeading(leading bool) Option {
	return func(o *options) {
		o.leading = leading
	}
}

// WithTrailing returns an Option that sets whether the function is invoked on
// the trailing edge. The default is true.
func WithTrailing(trailing bool) Option {
	return func(o *options) {
		o.trailing = trailing
	}
}

// WithMaxWait returns an Option that sets the maximum time the function is
// allowed to be delayed before it is invoked. A nonpositive value is treated as
// no max wait. The default is no max wait.
func WithMaxWait(maxWait time.Duration) Option {
	return func(o *options) {
		o.maxWait = maxWait
	}
}

// A Control struct contains control methods for a debounced function.
type Control struct {
	Cancel  func()
	Flush   func()
	Pending func() bool
}

type ControlWithReturnValue[T any] struct {
	Cancel  func()
	Flush   func() T
	Pending func() bool
}

// Debounce is a special case of [DebounceWithCustomSignature] where the
// function has no arguments and returns no value.
func Debounce(fn func(), wait time.Duration, opts ...Option) (debounced func(), control Control) {
	d, c := DebounceWithCustomSignature(func(...interface{}) interface{} {
		fn()
		return nil
	}, wait, opts...)
	debounced = func() { d() }
	control = Control{
		Cancel:  c.Cancel,
		Flush:   func() { c.Flush() },
		Pending: c.Pending,
	}
	return
}

// DebounceWithCustomSignature creates a debounced function that delays invoking
// fn until after wait has elapsed since the last time the debounced function
// was invoked. fn is invoked with the last arguments provided to the debounced
// function. Subsequent calls to the debounced function return the result of the
// last fn invocation.
//
// A control struct is also returned which comes with the following methods:
//   - Cancel() cancels any pending invocation;
//   - Flush() immediately invokes any pending invocation;
//   - Pending() returns whether there is a pending invocation.
//
// The wait timeout should be positive.
//
// Options can be provided to customize behavior:
//   - [WithLeading]: whether the function is invoked on the leading edge of the
//     wait timeout.
//   - [WithTrailing]: whether the function is invoked on the trailing edge of
//     the wait timeout.
//   - [WithMaxWait]: the maximum time fn is allowed to be delayed before it's
//     invoked.
//
// If the leading and trailing options are both true, fn is invoked on the
// trailing edge of the timeout only if the debounced function is invoked more
// than once during the wait timeout.
//
// The debounced function as well as control functions are safe for concurrent
// use by multiple goroutines. In particular, invocations of fn are serialized;
// using nonlocal variables in fn without synchronization won't lead to data
// races (provided they aren't accessed outside of fn at the same time).
func DebounceWithCustomSignature[T1, T2 any](
	fn func(args ...T1) T2,
	wait time.Duration,
	opts ...Option,
) (debounced func(args ...T1) T2, control ControlWithReturnValue[T2]) {
	o := &options{
		leading:  false,
		trailing: true,
		maxWait:  0,
	}
	for _, opt := range opts {
		opt(o)
	}

	leading := o.leading
	trailing := o.trailing
	hasMaxWait := o.maxWait > 0
	maxWait := o.maxWait
	if wait > maxWait {
		maxWait = wait
	}

	// Locking is necessary in this Go port; the JS implementation is
	// thread-safe only because JS is single-threaded.
	var lock sync.RWMutex

	var lastCallTime time.Time
	var lastInvokeTime time.Time
	var lastArgs []T1
	var lastArgsActive bool
	var timer *time.Timer
	var result T2

	// A function named ...Locked is a function that must be called with the
	// lock held.
	var invokeFuncLocked func(time.Time) T2
	var leadingEdgeLocked func(time.Time) T2
	var remainingWaitLocked func(time.Time) time.Duration
	var shouldInvokeLocked func(time.Time) bool
	var timerExpired func()
	var trailingEdgeLocked func(time.Time) T2
	var cancel func()
	var flush func() T2
	var pending func() bool

	invokeFuncLocked = func(t time.Time) T2 {
		lastInvokeTime = t
		result = fn(lastArgs...)
		lastArgs = nil
		lastArgsActive = false
		return result
	}

	leadingEdgeLocked = func(t time.Time) T2 {
		// Reset any `maxWait` timer.
		lastInvokeTime = t
		// Start the timer for the trailing edge.
		timer = time.AfterFunc(wait, timerExpired)
		// Invoke the leading edge.
		if leading {
			return invokeFuncLocked(t)
		}
		return result
	}

	remainingWaitLocked = func(t time.Time) time.Duration {
		timeSinceLastCall := t.Sub(lastCallTime)
		timeSinceLastInvoke := t.Sub(lastInvokeTime)
		timeWaiting := wait - timeSinceLastCall
		if hasMaxWait && timeWaiting > maxWait-timeSinceLastInvoke {
			return maxWait - timeSinceLastInvoke
		}
		return timeWaiting
	}

	shouldInvokeLocked = func(t time.Time) bool {
		timeSinceLastCall := t.Sub(lastCallTime)
		timeSinceLastInvoke := t.Sub(lastInvokeTime)
		// Either this is the first call, activity has stopped and we're at the
		// trailing edge, the system time has gone backwards and we're treating
		// it as the trailing edge, or we've hit the `maxWait` limit.
		//
		// Note that we preserved the timeSinceLastCall < 0 condition from
		// lodash, even though elapsed time should always be positive in Go as
		// time measurements are done with the monotonic clock.
		return lastCallTime.IsZero() || timeSinceLastCall >= wait || timeSinceLastCall < 0 || (hasMaxWait && timeSinceLastInvoke >= maxWait)
	}

	timerExpired = func() {
		lock.Lock()
		defer lock.Unlock()
		t := time.Now()
		if shouldInvokeLocked(t) {
			trailingEdgeLocked(t)
			return
		}
		// Restart the timer.
		timer = time.AfterFunc(remainingWaitLocked(t), timerExpired)
	}

	trailingEdgeLocked = func(t time.Time) T2 {
		timer = nil
		// Only invoke if `fn` has been debounced at least once.
		if trailing && lastArgsActive {
			return invokeFuncLocked(t)
		}
		lastArgs = nil
		lastArgsActive = false
		return result
	}

	cancel = func() {
		lock.Lock()
		defer lock.Unlock()
		if timer != nil {
			timer.Stop()
		}
		lastCallTime = time.Time{}
		lastInvokeTime = time.Time{}
		lastArgs = nil
		lastArgsActive = false
		timer = nil
	}

	flush = func() T2 {
		lock.Lock()
		defer lock.Unlock()
		if timer == nil {
			return result
		}
		return trailingEdgeLocked(time.Now())
	}

	pending = func() bool {
		lock.RLock()
		defer lock.RUnlock()
		return timer != nil
	}

	debounced = func(args ...T1) T2 {
		lock.Lock()
		defer lock.Unlock()
		t := time.Now()
		isInvoking := shouldInvokeLocked(t)
		lastCallTime = t
		lastArgs = args
		lastArgsActive = true
		if isInvoking {
			if timer == nil {
				return leadingEdgeLocked(t)
			}
			if hasMaxWait {
				// Handle invocations in a tight loop.
				timer = time.AfterFunc(wait, timerExpired)
				return invokeFuncLocked(t)
			}
		}
		if timer == nil {
			timer = time.AfterFunc(wait, timerExpired)
		}
		return result
	}
	control = ControlWithReturnValue[T2]{
		Cancel:  cancel,
		Flush:   flush,
		Pending: pending,
	}
	return
}

// Throttle is a special case of [Debounce] with both edges enabled, and maxWait
// set to wait.
func Throttle(fn func(), wait time.Duration) (throttled func(), control Control) {
	return Debounce(fn, wait, WithLeading(true), WithTrailing(true), WithMaxWait(wait))
}

// ThrottleWithCustomSignature is a special case of
// [DebounceWithCustomSignature] with both edges enabled, and maxWait set to
// wait.
func ThrottleWithCustomSignature[T1, T2 any](
	fn func(args ...T1) T2,
	wait time.Duration,
) (throttled func(args ...T1) T2, control ControlWithReturnValue[T2]) {
	return DebounceWithCustomSignature(fn, wait, WithLeading(true), WithTrailing(true), WithMaxWait(wait))
}
