package lock

import (
	"fmt"
	"sync"
	"time"
)

type LockType int

const (
	Shared LockType = iota
	Exclusive
)

type lockEntry struct {
	lockType LockType
	holders  map[string]bool
}

type LockManager struct {
	mu    sync.Mutex
	locks map[string]*lockEntry
	wait  map[string][]chan struct{}
}

func NewLockManager() *LockManager {
	return &LockManager{
		locks: make(map[string]*lockEntry),
		wait:  make(map[string][]chan struct{}),
	}
}

type Transaction struct {
	id        string
	locks     map[string]LockType
	locked    map[string]bool
	shrinking bool
}

func (lm *LockManager) Begin(txID string) *Transaction {
	return &Transaction{
		id:     txID,
		locks:  make(map[string]LockType),
		locked: make(map[string]bool),
	}
}

func (lm *LockManager) Lock(tx *Transaction, resource string, lockType LockType, timeout time.Duration) error {
	if tx.shrinking {
		return fmt.Errorf("transaction %s is in shrinking phase, cannot acquire new locks", tx.id)
	}

	if tx.locked[resource] {
		existing := tx.locks[resource]
		if existing >= lockType {
			return nil
		}
	}

	deadline := time.Now().Add(timeout)
	for {
		lm.mu.Lock()
		entry, exists := lm.locks[resource]

		if !exists {
			entry = &lockEntry{
				lockType: lockType,
				holders: map[string]bool{tx.id: true},
			}
			lm.locks[resource] = entry
			tx.locks[resource] = lockType
			tx.locked[resource] = true
			lm.mu.Unlock()
			return nil
		}

		if canAcquire(entry.lockType, lockType) {
			if lockType == Exclusive && entry.lockType == Shared {
				entry.lockType = Exclusive
			}
			entry.holders[tx.id] = true
			tx.locks[resource] = lockType
			tx.locked[resource] = true
			lm.mu.Unlock()
			return nil
		}

		if time.Now().After(deadline) {
			lm.mu.Unlock()
			return fmt.Errorf("timeout waiting for lock on resource %s (possible deadlock)", resource)
		}

		ch := make(chan struct{}, 1)
		lm.wait[resource] = append(lm.wait[resource], ch)
		lm.mu.Unlock()

		select {
		case <-ch:
		case <-time.After(time.Until(deadline)):
		}
	}
}

func (lm *LockManager) Unlock(tx *Transaction, resource string) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	entry, exists := lm.locks[resource]
	if !exists {
		return
	}

	delete(entry.holders, tx.id)
	if tx.locked[resource] {
		tx.shrinking = true
	}
	delete(tx.locked, resource)
	delete(tx.locks, resource)

	if len(entry.holders) == 0 {
		delete(lm.locks, resource)

		if waiters, ok := lm.wait[resource]; ok && len(waiters) > 0 {
			ch := waiters[0]
			lm.wait[resource] = waiters[1:]
			if len(lm.wait[resource]) == 0 {
				delete(lm.wait, resource)
			}
			close(ch)
		}
	}
}

func (lm *LockManager) Commit(tx *Transaction) {
	lm.mu.Lock()
	resources := make([]string, 0, len(tx.locks))
	for r := range tx.locks {
		resources = append(resources, r)
	}
	lm.mu.Unlock()

	for _, r := range resources {
		lm.Unlock(tx, r)
	}
}

func (lm *LockManager) Abort(tx *Transaction) {
	lm.Commit(tx)
}

func canAcquire(current LockType, requested LockType) bool {
	if current == Shared && requested == Shared {
		return true
	}
	if current == Exclusive && requested == Exclusive {
		return false
	}
	if current == Shared && requested == Exclusive {
		return false
	}
	return false
}