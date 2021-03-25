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
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/google/gopacket/layers"
	"github.com/stretchr/testify/assert"
	"github.com/zauberhaus/rest2dhcp/dhcp"
	"github.com/zauberhaus/rest2dhcp/mock"
)

func TestDualConn(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	local, remote := getTestAddr()
	udpData, err := ioutil.ReadFile("./testdata/udp.dat")
	assert.NoError(t, err)

	rawData, err := ioutil.ReadFile("./testdata/raw2.dat")
	assert.NoError(t, err)

	in := mock.NewMockUDPPacketConn(ctrl)
	out := mock.NewMockPacketConn(ctrl)
	mac, _ := net.ParseMAC("00:01:02:03:04:05")
	lease := dhcp.NewLease(layers.DHCPMsgTypeDiscover, 1999, mac, nil)

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 4, 1, 0, 0)

	out.EXPECT().
		LocalAddr().
		Return(local).Times(2)

	out.EXPECT().
		Close().
		Return(nil)

	in.EXPECT().
		Close().
		Return(nil)

	in.EXPECT().
		SetReadDeadline(gomock.Any()).
		Times(1)

	out.EXPECT().
		SetWriteDeadline(gomock.Any()).
		Times(1)

	out.EXPECT().
		WriteTo(gomock.Eq(rawData), &net.IPAddr{IP: remote.IP}).
		DoAndReturn(func(p []byte, addr net.Addr) (n int, err error) {
			return len(p), nil
		})

	in.EXPECT().
		ReadFromUDP(gomock.Any()).
		DoAndReturn(func(p []byte) (n int, addr *net.UDPAddr, err error) {
			l := copy(p, udpData)
			return l, remote, nil
		})

	conn := dhcp.NewDualConn(local, remote, true, out, in, logger)
	assert.NotNil(t, conn)
	assert.Equal(t, conn.Local(), local)
	assert.Equal(t, conn.Remote(), remote)

	ctx := context.Background()
	c1, c2 := conn.Send(ctx, lease.DHCP4)

	choice1(c1, c2, long, func(l int) {
		assert.Equal(t, l, 252)
	}, func(err error) {
		assert.NoError(t, err)
	}, func() {
		t.Error("Unexpected timeout")
	})

	c3, c2 := conn.Receive(ctx)
	choice2(c3, c2, long, func(dhcp *dhcp.DHCP4) {
		var v1 = *dhcp
		var v2 = *(lease.DHCP4)
		assert.EqualValues(t, v1.DHCPv4.Payload, v2.DHCPv4.Payload)
	}, func(err error) {
		assert.NoError(t, err)
	}, func() {
		t.Error("Unexpected timeout")
	})

	err = conn.Close()
	assert.NoError(t, err)
}

func TestDualConnGetPort(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	in := mock.NewMockUDPPacketConn(ctrl)
	out := mock.NewMockPacketConn(ctrl)
	local, remote := getTestAddr()

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 4, 0, 0, 0)

	out.EXPECT().
		LocalAddr().
		Return(local).
		AnyTimes()

	conn := dhcp.NewDualConn(local, remote, true, out, in, logger).(*dhcp.DualConn)

	port := conn.GetPort()
	assert.Equal(t, layers.UDPPort(68), port)

	port = conn.GetPort()
	assert.Equal(t, layers.UDPPort(68), port)

	conn = dhcp.NewDualConn(local, remote, false, out, in, logger).(*dhcp.DualConn)

	port = conn.GetPort()
	assert.Equal(t, layers.UDPPort(1), port)

	port = conn.GetPort()
	assert.Equal(t, layers.UDPPort(2), port)
}

