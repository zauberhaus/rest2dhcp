/*
Copyright © 2020 Dirk Lembke <dirk@lembke.nz>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dhcp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/gopacket/layers"
	"github.com/zauberhaus/rest2dhcp/logger"
)

var (
	nop = func() {
		// Do nothing because response is nil.
	}
)

//LeaseStoreItem describes a lease store item
type LeaseStoreItem struct {
	Lease  *Lease
	Expire time.Time
	Status layers.DHCPMsgType
}

// LeaseStore is thread-safe store for temporary DHCP lease packets with a TTL
type LeaseStore struct {
	store  map[uint32]*LeaseStoreItem
	mutex  sync.RWMutex
	ttl    time.Duration
	checks time.Duration
	logger logger.Logger
}

// NewStore creates a new lease store
func NewStore(ttl time.Duration, logger logger.Logger) *LeaseStore {
	return &LeaseStore{
		store:  make(map[uint32]*LeaseStoreItem),
		ttl:    ttl,
		logger: logger,
		checks: time.Duration(ttl / 2),
	}
}

// Set or add a lease to the store
func (l *LeaseStore) Set(lease *Lease) error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if lease != nil {

		l.store[lease.Xid] = &LeaseStoreItem{
			Lease:  lease,
			Expire: time.Now().Add(l.ttl),
			Status: lease.GetMsgType(),
		}
		return nil
	} else {
		return fmt.Errorf("try to store empty lease")
	}

}

// Get a value from the store
func (l *LeaseStore) Get(xid uint32) (*Lease, bool, context.CancelFunc) {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	val, ok := l.store[xid]
	if ok && val != nil {
		val.Lease.Lock()

		return val.Lease, ok, func() {
			val.Lease.Unlock()
		}
	}

	return nil, ok, nop
}

// GetItem returns an item from the store
func (l *LeaseStore) GetItem(xid uint32) (*LeaseStoreItem, bool) {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	val, ok := l.store[xid]
	if ok && val != nil {
		return val, ok
	}

	return nil, ok
}

// Has checks if a xid is in the store
func (l *LeaseStore) Has(xid uint32) bool {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	_, ok := l.store[xid]

	return ok
}

// Remove a value from the store
func (l *LeaseStore) Remove(xid uint32) bool {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	_, ok := l.store[xid]
	if ok {
		delete(l.store, xid)
		return true
	}

	return false
}

// Touch extends the expire time of an entry
func (l *LeaseStore) Touch(xid uint32) bool {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	v, ok := l.store[xid]
	if ok {
		v.Expire = time.Now().Add(l.ttl)
		return true
	}

	return false

}

// Touch extends the expire time of an entry
func (l *LeaseStore) HasStatus(xid uint32, status layers.DHCPMsgType) bool {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	v, ok := l.store[xid]
	if ok {
		return v.Lease.GetMsgType() == status
	}

	return false

}

func (l *LeaseStore) outdated() []uint32 {
	outdated := []uint32{}

	l.mutex.RLock()
	defer l.mutex.RUnlock()

	now := time.Now()

	for k, v := range l.store {
		if v.Expire.Before(now) {
			outdated = append(outdated, k)
		}
	}

	return outdated
}

// Clean removes outdated entries from the store
func (l *LeaseStore) Clean() {
	outdated := l.outdated()

	if len(outdated) > 0 {

		l.logger.Debugf("%v/%v outdated store items", len(outdated), len(l.store))

		l.mutex.Lock()
		defer l.mutex.Unlock()

		for _, key := range outdated {
			e, ok := l.store[key]
			if ok {
				if e.Lease != nil {
					e.Lease.Done <- true
				}

				delete(l.store, key)
			}
		}
	}
}

// Run the process to clean the store periodically
func (l *LeaseStore) Run(ctx context.Context) {
	l.logger.Debugf("Start LeaseStore clean up process (%v)", l.checks)

	go func() {
		timer := time.NewTimer(l.checks)
		defer timer.Stop()

		for {
			select {
			case <-timer.C:
				l.Clean()
				timer.Reset(l.checks)
			case <-ctx.Done():
				return
			}
		}
	}()

}
