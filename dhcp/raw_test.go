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

func TestRawConn(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	local, remote := getTestAddr()

	data, err := ioutil.ReadFile("./testdata/raw.dat")
	assert.NoError(t, err)

	data2 := make([]byte, len(data))
	copy(data2, data)
	data2[0] = data[2]
	data2[1] = data[3]
	data2[2] = data[0]
	data2[3] = data[1]

	raw := mock.NewMockPacketConn(ctrl)
	mac, _ := net.ParseMAC("00:01:02:03:04:05")
	lease := dhcp.NewLease(layers.DHCPMsgTypeDiscover, 1999, mac, nil)
	var buffer []byte = nil
	_ = buffer

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 2, 0, 0)

	raw.EXPECT().
		LocalAddr().
		Return(local).Times(2)

	raw.EXPECT().
		Close().
		Return(nil)

	raw.EXPECT().
		SetReadDeadline(gomock.Any())

	raw.EXPECT().
		SetWriteDeadline(gomock.Any())

	raw.EXPECT().
		WriteTo(gomock.Eq(data), &net.IPAddr{IP: remote.IP}).
		DoAndReturn(func(p []byte, addr net.Addr) (n int, err error) {
			return len(p), nil
		})

	raw.EXPECT().
		ReadFrom(gomock.Any()).
		DoAndReturn(func(p []byte) (n int, addr net.Addr, err error) {
			l := copy(p, data2)
			return l, &net.IPAddr{IP: local.IP}, nil
		})

	conn := dhcp.NewRawConn(local, remote, raw, logger)
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

func TestRawConnReadInvalidUDPBlock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	local, remote := getTestAddr()

	raw := mock.NewMockPacketConn(ctrl)

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 2, 0, 0)

	raw.EXPECT().
		LocalAddr().
		Return(local).
		AnyTimes()

	raw.EXPECT().
		Close().
		Return(nil)

	raw.EXPECT().
		SetReadDeadline(gomock.Any())

	raw.EXPECT().
		ReadFrom(gomock.Any()).
		DoAndReturn(func(p []byte) (n int, addr net.Addr, err error) {
			data := bytes.Repeat([]byte{45}, 6)
			l := copy(p, data)
			return l, &net.IPAddr{IP: local.IP}, nil
		})

	conn := dhcp.NewRawConn(local, remote, raw, logger)
	assert.NotNil(t, conn)
	assert.Equal(t, conn.Local(), local)
	assert.Equal(t, conn.Remote(), remote)

	ctx := context.Background()
	c1, c2 := conn.Receive(ctx)

	choice2(c1, c2, long, func(dhcp *dhcp.DHCP4) {
		t.Error("error expected")
	}, func(err error) {
		assert.Error(t, err)
		assert.EqualError(t, err, "Invalid UDP header. Length 6 less than 8")
	}, func() {
		t.Error("Unexpected timeout")
	})

	err := conn.Close()
	assert.NoError(t, err)
}

func TestRawConnReadInvalidDHCPBlock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	local, remote := getTestAddr()

	data, err := ioutil.ReadFile("./testdata/raw.dat")
	assert.NoError(t, err)

	data2 := make([]byte, 50)
	copy(data2, data)
	data2[0] = data[2]
	data2[1] = data[3]
	data2[2] = data[0]
	data2[3] = data[1]

	raw := mock.NewMockPacketConn(ctrl)

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 2, 0, 0)

	raw.EXPECT().
		LocalAddr().
		Return(local).
		AnyTimes()

	raw.EXPECT().
		Close().
		Return(nil)

	raw.EXPECT().
		SetReadDeadline(gomock.Any())

	raw.EXPECT().
		ReadFrom(gomock.Any()).
		DoAndReturn(func(p []byte) (n int, addr net.Addr, err error) {
			l := copy(p, data2)
			p[10] = 0
			return l, &net.IPAddr{IP: local.IP}, nil
		})

	conn := dhcp.NewRawConn(local, remote, raw, logger)
	assert.NotNil(t, conn)
	assert.Equal(t, conn.Local(), local)
	assert.Equal(t, conn.Remote(), remote)

	ctx := context.Background()
	c1, c2 := conn.Receive(ctx)

	choice2(c1, c2, long, func(dhcp *dhcp.DHCP4) {
		t.Error("error expected")
	}, func(err error) {
		assert.Error(t, err)
		assert.EqualError(t, err, "DHCPv4 length 42 too short")
	}, func() {
		t.Error("Unexpected timeout")
	})

	err = conn.Close()
	assert.NoError(t, err)
}

func TestRawConnReadFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	local, remote := getTestAddr()
	raw := mock.NewMockPacketConn(ctrl)

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 2, 0, 0)

	raw.EXPECT().
		LocalAddr().
		Return(local).
		AnyTimes()

	raw.EXPECT().
		Close().
		Return(nil)

	raw.EXPECT().
		SetReadDeadline(gomock.Any())

	raw.EXPECT().
		ReadFrom(gomock.Any()).
		DoAndReturn(func(p []byte) (n int, addr net.Addr, err error) {
			return 0, &net.IPAddr{IP: local.IP}, fmt.Errorf("read error")
		})

	conn := dhcp.NewRawConn(local, remote, raw, logger)
	assert.NotNil(t, conn)
	assert.Equal(t, conn.Local(), local)
	assert.Equal(t, conn.Remote(), remote)

	ctx := context.Background()
	c1, c2 := conn.Receive(ctx)

	choice2(c1, c2, long, func(dhcp *dhcp.DHCP4) {
		t.Error("error expected")
	}, func(err error) {
		assert.Error(t, err)
		assert.EqualError(t, err, "read error")
	}, func() {
		t.Error("Unexpected timeout")
	})

	err := conn.Close()
	assert.NoError(t, err)
}

func TestRawConnWriteFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	local, remote := getTestAddr()
	raw := mock.NewMockPacketConn(ctrl)
	mac, _ := net.ParseMAC("00:01:02:03:04:05")
	lease := dhcp.NewLease(layers.DHCPMsgTypeDiscover, 1999, mac, nil)

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 2, 0, 0)

	raw.EXPECT().
		LocalAddr().
		Return(local).
		AnyTimes()

	raw.EXPECT().
		Close().
		Return(nil)

	raw.EXPECT().
		SetWriteDeadline(gomock.Any())

	raw.EXPECT().
		WriteTo(gomock.Any(), &net.IPAddr{IP: remote.IP}).
		DoAndReturn(func(p []byte, addr net.Addr) (n int, err error) {
			return 0, fmt.Errorf("write error")
		})

	conn := dhcp.NewRawConn(local, remote, raw, logger)
	assert.NotNil(t, conn)
	assert.Equal(t, conn.Local(), local)
	assert.Equal(t, conn.Remote(), remote)

	ctx := context.Background()
	c1, c2 := conn.Send(ctx, lease.DHCP4)

	choice1(c1, c2, long, func(l int) {
		t.Error("error expected")
	}, func(err error) {
		assert.Error(t, err)
		assert.EqualError(t, err, "write error")
	}, func() {
		t.Error("Unexpected timeout")
	})

	err := conn.Close()
	assert.NoError(t, err)
}

func TestRawConnSendSetDeadlineFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	local, remote := getTestAddr()
	raw := mock.NewMockPacketConn(ctrl)
	mac, _ := net.ParseMAC("00:01:02:03:04:05")
	lease := dhcp.NewLease(layers.DHCPMsgTypeDiscover, 1999, mac, nil)

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 2, 0, 0)
	raw.EXPECT().
		LocalAddr().
		Return(local).
		AnyTimes()

	raw.EXPECT().
		Close().
		Return(nil)

	raw.EXPECT().
		SetWriteDeadline(gomock.Any()).
		Return(fmt.Errorf("error sdl"))

	conn := dhcp.NewRawConn(local, remote, raw, logger)
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

func TestRawConnReceiveSetDeadlineFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	local, remote := getTestAddr()
	raw := mock.NewMockPacketConn(ctrl)

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 2, 0, 0)

	raw.EXPECT().
		LocalAddr().
		Return(local).
		AnyTimes()

	raw.EXPECT().
		Close().
		Return(nil)

	raw.EXPECT().
		SetReadDeadline(gomock.Any()).
		Return(fmt.Errorf("error sdl"))

	conn := dhcp.NewRawConn(local, remote, raw, logger)
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
		t.Error("Unexpected timeout")
	})

	err := conn.Close()
	assert.NoError(t, err)
}

func TestRawConnBlock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	local, remote := getTestAddr()

	raw := mock.NewMockPacketConn(ctrl)

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 2, 0, 0)

	raw.EXPECT().
		LocalAddr().
		Return(local).AnyTimes()

	raw.EXPECT().
		Close().
		Return(nil)

	raw.EXPECT().
		SetWriteDeadline(gomock.Any()).
		AnyTimes()

	raw.EXPECT().
		WriteTo(gomock.Any(), &net.IPAddr{IP: remote.IP}).
		DoAndReturn(func(p []byte, addr net.Addr) (n int, err error) {
			return len(p), nil
		})

	conn := dhcp.NewRawConn(local, remote, raw, logger)
	assert.NotNil(t, conn)
	assert.Equal(t, conn.Local(), local)
	assert.Equal(t, conn.Remote(), remote)

	dhcp4 := getDHCP4()

	ctx, cancel := context.WithCancel(context.Background())
	conn.Block(ctx)

	ctx2 := context.Background()
	c1, c2 := conn.Send(ctx2, dhcp4)

	choice1(c1, c2, 200*time.Millisecond, func(l int) {
		t.Error("timeout expected")
	}, func(err error) {
		assert.NoError(t, err)
	}, func() {
	})

	cancel()

	choice1(c1, c2, long, func(l int) {
	}, func(err error) {
		assert.NoError(t, err)
	}, func() {
		t.Error("timeout")
	})

	fmt.Println("done.")

	err := conn.Close()
	assert.NoError(t, err)
}

func TestRawConnCancelSendContext(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	local, remote := getTestAddr()

	raw := mock.NewMockPacketConn(ctrl)

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 2, 1, 0)

	raw.EXPECT().
		LocalAddr().
		Return(local).AnyTimes()

	raw.EXPECT().
		Close().
		Return(nil)

	raw.EXPECT().
		SetWriteDeadline(gomock.Any()).Times(2)

	raw.EXPECT().
		WriteTo(gomock.Any(), gomock.Any()).
		DoAndReturn(func(p []byte, addr net.Addr) (n int, err error) {
			time.Sleep(5 * time.Second)
			return 0, nil
		})

	conn := dhcp.NewRawConn(local, remote, raw, logger)
	assert.NotNil(t, conn)
	assert.Equal(t, conn.Local(), local)
	assert.Equal(t, conn.Remote(), remote)

	dhcp4 := getDHCP4()

	ctx, cancel := context.WithCancel(context.Background())
	c1, c2 := conn.Send(ctx, dhcp4)

	time.Sleep(100 * time.Millisecond)

	cancel()

	choice1(c1, c2, long, func(l int) {
		t.Error("Error expected")
	}, func(err error) {
		assert.Error(t, err)
		assert.EqualError(t, err, "context canceled")
	}, func() {
		t.Error("Unexpected timeout")
	})

	err := conn.Close()
	assert.NoError(t, err)

}

func TestRawConnCancelReceiveContext(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	local, remote := getTestAddr()

	raw := mock.NewMockPacketConn(ctrl)
	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 2, 1, 0)

	raw.EXPECT().
		LocalAddr().
		Return(local).AnyTimes()

	raw.EXPECT().
		Close().
		Return(nil)

	raw.EXPECT().
		SetReadDeadline(gomock.Any()).
		Times(2)

	raw.EXPECT().
		ReadFrom(gomock.Any()).
		DoAndReturn(func(p []byte) (n int, addr net.Addr, err error) {
			time.Sleep(5 * time.Second)
			return 0, nil, nil
		})

	conn := dhcp.NewRawConn(local, remote, raw, logger)
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
