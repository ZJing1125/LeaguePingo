package engine

import (
	"context"
	"sync"
	"testing"
	"time"

	"leaguepingo/internal/apperrors"
)

func TestQuerySessionBudgetAndExclusiveSlot(t *testing.T) {
	q := newQuerySession()
	before := time.Now()
	ctx, release, err := q.begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	after := time.Now()
	deadline, ok := ctx.Deadline()
	if !ok || deadline.Before(before.Add(12*time.Second)) || deadline.After(after.Add(12*time.Second)) {
		t.Fatal("query did not retain its 12-second budget")
	}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if next, done, err := q.begin(context.Background()); apperrors.CodeOf(err) != apperrors.QueryBusy || next != nil || done != nil {
				if done != nil {
					done()
				}
				t.Error("a concurrent query bypassed the foreground slot")
			}
		}()
	}
	wg.Wait()
	release()
	if ctx.Err() != context.Canceled {
		t.Fatal("releasing a query did not cancel its context")
	}
	_, next, err := q.begin(context.Background())
	if err != nil {
		t.Fatal("completed query did not release the slot")
	}
	next()
}

func TestQuerySessionParentCancellationRetainsSlotUntilRelease(t *testing.T) {
	q := newQuerySession()
	parent, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ctx, release, err := q.begin(parent)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	deadline, _ := ctx.Deadline()
	parentDeadline, _ := parent.Deadline()
	if !deadline.Equal(parentDeadline) {
		t.Fatal("query extended its caller's shorter deadline")
	}
	cancel()
	if ctx.Err() != context.Canceled {
		t.Fatal("caller cancellation did not reach the query")
	}
	if _, done, err := q.begin(context.Background()); apperrors.CodeOf(err) != apperrors.QueryBusy {
		if done != nil {
			done()
		}
		t.Fatal("cancellation released the slot before the worker unwound")
	}
}

func TestQuerySessionHomeCancellationDoesNotCancelManualOrParent(t *testing.T) {
	q := newQuerySession()
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	manual, release, err := q.begin(parent)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	home, stop := q.startHome(manual)
	defer stop()
	q.stopHome()
	if home.Err() != context.Canceled || manual.Err() != nil || parent.Err() != nil {
		t.Fatal("leaving idle crossed the home query's cancellation boundary")
	}
	q.stopHome()
	next, stopNext := q.startHome(manual)
	defer stopNext()
	stop() // The previous worker's deferred cleanup must not stop the new child.
	if next.Err() != nil {
		t.Fatal("a later home query inherited the previous cancellation")
	}
	cancel()
	if next.Err() != context.Canceled {
		t.Fatal("caller cancellation did not reach the next home query")
	}
}

func TestNavigationWaitsForWorkerSlotAndCanCancelWaiting(t *testing.T) {
	q := newQuerySession()
	_, release, err := q.begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	waiting, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- q.waitForIdle(waiting) }()
	cancel()
	if err := <-done; err != context.Canceled {
		t.Fatal("closed view retained navigation wait")
	}
	done = make(chan error, 1)
	go func() { done <- q.waitForIdle(context.Background()) }()
	select {
	case <-done:
		t.Fatal("navigation released a still-running worker's slot")
	case <-time.After(10 * time.Millisecond):
	}
	release()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("navigation did not resume")
	}
	_, next, err := q.begin(context.Background())
	if err != nil {
		t.Fatal("idle wait consumed query slot")
	}
	next()
}
