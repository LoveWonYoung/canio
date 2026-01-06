//go:build !windows
// +build !windows

package drv

import (
	"context"
	"time"
)

// CanType keeps parity with the Windows implementation.
type CanType byte

const (
	CAN CanType = iota
	CANFD
)

// MockCAN is a lightweight CAN/CAN-FD stub for local builds and simple tests.
// It echoes writes into an optional responder and exposes an inject helper to
// push frames into the receive channel.
type MockCAN struct {
	rxChan    chan UnifiedCANMessage
	ctx       context.Context
	cancel    context.CancelFunc
	responder func(id int32, data []byte)
}

// NewCanMix returns a mock driver; canType is kept for API compatibility.
func NewCanMix(canType CanType) *MockCAN {
	ctx, cancel := context.WithCancel(context.Background())
	return &MockCAN{
		rxChan: make(chan UnifiedCANMessage, 16),
		ctx:    ctx,
		cancel: cancel,
	}
}

func (m *MockCAN) Init() error { return nil }
func (m *MockCAN) Start()      {}

func (m *MockCAN) Stop() {
	m.cancel()
	close(m.rxChan)
}

func (m *MockCAN) RxChan() <-chan UnifiedCANMessage { return m.rxChan }
func (m *MockCAN) Context() context.Context         { return m.ctx }

// Write forwards to an optional responder; otherwise it is a no-op.
func (m *MockCAN) Write(id int32, data []byte) error {
	if m.responder != nil {
		copyData := append([]byte(nil), data...)
		go m.responder(id, copyData)
	}
	return nil
}

// SetResponder lets callers simulate ECU behavior when a frame is written.
func (m *MockCAN) SetResponder(fn func(id int32, data []byte)) {
	m.responder = fn
}

// InjectRx pushes a frame into the receive channel for the adapter to consume.
func (m *MockCAN) InjectRx(id uint32, dlc byte, data []byte, isFD bool) {
	var buf [64]byte
	copy(buf[:], data)
	msg := UnifiedCANMessage{
		ID:   id,
		DLC:  dlc,
		Data: buf,
		IsFD: isFD,
	}
	select {
	case <-m.ctx.Done():
	case m.rxChan <- msg:
	}
}

// InjectRxAfter delays before injecting, convenient for timing tests.
func (m *MockCAN) InjectRxAfter(delay time.Duration, id uint32, data []byte, isFD bool) {
	dlc := byte(len(data))
	time.AfterFunc(delay, func() { m.InjectRx(id, dlc, data, isFD) })
}
