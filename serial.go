package gs5

import (
	"fmt"

	"go.bug.st/serial.v1"
)

// openSerial 以指定波特率打开串口（8 数据位、1 停止位、无校验）。
func openSerial(portName string, baud int) (serial.Port, error) {
	mode := &serial.Mode{
		BaudRate: baud,
		DataBits: 8,
		StopBits: serial.OneStopBit,
		Parity:   serial.NoParity,
	}
	port, err := serial.Open(portName, mode)
	if err != nil {
		return nil, fmt.Errorf("打开串口 %s 失败: %w", portName, err)
	}
	return port, nil
}

// baudRateValue 返回波特率代号对应的实际波特率值。
func baudRateValue(code uint8) (int, bool) {
	v, ok := map[uint8]int{
		Baud230400:  230400,
		Baud512000:  512000,
		Baud921600:  921600,
		Baud1500000: 1500000,
	}[code]
	return v, ok
}
