package service

import (
	"context"
	"sync"
)

// DeviceLocker serializes device-scoped writes inside one backend instance.
// PostgreSQL and MySQL additionally take SELECT ... FOR UPDATE row locks in
// the retirement transaction; SQLite (used for dev and tests) has no row
// locking, so this mutex provides the same mutual exclusion there. The key is
// the RelatedCode that ties 捕集装置, 许可规则, 排放样本 and 合规决定 together.
type DeviceLocker interface {
	WithLock(ctx context.Context, key string, fn func() error) error
}

type deviceLocker struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func NewDeviceLocker() DeviceLocker {
	return &deviceLocker{locks: make(map[string]*sync.Mutex)}
}

func (l *deviceLocker) mutexFor(key string) *sync.Mutex {
	l.mu.Lock()
	defer l.mu.Unlock()
	lock, exists := l.locks[key]
	if !exists {
		lock = &sync.Mutex{}
		l.locks[key] = lock
	}
	return lock
}

func (l *deviceLocker) WithLock(ctx context.Context, key string, fn func() error) error {
	if key == "" {
		return fn()
	}
	lock := l.mutexFor(key)
	lock.Lock()
	defer lock.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	return fn()
}
