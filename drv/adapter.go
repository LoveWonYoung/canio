package drv

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/LoveWonYoung/isotp/tp"
)

// _toomoss_adapter.go (与 main.go 放在同一目录下)

// ToomossAdapter 是连接 go-uds 和 Toomoss 硬件的适配器
type ToomossAdapter struct {
	driver CANDriver // 使用接口，使其可以同时支持 CAN 和 CAN-FD
	rxChan <-chan UnifiedCANMessage
}

// NewToomossAdapter 是适配器的构造函数
func NewToomossAdapter(dev CANDriver) (*ToomossAdapter, error) {
	if dev == nil {
		return nil, errors.New("CAN driver instance cannot be nil")
	}
	if err := dev.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize toomoss device: %w", err)
	}
	dev.Start()

	adapter := &ToomossAdapter{
		driver: dev,
		rxChan: dev.RxChan(),
	}

	log.Println("Toomoss-Adapter created and device started successfully.")
	return adapter, nil
}

// Close 用于停止驱动并释放资源
func (t *ToomossAdapter) Close() {
	log.Println("Closing Toomoss-Adapter...")
	t.driver.Stop()
}

// TxFunc adapts tp.TxFn for the driver.
func (t *ToomossAdapter) TxFunc(msg *tp.CanMessage) {
	if msg == nil {
		return
	}
	err := t.driver.Write(int32(msg.ArbitrationId), msg.Data)
	if err != nil {
		log.Printf("ERROR: ToomossAdapter failed to send message: %v", err)
	}
}

// RxFunc adapts tp.RxFn for the driver.
func (t *ToomossAdapter) RxFunc(timeout float64) *tp.CanMessage {
	if timeout <= 0 {
		select {
		case <-t.driver.Context().Done():
			return nil
		case receivedMsg, ok := <-t.rxChan:
			if !ok {
				return nil
			}
			return t.toTpMessage(receivedMsg)
		default:
			return nil
		}
	}

	var timeoutCh <-chan time.Time
	timer := time.NewTimer(time.Duration(timeout * float64(time.Second)))
	defer timer.Stop()
	timeoutCh = timer.C

	select {
	case <-t.driver.Context().Done():
		return nil
	case <-timeoutCh:
		return nil
	case receivedMsg, ok := <-t.rxChan:
		if !ok {
			return nil
		}
		return t.toTpMessage(receivedMsg)
	}
}

func (t *ToomossAdapter) toTpMessage(receivedMsg UnifiedCANMessage) *tp.CanMessage {
	payloadLength := int(receivedMsg.DLC)
	if payloadLength < 0 {
		return nil
	}

	if payloadLength > len(receivedMsg.Data) {
		log.Printf("警告: 收到的报文DLC (%d) 大于数据数组长度 (%d)。ID: 0x%X", receivedMsg.DLC, len(receivedMsg.Data), receivedMsg.ID)
		payloadLength = len(receivedMsg.Data)
	}

	data := make([]byte, payloadLength)
	copy(data, receivedMsg.Data[:payloadLength])

	return &tp.CanMessage{
		ArbitrationId: int(receivedMsg.ID),
		Dlc:           payloadLength,
		Data:          data,
		ExtendedId:    false,
		IsFd:          receivedMsg.IsFD,
		BitrateSwitch: false,
	}
}
