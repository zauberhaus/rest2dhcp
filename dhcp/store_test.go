package dhcp_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/gopacket/layers"
	"github.com/zauberhaus/rest2dhcp/dhcp"
)

func TestStoreSimple(t *testing.T) {
	store := dhcp.NewStore(5 * time.Second)
	var xid1 uint32 = 99
	var xid2 uint32 = 98

	lease := dhcp.NewLease(layers.DHCPMsgTypeDiscover, xid1, nil, nil)
	store.Set(lease)

	lease = dhcp.NewLease(layers.DHCPMsgTypeDiscover, xid2, nil, nil)
	store.Set(lease)

	if !store.Has(xid1) {
		t.Fatalf("Lease %v not found", xid1)
	}

	if store.Has(100) {
		t.Fatalf("Not existing lease %v found", 100)
	}

	l, ok := store.Get(xid1)

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

func TestStoreRun(t *testing.T) {
	store := dhcp.NewStore(1 * time.Second)
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
	store := dhcp.NewStore(1 * time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store.Run(ctx)
	var xid uint32 = 99

	lease := dhcp.NewLease(layers.DHCPMsgTypeDiscover, xid, nil, nil)
	store.Set(lease)

	if !store.Has(xid) {
		t.Fatalf("Lease %v not found", xid)
	}

	time.Sleep(500 * time.Millisecond)

	if !store.Touch(xid) {
		t.Fatalf("Lease %v not found", xid)
	}

	if store.Touch(100) {
		t.Fatalf("Not existing lease %v found", 100)
	}

	time.Sleep(750 * time.Millisecond)

	if !store.Has(xid) {
		t.Fatalf("Lease %v not found", xid)
	}

	time.Sleep(1 * time.Second)

	if store.Has(xid) {
		t.Fatalf("Auto remove for %v failed", xid)
	}

}
