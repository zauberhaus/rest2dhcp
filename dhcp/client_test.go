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
	"encoding/json"
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

func TestStartStop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	local := net.IP{1, 1, 1, 1}
	remote := net.IP{2, 2, 2, 2}

	conn := mock.NewMockConnection(ctrl)
	ipResolver := mock.NewMockIPResolver(local, remote, local)
	connResolver := mock.NewMockConnectionResolver(conn)

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 6, 5, 0, 0)

	conn.EXPECT().Receive(gomock.Any()).DoAndReturn(func(ctx context.Context) (chan *dhcp.DHCP4, chan error) {
		chan1 := make(chan *dhcp.DHCP4)
		chan2 := make(chan error)

		go func() {
			select {
			case <-ctx.Done():
				close(chan1)
			}
		}()

		return chan1, chan2
	})

	conn.EXPECT().Block(gomock.Any()).AnyTimes()

	conn.EXPECT().Close().Times(1)

	client := dhcp.NewClient(ipResolver, connResolver, dhcp.UDP, 10*time.Second, 10*time.Second, logger)

	<-client.Start()

	client.Stop()

}

func TestGetLease(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	local := net.IP{1, 1, 1, 1}
	remote := net.IP{2, 2, 2, 2}

	conn := mock.NewMockConnection(ctrl)
	ipResolver := mock.NewMockIPResolver(local, remote, local)
	connResolver := mock.NewMockConnectionResolver(conn)

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 6, 22, 0, 8)

	chan1 := make(chan *dhcp.DHCP4)
	chan2 := make(chan error)

	count := 0

	conn.EXPECT().Block(gomock.Any()).AnyTimes()

	conn.EXPECT().Close().DoAndReturn(func() error {
		logger.Test("Close connection")
		return nil
	}).Times(1)

	conn.EXPECT().Send(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, d *dhcp.DHCP4) (chan int, chan error) {
			chan3 := make(chan int)
			chan4 := make(chan error)

			logger.Testf("send %v", d.GetMsgType())

			go func() {
				chan3 <- 1
			}()

			go func() {
				time.Sleep(10 * time.Millisecond)

				if d.GetMsgType() == layers.DHCPMsgTypeDiscover {
					var packet dhcp.DHCP4
					data, err := ioutil.ReadFile("./testdata/Offer.json")
					assert.NoError(t, err)
					json.Unmarshal(data, &packet)
					packet.DHCPv4.Xid = d.Xid
					logger.Testf("receive %v", packet.GetMsgType())
					chan1 <- &packet
				} else if d.GetMsgType() == layers.DHCPMsgTypeRequest {
					var packet dhcp.DHCP4
					data, err := ioutil.ReadFile("./testdata/Ack.json")
					assert.NoError(t, err)
					json.Unmarshal(data, &packet)
					packet.DHCPv4.Xid = d.Xid
					logger.Testf("receive %v", packet.GetMsgType())
					chan1 <- &packet
				}

				count++
			}()

			return chan3, chan4
		}).AnyTimes()

	conn.EXPECT().Receive(gomock.Any()).DoAndReturn(func(ctx context.Context) (chan *dhcp.DHCP4, chan error) {
		return chan1, chan2
	}).AnyTimes()

	client := dhcp.NewClient(ipResolver, connResolver, dhcp.UDP, 10*time.Second, 100*time.Second, logger)

	<-client.Start()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	lease := <-client.GetLease(ctx, "hostname", net.HardwareAddr{0, 1, 2, 3, 4, 5})

	assert.NotNil(t, lease)
	assert.Equal(t, net.IP{192, 168, 1, 46}, lease.YourClientIP.To4())
	assert.Equal(t, layers.DHCPMsgTypeAck, lease.GetMsgType())

	assert.Equal(t, net.IP{192, 168, 1, 1}, lease.GetRouter())
	assert.Equal(t, net.IP{192, 168, 1, 1}, lease.GetDNS())
	assert.Equal(t, net.IP{255, 255, 252, 0}, lease.GetSubnetMask())

	assert.Less(t, time.Now().Add(10*24*time.Hour).Sub(lease.GetExpireTime()), 1*time.Second)
	assert.Less(t, time.Now().Add(210*time.Hour).Sub(lease.GetRebindTime()), 1*time.Second)
	assert.Less(t, time.Now().Add(5*24*time.Hour).Sub(lease.GetRenewalTime()), 1*time.Second)

	lease = <-client.Renew(context.Background(), "hostname", net.HardwareAddr{0, 1, 2, 3, 4, 5}, lease.YourClientIP)

	assert.NotNil(t, lease)
	assert.Equal(t, net.IP{192, 168, 1, 46}, lease.YourClientIP.To4())

	err := <-client.Release(context.Background(), "hostname", net.HardwareAddr{0, 1, 2, 3, 4, 5}, lease.YourClientIP)
	assert.NoError(t, err)

	client.Stop()

}

