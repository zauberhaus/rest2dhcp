// Code generated by MockGen. DO NOT EDIT.
// Source: ../dhcp/client.go

// Package mock is a generated GoMock package.
package mock

import (
	context "context"
	net "net"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	dhcp "github.com/zauberhaus/rest2dhcp/dhcp"
)

// MockDHCPClient is a mock of DHCPClient interface.
type MockDHCPClient struct {
	ctrl     *gomock.Controller
	recorder *MockDHCPClientMockRecorder
}

// MockDHCPClientMockRecorder is the mock recorder for MockDHCPClient.
type MockDHCPClientMockRecorder struct {
	mock *MockDHCPClient
}

// NewMockDHCPClient creates a new mock instance.
func NewMockDHCPClient(ctrl *gomock.Controller) *MockDHCPClient {
	mock := &MockDHCPClient{ctrl: ctrl}
	mock.recorder = &MockDHCPClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockDHCPClient) EXPECT() *MockDHCPClientMockRecorder {
	return m.recorder
}

// GetLease mocks base method.
func (m *MockDHCPClient) GetLease(ctx context.Context, hostname string, chaddr net.HardwareAddr) chan *dhcp.Lease {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetLease", ctx, hostname, chaddr)
	ret0, _ := ret[0].(chan *dhcp.Lease)
	return ret0
}

// GetLease indicates an expected call of GetLease.
func (mr *MockDHCPClientMockRecorder) GetLease(ctx, hostname, chaddr interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetLease", reflect.TypeOf((*MockDHCPClient)(nil).GetLease), ctx, hostname, chaddr)
}

// Release mocks base method.
func (m *MockDHCPClient) Release(ctx context.Context, hostname string, chaddr net.HardwareAddr, ip net.IP) chan error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Release", ctx, hostname, chaddr, ip)
	ret0, _ := ret[0].(chan error)
	return ret0
}

// Release indicates an expected call of Release.
func (mr *MockDHCPClientMockRecorder) Release(ctx, hostname, chaddr, ip interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Release", reflect.TypeOf((*MockDHCPClient)(nil).Release), ctx, hostname, chaddr, ip)
}

// Renew mocks base method.
func (m *MockDHCPClient) Renew(ctx context.Context, hostname string, chaddr net.HardwareAddr, ip net.IP) chan *dhcp.Lease {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Renew", ctx, hostname, chaddr, ip)
	ret0, _ := ret[0].(chan *dhcp.Lease)
	return ret0
}

// Renew indicates an expected call of Renew.
func (mr *MockDHCPClientMockRecorder) Renew(ctx, hostname, chaddr, ip interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Renew", reflect.TypeOf((*MockDHCPClient)(nil).Renew), ctx, hostname, chaddr, ip)
}

// Start mocks base method.
func (m *MockDHCPClient) Start() chan bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Start")
	ret0, _ := ret[0].(chan bool)
	return ret0
}

// Start indicates an expected call of Start.
func (mr *MockDHCPClientMockRecorder) Start() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockDHCPClient)(nil).Start))
}

// Stop mocks base method.
func (m *MockDHCPClient) Stop() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Stop")
}

// Stop indicates an expected call of Stop.
func (mr *MockDHCPClientMockRecorder) Stop() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockDHCPClient)(nil).Stop))
}
