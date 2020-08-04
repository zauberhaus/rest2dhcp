package client

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/google/gopacket/layers"
)

type LeaseStoreItem struct {
	lease   *Lease
	expire  time.Time
	renewal time.Time
	status  layers.DHCPMsgType
}

type LeaseStore struct {
	store  map[uint32]LeaseStoreItem
	mutex  sync.RWMutex
	ttl    time.Duration
	checks time.Duration
}

func NewStore(ttl time.Duration) *LeaseStore {
	return &LeaseStore{
		store:  make(map[uint32]LeaseStoreItem),
		ttl:    ttl,
		checks: time.Duration(10 * time.Second),
	}
}

func (l *LeaseStore) Set(lease *Lease) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	l.store[lease.Xid] = LeaseStoreItem{
		lease:   lease,
		expire:  time.Now().Add(l.ttl),
		status:  lease.GetMsgType(),
		renewal: lease.GetRenewalTime(),
	}
}

func (l *LeaseStore) Get(xid uint32) (*Lease, bool) {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	val, ok := l.store[xid]
	return val.lease, ok

}

func (l *LeaseStore) Has(xid uint32) bool {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	_, ok := l.store[xid]

	return ok
}

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