func TestGetLeaseSlow(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	local := net.IP{1, 1, 1, 1}
	remote := net.IP{2, 2, 2, 2}

	conn := mock.NewMockConnection(ctrl)
	ipResolver := mock.NewMockIPResolver(local, remote, local)
	connResolver := mock.NewMockConnectionResolver(conn)

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 9, 19, 0, 8)

	chan1 := make(chan *dhcp.DHCP4)
	chan2 := make(chan error)

	count := 0

	conn.EXPECT().Block(gomock.Any()).AnyTimes()

	conn.EXPECT().Close().DoAndReturn(func() error {
		logger.Test("Close connection")
		return nil
	}).Times(1)

	conn.EXPECT().Send(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, d *dhcp.DHCP4) (chan int, chan error) {
			chan3 := make(chan int)
			chan4 := make(chan error)

			logger.Testf("send %v", d.GetMsgType())

			go func() {
				chan3 <- 1
			}()

			go func() {
				time.Sleep(150 * time.Millisecond)

				if d.GetMsgType() == layers.DHCPMsgTypeDiscover {
					var packet dhcp.DHCP4
					data, err := ioutil.ReadFile("./testdata/Offer.json")
					assert.NoError(t, err)
					json.Unmarshal(data, &packet)
					packet.DHCPv4.Xid = d.Xid
					logger.Testf("receive %v", packet.GetMsgType())
					chan1 <- &packet
				} else if d.GetMsgType() == layers.DHCPMsgTypeRequest {
					var packet dhcp.DHCP4
					data, err := ioutil.ReadFile("./testdata/Ack.json")
					assert.NoError(t, err)
					json.Unmarshal(data, &packet)
					packet.DHCPv4.Xid = d.Xid
					logger.Testf("receive %v", packet.GetMsgType())
					chan1 <- &packet
				}

				count++
			}()

			return chan3, chan4
		}).AnyTimes()

	conn.EXPECT().Receive(gomock.Any()).DoAndReturn(func(ctx context.Context) (chan *dhcp.DHCP4, chan error) {
		return chan1, chan2
	}).AnyTimes()

	client := dhcp.NewClient(ipResolver, connResolver, dhcp.UDP, 100*time.Millisecond, 500*time.Millisecond, logger)

	<-client.Start()

	lease := <-client.GetLease(context.Background(), "hostname", net.HardwareAddr{0, 1, 2, 3, 4, 5})

	assert.NotNil(t, lease)
	assert.Equal(t, net.IP{192, 168, 1, 46}, lease.YourClientIP.To4())
	assert.Equal(t, layers.DHCPMsgTypeAck, lease.GetMsgType())

	assert.Equal(t, net.IP{192, 168, 1, 1}, lease.GetRouter())
	assert.Equal(t, net.IP{192, 168, 1, 1}, lease.GetDNS())
	assert.Equal(t, net.IP{255, 255, 252, 0}, lease.GetSubnetMask())

	assert.Less(t, time.Now().Add(10*24*time.Hour).Sub(lease.GetExpireTime()), 1*time.Second)
	assert.Less(t, time.Now().Add(210*time.Hour).Sub(lease.GetRebindTime()), 1*time.Second)
	assert.Less(t, time.Now().Add(5*24*time.Hour).Sub(lease.GetRenewalTime()), 1*time.Second)

	lease = <-client.Renew(context.Background(), "hostname", net.HardwareAddr{0, 1, 2, 3, 4, 5}, lease.YourClientIP)

	assert.NotNil(t, lease)
	assert.Equal(t, net.IP{192, 168, 1, 46}, lease.YourClientIP.To4())

	err := <-client.Release(context.Background(), "hostname", net.HardwareAddr{0, 1, 2, 3, 4, 5}, lease.YourClientIP)
	assert.NoError(t, err)

	client.Stop()
}

