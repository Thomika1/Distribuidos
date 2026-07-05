package lock

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestSharedLockConcurrent(t *testing.T) {
	lm := NewLockManager()
	tx1 := lm.Begin("tx1")
	tx2 := lm.Begin("tx2")

	if err := lm.Lock(tx1, "resource1", Shared, 2*time.Second); err != nil {
		t.Fatalf("tx1 failed to acquire shared lock: %v", err)
	}
	if err := lm.Lock(tx2, "resource1", Shared, 2*time.Second); err != nil {
		t.Fatalf("tx2 failed to acquire shared lock: %v", err)
	}

	lm.Commit(tx1)
	lm.Commit(tx2)
}

func TestExclusiveLockBlocks(t *testing.T) {
	lm := NewLockManager()
	tx1 := lm.Begin("tx1")
	tx2 := lm.Begin("tx2")

	if err := lm.Lock(tx1, "resource1", Exclusive, 2*time.Second); err != nil {
		t.Fatalf("tx1 failed to acquire exclusive lock: %v", err)
	}

	err := lm.Lock(tx2, "resource1", Exclusive, 500*time.Millisecond)
	if err == nil {
		t.Fatal("tx2 should have been blocked by tx1's exclusive lock")
	}

	lm.Commit(tx1)
	lm.Commit(tx2)
}

func Test2PLGrowingShrinkingPhases(t *testing.T) {
	lm := NewLockManager()
	tx := lm.Begin("test")

	if err := lm.Lock(tx, "r1", Exclusive, 1*time.Second); err != nil {
		t.Fatalf("failed in growing phase: %v", err)
	}

	lm.Unlock(tx, "r1")

	err := lm.Lock(tx, "r2", Exclusive, 1*time.Second)
	if err == nil {
		t.Fatal("should not acquire lock in shrinking phase")
	}
}

func TestConcurrentWrites(t *testing.T) {
	lm := NewLockManager()
	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			tx := lm.Begin(fmt.Sprintf("tx-%d", id))
			if err := lm.Lock(tx, "counter", Exclusive, 2*time.Second); err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
				time.Sleep(10 * time.Millisecond)
				lm.Commit(tx)
			}
		}(i)
	}

	wg.Wait()
	if successCount != 5 {
		t.Fatalf("expected 5 successful transactions, got %d", successCount)
	}
}