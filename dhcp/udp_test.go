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
	"github.com/stretchr/testify/assert"
	"github.com/zauberhaus/rest2dhcp/dhcp"
	"github.com/zauberhaus/rest2dhcp/mock"
)

func TestUDPConn(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	local, remote := getTestAddr()

	data, err := ioutil.ReadFile("./testdata/udp.dat")
	assert.NoError(t, err)

	udp := mock.NewMockUDPPacketConn(ctrl)
	dhcp4 := getDHCP4()
	var buffer []byte = nil
	_ = buffer

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 2, 0, 0)

	udp.EXPECT().
		LocalAddr().
		Return(local).Times(3)

	udp.EXPECT().
		RemoteAddr().
		Return(remote).Times(3)

	udp.EXPECT().
		Close().
		Return(nil)

	udp.EXPECT().
		SetReadDeadline(gomock.Any())

	udp.EXPECT().
		SetWriteDeadline(gomock.Any())

	udp.EXPECT().
		Write(gomock.Eq(data)).
		DoAndReturn(func(p []byte) (n int, err error) {
			buffer = p
			return len(p), nil
		})

	udp.EXPECT().
		ReadFromUDP(gomock.Any()).
		DoAndReturn(func(p []byte) (n int, addr *net.UDPAddr, err error) {
			l := copy(p, data)
			return l, remote, nil
		})

	conn := dhcp.NewUDPConn(local, remote, udp, logger)
	assert.NotNil(t, conn)
	assert.Equal(t, conn.Local(), local)
	assert.Equal(t, conn.Remote(), remote)

	ctx := context.Background()
	c1, c2 := conn.Send(ctx, dhcp4)

	choice1(c1, c2, 10*time.Second, func(l int) {
		assert.Equal(t, l, 301)
	}, func(err error) {
		assert.NoError(t, err)
	}, func() {
		t.Error("Unexpected timeout")
	})

	c3, c2 := conn.Receive(ctx)
	choice2(c3, c2, 10*time.Second, func(dhcp *dhcp.DHCP4) {
		var v1 = *dhcp
		var v2 = *(dhcp4)
		assert.EqualValues(t, v1.DHCPv4.Payload, v2.DHCPv4.Payload)
	}, func(err error) {
		assert.NoError(t, err)
	}, func() {
		t.Error("Unexpected timeout")
	})

	err = conn.Close()
	assert.NoError(t, err)
}

func TestUDPConnReadInvalidBlock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	local, remote := getTestAddr()
	udp := mock.NewMockUDPPacketConn(ctrl)

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 2, 0, 0)

	udp.EXPECT().
		LocalAddr().
		Return(local).Times(3)

	udp.EXPECT().
		RemoteAddr().
		Return(remote).Times(3)

	udp.EXPECT().
		Close().
		Return(nil)

	udp.EXPECT().
		SetReadDeadline(gomock.Any()).
		Times(1)

	udp.EXPECT().
		ReadFromUDP(gomock.Any()).
		DoAndReturn(func(p []byte) (n int, addr *net.UDPAddr, err error) {
			data := bytes.Repeat([]byte{45}, 302)
			l := copy(p, data)
			return l, remote, nil
		})

	conn := dhcp.NewUDPConn(local, remote, udp, logger)
	assert.NotNil(t, conn)
	assert.Equal(t, conn.Local(), local)
	assert.Equal(t, conn.Remote(), remote)

	ctx := context.Background()
	c1, c2 := conn.Receive(ctx)

	choice2(c1, c2, 10*time.Second, func(d *dhcp.DHCP4) {
		t.Error("Error expected")
	}, func(err error) {
		assert.Error(t, err)
		assert.EqualError(t, err, "Bad DHCP header")
	}, func() {
		t.Error("Unexpected timeout")
	})

	err := conn.Close()
	assert.NoError(t, err)
}

func TestUDPConnReadFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	local, remote := getTestAddr()
	udp := mock.NewMockUDPPacketConn(ctrl)

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 2, 0, 0)

	udp.EXPECT().
		Close().
		Return(nil)

	udp.EXPECT().
		LocalAddr().
		Return(local).Times(3)

	udp.EXPECT().
		RemoteAddr().
		Return(remote).Times(3)

	udp.EXPECT().
		SetReadDeadline(gomock.Any()).
		Times(1)

	udp.EXPECT().
		ReadFromUDP(gomock.Any()).
		DoAndReturn(func(p []byte) (n int, addr *net.UDPAddr, err error) {
			return 0, nil, fmt.Errorf("read error")
		})

	conn := dhcp.NewUDPConn(local, remote, udp, logger)
	assert.NotNil(t, conn)
	assert.Equal(t, conn.Local(), local)
	assert.Equal(t, conn.Remote(), remote)

	ctx := context.Background()
	c1, c2 := conn.Receive(ctx)

	choice2(c1, c2, 10*time.Second, func(d *dhcp.DHCP4) {
		t.Error("Error expected")
	}, func(err error) {
		assert.Error(t, err)
		assert.EqualError(t, err, "read error")
	}, func() {
		t.Error("Unexpected timeout")
	})

	err := conn.Close()
	assert.NoError(t, err)

}

func TestUDPConnWriteFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	local, remote := getTestAddr()

	udp := mock.NewMockUDPPacketConn(ctrl)
	dhcp4 := getDHCP4()

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 2, 0, 0)

	udp.EXPECT().
		LocalAddr().
		Return(local).
		AnyTimes()

	udp.EXPECT().
		RemoteAddr().
		Return(remote).
		AnyTimes()

	udp.EXPECT().
		Close().
		Return(nil)

	udp.EXPECT().
		SetWriteDeadline(gomock.Any())

	udp.EXPECT().
		Write(gomock.Any()).
		DoAndReturn(func(p []byte) (n int, err error) {
			return 0, fmt.Errorf("error write")
		})

	conn := dhcp.NewUDPConn(local, remote, udp, logger)
	assert.NotNil(t, conn)
	assert.Equal(t, conn.Local(), local)
	assert.Equal(t, conn.Remote(), remote)

	ctx := context.Background()
	c1, c2 := conn.Send(ctx, dhcp4)

	choice1(c1, c2, 10*time.Second, func(d int) {
		t.Error("Error expected")
	}, func(err error) {
		assert.Error(t, err)
		assert.EqualError(t, err, "error write")
	}, func() {
		t.Error("Unexpected timeout")
	})

	err := conn.Close()
	assert.NoError(t, err)
}

func TestUDPConnSendSetDeadlineFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	local, remote := getTestAddr()

	udp := mock.NewMockUDPPacketConn(ctrl)
	dhcp4 := getDHCP4()

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 2, 0, 0)

	udp.EXPECT().
		LocalAddr().
		Return(local).
		AnyTimes()

	udp.EXPECT().
		RemoteAddr().
		Return(remote).
		AnyTimes()

	udp.EXPECT().
		Close().
		Return(nil)

	udp.EXPECT().
		SetWriteDeadline(gomock.Any()).
		Return(fmt.Errorf("error sdl"))

	conn := dhcp.NewUDPConn(local, remote, udp, logger)
	assert.NotNil(t, conn)
	assert.Equal(t, conn.Local(), local)
	assert.Equal(t, conn.Remote(), remote)

	ctx := context.Background()
	c1, c2 := conn.Send(ctx, dhcp4)

	choice1(c1, c2, 10*time.Second, func(d int) {
		t.Error("Error expected")
	}, func(err error) {
		assert.Error(t, err)
		assert.EqualError(t, err, "error sdl")
	}, func() {
		t.Error("Unexpected timeout")
	})

	err := conn.Close()
	assert.NoError(t, err)
}

func TestUDPConnReceiveSetDeadlineFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	local, remote := getTestAddr()

	udp := mock.NewMockUDPPacketConn(ctrl)
	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 2, 0, 0)

	udp.EXPECT().
		LocalAddr().
		Return(local).
		AnyTimes()

	udp.EXPECT().
		RemoteAddr().
		Return(remote).
		AnyTimes()

	udp.EXPECT().
		Close().
		Return(nil)

	udp.EXPECT().
		SetReadDeadline(gomock.Any()).
		Return(fmt.Errorf("error sdl"))

	conn := dhcp.NewUDPConn(local, remote, udp, logger)
	assert.NotNil(t, conn)
	assert.Equal(t, conn.Local(), local)
	assert.Equal(t, conn.Remote(), remote)

	ctx := context.Background()
	c1, c2 := conn.Receive(ctx)

	choice2(c1, c2, 10*time.Second, func(d *dhcp.DHCP4) {
		t.Error("Error expected")
	}, func(err error) {
		assert.Error(t, err)
		assert.EqualError(t, err, "error sdl")
	}, func() {
		t.Error("Unexpected timeout")
	})

	err := conn.Close()
	assert.NoError(t, err)
}