func TestGetLeaseVerySlow(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	local := net.IP{1, 1, 1, 1}
	remote := net.IP{2, 2, 2, 2}

	conn := mock.NewMockConnection(ctrl)
	ipResolver := mock.NewMockIPResolver(local, remote, local)
	connResolver := mock.NewMockConnectionResolver(conn)
	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 12, 25, 0, 11)

	chan1 := make(chan *dhcp.DHCP4)
	chan2 := make(chan error)

	count := 0

	conn.EXPECT().Block(gomock.Any()).AnyTimes()

	conn.EXPECT().Close().DoAndReturn(func() error {
		logger.Test("Close connection")
		return nil
	}).Times(1)

	conn.EXPECT().Send(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, d *dhcp.DHCP4) (chan int, chan error) {
			chan3 := make(chan int)
			chan4 := make(chan error)

			logger.Testf("send %v", d.GetMsgType())

			go func() {
				chan3 <- 1
			}()

			go func() {
				count++

				if count&1 == 1 {
					return
				} else {
					time.Sleep(10 * time.Millisecond)
				}

				if d.GetMsgType() == layers.DHCPMsgTypeDiscover {
					var packet dhcp.DHCP4
					data, err := ioutil.ReadFile("./testdata/Offer.json")
					assert.NoError(t, err)
					json.Unmarshal(data, &packet)
					packet.DHCPv4.Xid = d.Xid
					logger.Testf("receive %v", packet.GetMsgType())
					chan1 <- &packet
				} else if d.GetMsgType() == layers.DHCPMsgTypeRequest {
					var packet dhcp.DHCP4
					data, err := ioutil.ReadFile("./testdata/Ack.json")
					assert.NoError(t, err)
					json.Unmarshal(data, &packet)
					packet.DHCPv4.Xid = d.Xid
					logger.Testf("receive %v", packet.GetMsgType())
					chan1 <- &packet
				}
			}()

			return chan3, chan4
		}).AnyTimes()

	conn.EXPECT().Receive(gomock.Any()).DoAndReturn(func(ctx context.Context) (chan *dhcp.DHCP4, chan error) {
		return chan1, chan2
	}).AnyTimes()

	client := dhcp.NewClient(ipResolver, connResolver, dhcp.UDP, 300*time.Millisecond, 100*time.Millisecond, logger)

	<-client.Start()

	lease := <-client.GetLease(context.Background(), "hostname", net.HardwareAddr{0, 1, 2, 3, 4, 5})

	assert.NotNil(t, lease)
	assert.Equal(t, net.IP{192, 168, 1, 46}, lease.YourClientIP.To4())
	assert.Equal(t, layers.DHCPMsgTypeAck, lease.GetMsgType())

	assert.Equal(t, net.IP{192, 168, 1, 1}, lease.GetRouter())
	assert.Equal(t, net.IP{192, 168, 1, 1}, lease.GetDNS())
	assert.Equal(t, net.IP{255, 255, 252, 0}, lease.GetSubnetMask())

	assert.Less(t, time.Now().Add(10*24*time.Hour).Sub(lease.GetExpireTime()), 10*time.Second)
	assert.Less(t, time.Now().Add(210*time.Hour).Sub(lease.GetRebindTime()), 10*time.Second)
	assert.Less(t, time.Now().Add(5*24*time.Hour).Sub(lease.GetRenewalTime()), 10*time.Second)

	lease = <-client.Renew(context.Background(), "hostname", net.HardwareAddr{0, 1, 2, 3, 4, 5}, lease.YourClientIP)

	assert.NotNil(t, lease)
	assert.Equal(t, net.IP{192, 168, 1, 46}, lease.YourClientIP.To4())

	err := <-client.Release(context.Background(), "hostname", net.HardwareAddr{0, 1, 2, 3, 4, 5}, lease.YourClientIP)
	assert.NoError(t, err)

	client.Stop()
}

