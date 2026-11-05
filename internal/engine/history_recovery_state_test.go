package engine

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"leaguepingo/internal/apperrors"
	"leaguepingo/internal/core"
)

func recoveryAttemptsForTest(r *historyRecovery, key string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if entry := r.entries[key]; entry != nil {
		return entry.attempts
	}
	return 0
}

func TestRecoveryEpisodeBudgetAndQuota(t *testing.T) {
	var r historyRecovery
	now := time.Now()
	first, attempt, err := r.beginRetry(context.Background(), "p", now)
	if err != nil {
		t.Fatal(err)
	}
	deadline, _ := first.Deadline()
	if !deadline.Equal(now.Add(6 * time.Second)) {
		t.Fatal("first retry did not retain six-second budget")
	}
	if _, _, err := r.beginRetry(context.Background(), "p", now); apperrors.CodeOf(err) != apperrors.QueryBusy {
		t.Fatal("parallel recovery admitted")
	}
	r.finishRetry(attempt, false)
	second, attempt, err := r.beginRetry(context.Background(), "p", now.Add(time.Minute-100*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	deadline, _ = second.Deadline()
	if !deadline.Equal(now.Add(time.Minute)) {
		t.Fatal("second retry extended the episode")
	}
	r.finishRetry(attempt, false)
	if _, _, err := r.beginRetry(context.Background(), "p", now); apperrors.CodeOf(err) != apperrors.RecoveryExhausted {
		t.Fatal("more than two attempts admitted")
	}
	_, refunded, err := r.beginRetry(context.Background(), "expired", now)
	if err != nil {
		t.Fatal(err)
	}
	r.finishRetry(refunded, true)
	if _, _, err := r.beginRetry(context.Background(), "expired", now.Add(time.Minute)); apperrors.CodeOf(err) != apperrors.RecoveryExhausted {
		t.Fatal("refund renewed an expired episode")
	}
}

func TestRecoveryLateCleanupKeepsNewTaskAndRefundsOnlyOldAttempt(t *testing.T) {
	var r historyRecovery
	now := time.Now()
	first, old, err := r.beginRetry(context.Background(), "p", now)
	if err != nil {
		t.Fatal(err)
	}
	r.stop()
	if first.Err() != context.Canceled {
		t.Fatal("phase stop did not cancel the old task")
	}
	current, next, err := r.beginRetry(context.Background(), "p", now)
	if err != nil {
		t.Fatal(err)
	}
	defer r.finishRetry(next, false)
	r.finishRetry(old, true)
	if current.Err() != nil || recoveryAttemptsForTest(&r, "p") != 1 {
		t.Fatal("late cleanup cancelled the new task or lost its old-attempt refund")
	}
	if _, unexpected, err := r.beginRetry(context.Background(), "p", now); apperrors.CodeOf(err) != apperrors.QueryBusy {
		if err == nil {
			r.finishRetry(unexpected, false)
		}
		t.Fatal("late cleanup released the new task's occupancy slot")
	}
	r.finishRetry(old, true)
	if recoveryAttemptsForTest(&r, "p") != 1 {
		t.Fatal("old task refunded the quota twice")
	}
}

func TestRecoveryResetIsolatesOldRetryAndBackgroundCompletion(t *testing.T) {
	var r historyRecovery
	r.init()
	updates := r.updatesChannel()
	now := time.Now()
	oldCtx, old, _ := r.beginRetry(context.Background(), "p", now)
	oldBG, oldTask, _ := r.beginBackground(context.Background(), "p")
	r.reset()
	if oldCtx.Err() != context.Canceled || oldBG.Err() != context.Canceled || r.updatesChannel() != updates {
		t.Fatal("reset did not cancel tasks or replaced the UI channel")
	}
	current, next, err := r.beginRetry(context.Background(), "p", now)
	if err != nil {
		t.Fatal(err)
	}
	defer r.finishRetry(next, false)
	currentBG, nextTask, started := r.beginBackground(context.Background(), "p")
	if !started {
		t.Fatal("new connection inherited old background occupancy")
	}
	defer r.finishBackground("p", nextTask)
	r.finishRetry(old, true)
	r.finishBackground("p", oldTask)
	if current.Err() != nil || currentBG.Err() != nil || recoveryAttemptsForTest(&r, "p") != 1 {
		t.Fatal("old connection completion mutated the new tasks")
	}
	if _, _, started := r.beginBackground(context.Background(), "p"); started {
		t.Fatal("old completion released the new background slot")
	}
}

func TestRecoveryBackgroundCoalescingCapacityAndUpdateRetention(t *testing.T) {
	var r historyRecovery
	r.init()
	defer r.reset()
	before := time.Now()
	ctx, first, started := r.beginBackground(context.Background(), "p")
	if !started {
		t.Fatal("background read not admitted")
	}
	after := time.Now()
	deadline, _ := ctx.Deadline()
	if deadline.Before(before.Add(12*time.Second)) || deadline.After(after.Add(12*time.Second)) {
		t.Fatal("background read changed its twelve-second budget")
	}
	if _, _, started := r.beginBackground(context.Background(), "p"); started {
		t.Fatal("duplicate background read not coalesced")
	}
	for i := 1; i < 64; i++ {
		if _, _, started := r.beginBackground(context.Background(), fmt.Sprint(i)); !started {
			t.Fatal("background capacity reduced")
		}
	}
	if _, _, started := r.beginBackground(context.Background(), "extra"); started {
		t.Fatal("background cap exceeded")
	}
	r.finishBackground("p", first)
	if _, _, started := r.beginBackground(context.Background(), "extra"); !started {
		t.Fatal("finished background task did not release its slot")
	}
	for i := 0; i < 65; i++ {
		r.publish(core.Participant{Key: fmt.Sprint(i)})
	}
	for i := 1; i < 65; i++ {
		if p := <-r.updatesChannel(); p.Key != fmt.Sprint(i) {
			t.Fatal("bounded update queue did not retain newest events")
		}
	}
}

func TestRecoveryConcurrentLifecycle(t *testing.T) {
	var r historyRecovery
	r.init()
	var wg sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				key := fmt.Sprint(worker)
				if _, attempt, err := r.beginRetry(context.Background(), key, time.Now()); err == nil {
					r.finishRetry(attempt, true)
				}
				if _, task, started := r.beginBackground(context.Background(), key); started {
					r.finishBackground(key, task)
				}
				r.publish(core.Participant{Key: key})
				r.updatesChannel()
				r.stop()
				r.reset()
			}
		}(worker)
	}
	wg.Wait()
	r.reset()
}
