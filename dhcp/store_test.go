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

package dhcp_test

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/google/gopacket/layers"
	"github.com/stretchr/testify/assert"
	"github.com/zauberhaus/rest2dhcp/dhcp"
	"github.com/zauberhaus/rest2dhcp/mock"
)

func TestStoreSimple(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 0, 0, 0)

	store := dhcp.NewStore(5*time.Second, logger)
	var xid1 uint32 = 99
	var xid2 uint32 = 98

	lease := dhcp.NewLease(layers.DHCPMsgTypeDiscover, xid1, nil, nil)
	store.Set(lease)

	lease = dhcp.NewLease(layers.DHCPMsgTypeDiscover, xid2, nil, nil)

	err := store.Set(lease)
	if assert.NoError(t, err) {
		if !store.Has(xid1) {
			t.Fatalf("Lease %v not found", xid1)
		}

		if store.Has(100) {
			t.Fatalf("Not existing lease %v found", 100)
		}

		l, ok, cancel := store.Get(xid1)
		defer cancel()

		if !ok {
			t.Fatalf("Lease %v not found", xid1)
		}

		if l.Xid != xid1 {
			t.Fatalf("Got wrong lease %v != %v", l.Xid, xid1)
		}

		if !store.Remove(xid1) {
			t.Fatalf("Lease %v not found", xid1)
		}

		if store.Remove(100) {
			t.Fatalf("Not existing lease %v found", 100)
		}

		if store.Has(xid1) {
			t.Fatalf("Remove for %v failed", xid1)
		}

		if !store.Has(xid2) {
			t.Fatalf("Lease %v not found", xid2)
		}
	}

	err = store.Set(nil)
	assert.Error(t, err)
}

func TestStoreRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 0, 2, 0)

	store := dhcp.NewStore(200*time.Millisecond, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store.Run(ctx)
	var xid uint32 = 99

	lease := dhcp.NewLease(layers.DHCPMsgTypeDiscover, xid, nil, nil)
	store.Set(lease)

	if !store.Has(xid) {
		t.Fatalf("Lease %v not found", xid)
	}

	time.Sleep(2 * time.Second)

	if store.Has(xid) {
		t.Fatalf("Auto remove for %v failed", xid)
	}

}

func TestStoreTouch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 0, 1, 0, 0, 0)

	store := dhcp.NewStore(1*time.Hour, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store.Run(ctx)
	var xid uint32 = 99

	lease := dhcp.NewLease(layers.DHCPMsgTypeDiscover, xid, nil, nil)
	store.Set(lease)

	item, ok := store.GetItem(xid)
	if assert.True(t, ok) {
		expire := item.Expire

		if !store.Touch(xid) {
			t.Fatalf("Lease %v not found", xid)
		}

		diff := item.Expire.Sub(expire)
		assert.Greater(t, diff, 0*time.Second)
	}
}