func TestRenew(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	local := net.IP{1, 1, 1, 1}
	remote := net.IP{2, 2, 2, 2}

	conn := mock.NewMockConnection(ctrl)
	ipResolver := mock.NewMockIPResolver(local, remote, local)
	connResolver := mock.NewMockConnectionResolver(conn)

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 3, 0, 6, 11, 0, 5)

	chan1 := make(chan *dhcp.DHCP4, 10)
	chan2 := make(chan error)

	conn.EXPECT().Block(gomock.Any()).AnyTimes()

	conn.EXPECT().Close().AnyTimes()

	conn.EXPECT().Send(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, d *dhcp.DHCP4) (chan int, chan error) {
			chan3 := make(chan int)
			chan4 := make(chan error)

			logger.Testf("send %v", d.GetMsgType())

			go func() {
				chan3 <- 1
			}()

			go func() {
				// Offer
				time.Sleep(10 * time.Millisecond)
				var packet dhcp.DHCP4
				data, err := ioutil.ReadFile("./testdata/Offer.json")
				assert.NoError(t, err)
				err = json.Unmarshal(data, &packet)
				assert.NoError(t, err)
				packet.DHCPv4.Xid = d.Xid
				logger.Testf("receive %v instead of ACK", packet.GetMsgType())
				chan1 <- &packet

				// Unknown Offer
				time.Sleep(10 * time.Millisecond)
				data, err = ioutil.ReadFile("./testdata/Offer.json")
				assert.NoError(t, err)
				err = json.Unmarshal(data, &packet)
				assert.NoError(t, err)
				logger.Testf("receive %v with invalid XID", packet.GetMsgType())
				chan1 <- &packet

				// Empty packet
				time.Sleep(10 * time.Millisecond)
				logger.Test("receive empty packet")
				time.Sleep(10 * time.Millisecond)
				chan1 <- nil

				// NAK
				time.Sleep(10 * time.Millisecond)
				data, err = ioutil.ReadFile("./testdata/Nak.json")
				assert.NoError(t, err)
				err = json.Unmarshal(data, &packet)
				assert.NoError(t, err)
				packet.DHCPv4.Xid = d.Xid
				logger.Testf("receive %v instead of ACK", packet.GetMsgType())
				chan1 <- &packet

			}()

			return chan3, chan4
		})

	conn.EXPECT().Receive(gomock.Any()).DoAndReturn(func(ctx context.Context) (chan *dhcp.DHCP4, chan error) {
		return chan1, chan2
	}).AnyTimes()

	client := dhcp.NewClient(ipResolver, connResolver, dhcp.UDP, 200*time.Millisecond, 200*time.Millisecond, logger)

	<-client.Start()

	lease := <-client.Renew(context.Background(), "hostname", nil, net.IP{1, 1, 1, 1})

	assert.NotNil(t, lease)
	assert.Equal(t, layers.DHCPMsgTypeNak, lease.GetMsgType())
	assert.Error(t, lease.Error())

	client.Stop()
}