func TestDualConnReadInvalidBlock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	local, remote := getTestAddr()
	in := mock.NewMockUDPPacketConn(ctrl)
	out := mock.NewMockPacketConn(ctrl)

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 4, 0, 0)

	out.EXPECT().
		LocalAddr().
		Return(local).
		AnyTimes()

	out.EXPECT().
		Close().
		Return(nil)

	in.EXPECT().
		Close().
		Return(nil)

	in.EXPECT().
		SetReadDeadline(gomock.Any()).
		Times(1)

	in.EXPECT().
		ReadFromUDP(gomock.Any()).
		DoAndReturn(func(p []byte) (n int, addr *net.UDPAddr, err error) {
			data := bytes.Repeat([]byte{45}, 302)
			l := copy(p, data)
			return l, remote, nil
		})

	conn := dhcp.NewDualConn(local, remote, true, out, in, logger)
	assert.NotNil(t, conn)
	assert.Equal(t, conn.Local(), local)
	assert.Equal(t, conn.Remote(), remote)

	ctx := context.Background()
	c1, c2 := conn.Receive(ctx)

	choice2(c1, c2, long, func(dhcp *dhcp.DHCP4) {
		t.Error("error expected")
	}, func(err error) {
		assert.Error(t, err)
		assert.EqualError(t, err, "Bad DHCP header")
	}, func() {
		t.Error("unexpected timeout")
	})

	err := conn.Close()
	assert.NoError(t, err)
}

func TestDualConnReadFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	local, remote := getTestAddr()
	in := mock.NewMockUDPPacketConn(ctrl)
	out := mock.NewMockPacketConn(ctrl)

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 4, 0, 0)

	out.EXPECT().
		LocalAddr().
		Return(local).
		AnyTimes()

	out.EXPECT().
		Close().
		Return(nil)

	in.EXPECT().
		Close().
		Return(nil)

	in.EXPECT().
		SetReadDeadline(gomock.Any()).
		Times(1)

	in.EXPECT().
		ReadFromUDP(gomock.Any()).
		DoAndReturn(func(p []byte) (n int, addr *net.UDPAddr, err error) {
			return 0, remote, fmt.Errorf(t.Name() + " read error")
		})

	conn := dhcp.NewDualConn(local, remote, true, out, in, logger)
	assert.NotNil(t, conn)
	assert.Equal(t, conn.Local(), local)
	assert.Equal(t, conn.Remote(), remote)

	ctx := context.Background()
	c1, c2 := conn.Receive(ctx)

	choice2(c1, c2, long, func(dhcp *dhcp.DHCP4) {
		t.Error("error expected")
	}, func(err error) {
		assert.Error(t, err)
		assert.EqualError(t, err, t.Name()+" read error")
	}, func() {
		t.Error("unexpected timeout")
	})

	err := conn.Close()
	assert.NoError(t, err)
}

func TestDualConnWriteFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	local, remote := getTestAddr()
	rawData, err := ioutil.ReadFile("./testdata/raw2.dat")
	assert.NoError(t, err)

	in := mock.NewMockUDPPacketConn(ctrl)
	out := mock.NewMockPacketConn(ctrl)
	mac, _ := net.ParseMAC("00:01:02:03:04:05")
	lease := dhcp.NewLease(layers.DHCPMsgTypeDiscover, 1999, mac, nil)

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 4, 1, 0)

	out.EXPECT().
		LocalAddr().
		Return(local).
		AnyTimes()

	out.EXPECT().
		Close().
		Return(nil)

	in.EXPECT().
		Close().
		Return(nil)

	out.EXPECT().
		SetWriteDeadline(gomock.Any()).
		AnyTimes()

	out.EXPECT().
		WriteTo(gomock.Eq(rawData), &net.IPAddr{IP: remote.IP}).
		DoAndReturn(func(p []byte, addr net.Addr) (n int, err error) {
			return len(p), fmt.Errorf(t.Name() + " write error")
		})

	conn := dhcp.NewDualConn(local, remote, true, out, in, logger)
	assert.NotNil(t, conn)
	assert.Equal(t, conn.Local(), local)
	assert.Equal(t, conn.Remote(), remote)

	ctx := context.Background()

	c1, c2 := conn.Send(ctx, lease.DHCP4)
	choice1(c1, c2, long, func(l int) {
		t.Error("error expected")
	}, func(err error) {
		assert.Error(t, err)
		assert.EqualError(t, err, t.Name()+" write error")
	}, func() {
		t.Error("Unexpected timeout")
	})

	err = conn.Close()
	assert.NoError(t, err)
}

func TestDualConnSendSetDeadlineFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	local, remote := getTestAddr()
	in := mock.NewMockUDPPacketConn(ctrl)
	out := mock.NewMockPacketConn(ctrl)
	mac, _ := net.ParseMAC("00:01:02:03:04:05")
	lease := dhcp.NewLease(layers.DHCPMsgTypeDiscover, 1999, mac, nil)

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 4, 1, 0)

	out.EXPECT().
		LocalAddr().
		Return(local).
		AnyTimes()

	out.EXPECT().
		Close().
		Return(nil)

	in.EXPECT().
		Close().
		Return(nil)

	out.EXPECT().
		SetWriteDeadline(gomock.Any()).
		Return(fmt.Errorf("error sdl")).
		AnyTimes()

	conn := dhcp.NewDualConn(local, remote, true, out, in, logger)
	assert.NotNil(t, conn)
	assert.Equal(t, conn.Local(), local)
	assert.Equal(t, conn.Remote(), remote)

	ctx := context.Background()
	c1, c2 := conn.Send(ctx, lease.DHCP4)
	choice1(c1, c2, long, func(l int) {
		t.Error("error expected")
	}, func(err error) {
		assert.Error(t, err)
		assert.EqualError(t, err, "error sdl")
	}, func() {
		t.Error("Unexpected timeout")
	})

	err := conn.Close()
	assert.NoError(t, err)
}

func TestDualConnReceiveSetDeadlineFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	local, remote := getTestAddr()
	in := mock.NewMockUDPPacketConn(ctrl)
	out := mock.NewMockPacketConn(ctrl)

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 4, 0, 0)

	out.EXPECT().
		LocalAddr().
		Return(local).
		AnyTimes()

	out.EXPECT().
		Close().
		Return(nil)

	in.EXPECT().
		Close().
		Return(nil)

	in.EXPECT().
		SetReadDeadline(gomock.Any()).
		Return(fmt.Errorf("error sdl"))

	conn := dhcp.NewDualConn(local, remote, true, out, in, logger)
	assert.NotNil(t, conn)
	assert.Equal(t, conn.Local(), local)
	assert.Equal(t, conn.Remote(), remote)

	ctx := context.Background()
	c1, c2 := conn.Receive(ctx)
	choice2(c1, c2, long, func(dhcp *dhcp.DHCP4) {
		t.Error("error expected")
	}, func(err error) {
		assert.Error(t, err)
		assert.EqualError(t, err, "error sdl")
	}, func() {
		t.Error("unexpected timeout")
	})

	err := conn.Close()
	assert.NoError(t, err)
}

func TestDualConnBlock(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	local, remote := getTestAddr()

	in := mock.NewMockUDPPacketConn(ctrl)
	out := mock.NewMockPacketConn(ctrl)

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 6, 1, 0)

	out.EXPECT().
		LocalAddr().
		Return(local).AnyTimes()

	out.EXPECT().
		Close().
		Return(nil)

	in.EXPECT().
		Close().
		Return(nil)

	out.EXPECT().
		SetWriteDeadline(gomock.Any()).
		AnyTimes()

	out.EXPECT().
		WriteTo(gomock.Any(), &net.IPAddr{IP: remote.IP}).
		DoAndReturn(func(p []byte, addr net.Addr) (n int, err error) {
			return len(p), nil
		})

	conn := dhcp.NewDualConn(local, remote, true, out, in, logger)
	assert.NotNil(t, conn)
	assert.Equal(t, conn.Local(), local)
	assert.Equal(t, conn.Remote(), remote)

	dhcp4 := getDHCP4()

	ctx, cancel := context.WithCancel(context.Background())
	conn.Block(ctx)

	ctx2 := context.Background()
	c1, c2 := conn.Send(ctx2, dhcp4)
	choice1(c1, c2, 200*time.Millisecond, func(l int) {
		t.Error("error expected")
	}, func(err error) {
		assert.NoError(t, err)
	}, func() {
		logger.Info("Blocked")
	})

	cancel()

	choice1(c1, c2, long, func(l int) {
		logger.Info("Unblocked")
	}, func(err error) {
		assert.NoError(t, err)
	}, func() {
		t.Error("timeout")
	})

	err := conn.Close()
	assert.NoError(t, err)
}

