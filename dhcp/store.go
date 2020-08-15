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
	"log"
	"sync"
	"time"

	"github.com/google/gopacket/layers"
)

//LeaseStoreItem describes a lease store item
type LeaseStoreItem struct {
	lease   *Lease
	expire  time.Time
	renewal time.Time
	status  layers.DHCPMsgType
}

// LeaseStore is thread-safe store for temporary DHCP lease packets with a TTL
type LeaseStore struct {
	store  map[uint32]*LeaseStoreItem
	mutex  sync.RWMutex
	ttl    time.Duration
	checks time.Duration
}

// NewStore creates a new lease store
func NewStore(ttl time.Duration) *LeaseStore {
	return &LeaseStore{
		store:  make(map[uint32]*LeaseStoreItem),
		ttl:    ttl,
		checks: time.Duration(ttl / 2),
	}
}

// Set or add a lease to the store
func (l *LeaseStore) Set(lease *Lease) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if lease != nil {
		l.store[lease.Xid] = &LeaseStoreItem{
			lease:   lease,
			expire:  time.Now().Add(l.ttl),
			status:  lease.GetMsgType(),
			renewal: lease.GetRenewalTime(),
		}
	} else {
		log.Printf("Try to store empty lease")
	}
}

// Get a value from the store
func (l *LeaseStore) Get(xid uint32) (*Lease, bool) {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	val, ok := l.store[xid]
	if ok && val != nil {
		return val.lease, ok
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
		v.expire = time.Now().Add(l.ttl)
		return true
	}

	return false

}

func (l *LeaseStore) outdated() []uint32 {
	outdated := []uint32{}

	l.mutex.RLock()
	defer l.mutex.RUnlock()

	now := time.Now()

	for k, v := range l.store {
		if v.expire.Before(now) {
			outdated = append(outdated, k)
		}
	}

	return outdated
}

// Clean removes outdated entries from the store
func (l *LeaseStore) Clean() {
	outdated := l.outdated()

	if len(outdated) > 0 {

		log.Printf("%v/%v outdated store items", len(outdated), len(l.store))

		l.mutex.Lock()
		defer l.mutex.Unlock()

		for _, key := range outdated {
			l.store[key].lease.Done <- true
			delete(l.store, key)
		}
	}
}

// Run the process to clean the store periodically
func (l *LeaseStore) Run(ctx context.Context) {
	go func() {
		log.Printf("Start LeaseStore clean up process (%v)", l.checks)
		for {
			select {
			case <-time.After(l.checks):
				l.Clean()
			case <-ctx.Done():
				break
			}
		}
	}()
}