func TestUDPConnBlock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	local, remote := getTestAddr()

	udp := mock.NewMockUDPPacketConn(ctrl)
	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 2, 0, 0)

	udp.EXPECT().
		LocalAddr().
		Return(local).AnyTimes()

	udp.EXPECT().
		RemoteAddr().
		Return(remote).AnyTimes()

	udp.EXPECT().
		Close().
		Return(nil)

	udp.EXPECT().
		SetWriteDeadline(gomock.Any())

	udp.EXPECT().
		Write(gomock.Any()).
		DoAndReturn(func(p []byte) (n int, err error) {
			return 0, nil
		})

	conn := dhcp.NewUDPConn(local, remote, udp, logger)
	assert.NotNil(t, conn)
	assert.Equal(t, conn.Local(), local)
	assert.Equal(t, conn.Remote(), remote)

	dhcp4 := getDHCP4()

	ctx, cancel := context.WithCancel(context.Background())
	conn.Block(ctx)

	ctx2 := context.Background()
	c1, c2 := conn.Send(ctx2, dhcp4)

	choice1(c1, c2, 200*time.Millisecond, func(d int) {
		t.Error("Error expected")
	}, func(err error) {
		assert.NoError(t, err)
		t.Error(err)
	}, func() {
	})

	cancel()

	choice1(c1, c2, 10*time.Second, func(d int) {
	}, func(err error) {
		assert.NoError(t, err)
		t.Error(err)
	}, func() {
		t.Error("Unexpected timeout")
	})

	err := conn.Close()
	assert.NoError(t, err)
}

func TestUDPConnCancelSendContext(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	local, remote := getTestAddr()

	udp := mock.NewMockUDPPacketConn(ctrl)
	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 2, 1, 0)

	udp.EXPECT().
		LocalAddr().
		Return(local).AnyTimes()

	udp.EXPECT().
		RemoteAddr().
		Return(remote).AnyTimes()

	udp.EXPECT().
		Close().
		Return(nil)

	udp.EXPECT().
		SetWriteDeadline(gomock.Any()).
		DoAndReturn(func(t time.Time) error {
			fmt.Printf("set deadline %v\n", t)
			return nil
		}).
		Times(2)

	udp.EXPECT().
		Write(gomock.Any()).
		DoAndReturn(func(p []byte) (n int, err error) {
			time.Sleep(5 * time.Second)
			return 0, nil
		})

	conn := dhcp.NewUDPConn(local, remote, udp, logger)
	assert.NotNil(t, conn)
	assert.Equal(t, conn.Local(), local)
	assert.Equal(t, conn.Remote(), remote)

	dhcp4 := getDHCP4()

	ctx, cancel := context.WithCancel(context.Background())
	c1, c2 := conn.Send(ctx, dhcp4)

	time.Sleep(100 * time.Millisecond)

	cancel()

	choice1(c1, c2, 10*time.Second, func(d int) {
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

func TestUDPConnCancelReceiveContext(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	local, remote := getTestAddr()

	udp := mock.NewMockUDPPacketConn(ctrl)
	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 2, 1, 0)

	udp.EXPECT().
		LocalAddr().
		Return(local).AnyTimes()

	udp.EXPECT().
		RemoteAddr().
		Return(remote).AnyTimes()

	udp.EXPECT().
		Close().
		Return(nil)

	udp.EXPECT().
		SetReadDeadline(gomock.Any()).
		DoAndReturn(func(t time.Time) error {
			fmt.Printf("set deadline %v\n", t)
			return nil
		}).
		Times(2)

	udp.EXPECT().ReadFromUDP(gomock.Any()).
		DoAndReturn(func(p []byte) (int, *net.UDPAddr, error) {
			time.Sleep(5 * time.Second)
			return 0, nil, nil
		})

	conn := dhcp.NewUDPConn(local, remote, udp, logger)
	assert.NotNil(t, conn)
	assert.Equal(t, conn.Local(), local)
	assert.Equal(t, conn.Remote(), remote)

	ctx, cancel := context.WithCancel(context.Background())
	c1, c2 := conn.Receive(ctx)

	time.Sleep(50 * time.Millisecond)

	cancel()

	choice2(c1, c2, 10*time.Second, func(d *dhcp.DHCP4) {
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

func choice1(c1 chan int, c2 chan error, timeout time.Duration, f1 func(int), f2 func(error), f3 func()) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case l := <-c1:
		f1(l)
	case err := <-c2:
		f2(err)
	case <-timer.C:
		f3()
	}
}

func choice2(c1 chan *dhcp.DHCP4, c2 chan error, timeout time.Duration, f1 func(*dhcp.DHCP4), f2 func(error), f3 func()) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case l := <-c1:
		f1(l)
	case err := <-c2:
		f2(err)
	case <-timer.C:
		f3()
	}
}