func TestDualConnCancelSendContext(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	local, remote := getTestAddr()

	out := mock.NewMockPacketConn(ctrl)
	in := mock.NewMockUDPPacketConn(ctrl)

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 4, 2, 0)

	out.EXPECT().
		LocalAddr().
		Return(local).AnyTimes()

	out.EXPECT().
		Close().
		Return(nil)

	in.EXPECT().
		Close().
		Return(nil)

	out.EXPECT().
		SetWriteDeadline(gomock.Any()).
		DoAndReturn(func(t time.Time) error {
			fmt.Printf("set deadline %v\n", t)
			return nil
		}).
		Times(2)

	out.EXPECT().
		WriteTo(gomock.Any(), gomock.Any()).
		DoAndReturn(func(p []byte, addr net.Addr) (n int, err error) {
			time.Sleep(5 * time.Second)
			return 0, nil
		})

	conn := dhcp.NewDualConn(local, remote, true, out, in, logger)
	assert.NotNil(t, conn)
	assert.Equal(t, conn.Local(), local)
	assert.Equal(t, conn.Remote(), remote)

	dhcp4 := getDHCP4()

	ctx, cancel := context.WithCancel(context.Background())
	c1, c2 := conn.Send(ctx, dhcp4)

	time.Sleep(100 * time.Millisecond)

	cancel()

	choice1(c1, c2, long, func(l int) {
		t.Error("error expected")
	}, func(err error) {
		assert.Error(t, err)
		assert.EqualError(t, err, "context canceled")
	}, func() {
		t.Error("Unexpected timeout")
	})

	err := conn.Close()
	assert.NoError(t, err)
}

func TestDualConnCancelReceiveContext(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	local, remote := getTestAddr()

	out := mock.NewMockPacketConn(ctrl)
	in := mock.NewMockUDPPacketConn(ctrl)

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 4, 1, 0)

	out.EXPECT().
		LocalAddr().
		Return(local).AnyTimes()

	out.EXPECT().
		Close().
		Return(nil)

	in.EXPECT().
		Close().
		Return(nil)

	in.EXPECT().
		SetReadDeadline(gomock.Any()).
		DoAndReturn(func(t time.Time) error {
			fmt.Printf("set deadline %v\n", t)
			return nil
		}).
		Times(2)

	in.EXPECT().ReadFromUDP(gomock.Any()).
		DoAndReturn(func(p []byte) (int, *net.UDPAddr, error) {
			time.Sleep(30 * time.Second)
			return 0, nil, nil
		})

	conn := dhcp.NewDualConn(local, remote, true, out, in, logger)
	assert.NotNil(t, conn)
	assert.Equal(t, conn.Local(), local)
	assert.Equal(t, conn.Remote(), remote)

	ctx, cancel := context.WithCancel(context.Background())
	c1, c2 := conn.Receive(ctx)

	time.Sleep(100 * time.Millisecond)

	cancel()

	choice2(c1, c2, long, func(dhcp *dhcp.DHCP4) {
		t.Error("error expected")
	}, func(err error) {
		assert.Error(t, err)
		assert.EqualError(t, err, "context canceled")
	}, func() {
		t.Error("Unexpected timeout")
	})

	err := conn.Close()
	assert.NoError(t, err)
}
