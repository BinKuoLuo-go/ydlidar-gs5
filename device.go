package gs5

import (
	"fmt"
	"io"
	"log"
	"time"

	"go.bug.st/serial.v1"
)

// Device 表示一个已连接的 GS5 设备（可能包含级联的多个模组）
//
// 注意：Device 的方法并非并发安全，串口读写应限定在单个 goroutine 中调用
// 如需并发控制，请调用方自行加锁。
type Device struct {
	port     serial.Port
	portName string
	baud     int

	params   map[byte]*DeviceParams // 各设备地址对应的参数
	addrs    []byte                 // 级联设备地址列表（0x01/0x02/0x04）
	scanning bool

	logger *log.Logger
}

// Option 配置项。
type Option func(*Device)

// WithBaudRate 设置串口波特率（默认 921600）。
func WithBaudRate(baud int) Option {
	return func(d *Device) { d.baud = baud }
}

// WithLogger 设置日志输出（默认不输出）。
func WithLogger(l *log.Logger) Option {
	return func(d *Device) {
		if l != nil {
			d.logger = l
		}
	}
}

// Open 打开串口并初始化 GS5（推荐流程：获取地址 → 获取参数，见开发手册 6-3）。
func Open(portName string, opts ...Option) (*Device, error) {
	d := &Device{
		portName: portName,
		baud:     DefaultBaudRate,
		params:   make(map[byte]*DeviceParams),
		logger:   log.New(io.Discard, "", 0),
	}
	for _, opt := range opts {
		opt(d)
	}
	port, err := openSerial(portName, d.baud)
	if err != nil {
		return nil, err
	}
	d.port = port

	if err := d.init(); err != nil {
		_ = port.Close()
		return nil, err
	}
	return d, nil
}

// init 获取级联设备地址与各设备参数。
func (d *Device) init() error {
	// 1. 获取设备地址，确定级联个数
	addr, err := d.GetDeviceAddress()
	if err != nil {
		return fmt.Errorf("获取设备地址失败: %w", err)
	}
	switch addr {
	case AddrDev1:
		d.addrs = []byte{AddrDev1}
	case AddrDev2:
		d.addrs = []byte{AddrDev1, AddrDev2}
	case AddrDev3:
		d.addrs = []byte{AddrDev1, AddrDev2, AddrDev3}
	default:
		return fmt.Errorf("未知设备地址 0x%02X", addr)
	}
	// 2. 获取各设备参数
	params, err := d.GetParams()
	if err != nil {
		return fmt.Errorf("获取设备参数失败: %w", err)
	}
	d.params = params
	return nil
}

// Addresses 返回级联设备地址列表。
func (d *Device) Addresses() []byte {
	return append([]byte(nil), d.addrs...)
}

// Params 返回指定地址设备的参数（nil 表示未获取）。
func (d *Device) Params(addr byte) *DeviceParams {
	return d.params[addr]
}

// IsScanning 返回当前是否处于扫描模式。
func (d *Device) IsScanning() bool {
	return d.scanning
}

// GetDeviceAddress 获取级联设备地址（返回最大地址，0x01/0x02/0x04 分别对应 1/2/3 个模组）。
func (d *Device) GetDeviceAddress() (byte, error) {
	if d.scanning {
		return 0, ErrScanning
	}
	addr, cmd, _, err := d.command(AddrAll, CmdGetAddress, nil, waitGetAddress)
	if err != nil {
		return 0, err
	}
	if cmd != CmdGetAddress {
		return 0, fmt.Errorf("应答命令码不匹配: 期望 0x%02X, 实际 0x%02X", CmdGetAddress, cmd)
	}
	return addr, nil
}

// GetVersion 获取所有级联设备的版本信息（发送 0x62 广播，级联时返回多个应答）。
func (d *Device) GetVersion() ([]*VersionInfo, error) {
	if d.scanning {
		return nil, ErrScanning
	}
	if err := d.writeCommand(AddrAll, CmdGetVersion, nil); err != nil {
		return nil, err
	}
	time.Sleep(waitGetVersion)

	infos := make([]*VersionInfo, 0, len(d.addrs))
	for range d.addrs {
		addr, cmd, data, err := d.readPacket()
		if err != nil {
			return nil, err
		}
		if cmd != CmdGetVersion {
			return nil, fmt.Errorf("应答命令码不匹配: 期望 0x%02X, 实际 0x%02X", CmdGetVersion, cmd)
		}
		info, err := parseVersion(addr, data)
		if err != nil {
			return nil, err
		}
		infos = append(infos, info)
	}
	return infos, nil
}

// GetParams 获取所有级联设备的计算参数（发送 0x61 广播，级联时返回多个应答）。
func (d *Device) GetParams() (map[byte]*DeviceParams, error) {
	if d.scanning {
		return nil, ErrScanning
	}
	if err := d.writeCommand(AddrAll, CmdGetParams, nil); err != nil {
		return nil, err
	}
	time.Sleep(waitGetParams)

	out := make(map[byte]*DeviceParams, len(d.addrs))
	for range d.addrs {
		addr, cmd, data, err := d.readPacket()
		if err != nil {
			return nil, err
		}
		if cmd != CmdGetParams {
			return nil, fmt.Errorf("应答命令码不匹配: 期望 0x%02X, 实际 0x%02X", CmdGetParams, cmd)
		}
		p, err := parseParams(data)
		if err != nil {
			return nil, err
		}
		out[addr] = p
	}
	return out, nil
}

// StartScan 进入扫描模式，开始输出点云数据（发送 0x63 广播，启动所有设备）。
func (d *Device) StartScan() error {
	if d.scanning {
		return ErrScanning
	}
	if err := d.writeCommand(AddrAll, CmdStartScan, nil); err != nil {
		return err
	}
	time.Sleep(waitStartScan)
	// 扫描启动应答为数据长度 0 的报文；跳过可能残留的点云帧
	for i := 0; i < 200; i++ {
		_, cmd, data, err := d.readPacket()
		if err != nil {
			return err
		}
		if cmd == CmdStartScan && len(data) == 0 {
			d.scanning = true
			return nil
		}
	}
	return fmt.Errorf("未收到扫描启动应答")
}

// StopScan 停止扫描，设备进入待机休眠状态（发送 0x64 广播，停止所有设备）。
func (d *Device) StopScan() error {
	if !d.scanning {
		return ErrNotScanning
	}
	if err := d.stopScanLocked(); err != nil {
		return err
	}
	return nil
}

// GrabFrame 读取一帧点云数据（阻塞，直到读取到一帧）。
// 级联模式下，不同设备的数据帧会交织出现，可循环调用本方法获取各设备数据。
func (d *Device) GrabFrame() (*Frame, error) {
	if !d.scanning {
		return nil, ErrNotScanning
	}
	for {
		addr, cmd, data, err := d.readPacket()
		if err != nil {
			return nil, err
		}
		if cmd != CmdStartScan {
			d.debugf("忽略非点云报文 cmd=0x%02X", cmd)
			continue
		}
		p, ok := d.params[addr]
		if !ok {
			return nil, fmt.Errorf("设备 0x%02X 未获取到参数", addr)
		}
		return parseFrame(addr, data, p)
	}
}

// SoftReset 软重启指定地址的模组（Address 只能是 0x01/0x02/0x04）。
func (d *Device) SoftReset(addr byte) error {
	if addr != AddrDev1 && addr != AddrDev2 && addr != AddrDev3 {
		return fmt.Errorf("无效复位地址 0x%02X（只能是 0x01/0x02/0x04）", addr)
	}
	_, cmd, _, err := d.command(addr, CmdSoftReset, nil, waitSoftReset)
	if err != nil {
		return err
	}
	if cmd != CmdSoftReset {
		return fmt.Errorf("应答命令码不匹配: 期望 0x%02X, 实际 0x%02X", CmdSoftReset, cmd)
	}
	d.scanning = false
	return nil
}

// SetBaudRate 设置串口波特率（发送 0x68 广播），随后软重启设备并重配串口。
// 三模块级联时波特率需 ≥ 921600
func (d *Device) SetBaudRate(code uint8) error {
	if d.scanning {
		return ErrScanning
	}
	newBaud, ok := baudRateValue(code)
	if !ok {
		return fmt.Errorf("无效波特率代号 %d（需 0~3）", code)
	}
	if len(d.addrs) == 3 && code < Baud921600 {
		return fmt.Errorf("三模块级联需波特率 ≥ 921600（代号 2/3），当前代号 %d", code)
	}
	// 1. 设置波特率
	if err := d.writeCommand(AddrAll, CmdSetBaudRate, []byte{code}); err != nil {
		return err
	}
	time.Sleep(waitSetBaudRate)
	_, cmd, data, err := d.readPacket()
	if err != nil {
		return err
	}
	if cmd != CmdSetBaudRate {
		return fmt.Errorf("应答命令码不匹配: 期望 0x%02X, 实际 0x%02X", CmdSetBaudRate, cmd)
	}
	if len(data) < 1 || data[0] != code {
		return fmt.Errorf("波特率设置应答异常: 回显 0x%02X, 期望 0x%02X", data[0], code)
	}
	// 2. 软重启各模组（仍在旧波特率下通信，重启后生效新波特率）
	for _, a := range d.addrs {
		if _, _, _, err := d.command(a, CmdSoftReset, nil, waitSoftReset); err != nil {
			return fmt.Errorf("软重启设备 0x%02X 失败: %w", a, err)
		}
	}
	// 3. 重配串口波特率
	if err := d.setPortBaud(newBaud); err != nil {
		return fmt.Errorf("重配串口波特率失败: %w", err)
	}
	d.baud = newBaud
	d.debugf("波特率已设置为 %d", newBaud)
	return nil
}

// setPortBaud 重配串口波特率。
func (d *Device) setPortBaud(baud int) error {
	return d.port.SetMode(&serial.Mode{
		BaudRate: baud,
		DataBits: 8,
		StopBits: serial.OneStopBit,
		Parity:   serial.NoParity,
	})
}

// Close 关闭设备（若正在扫描则先停止，再关闭串口）。
func (d *Device) Close() error {
	if d.scanning {
		_ = d.stopScanLocked()
	}
	return d.port.Close()
}

// command 发送命令并读取单次应答（发送 + 延时 + 读取）。
func (d *Device) command(addr, cmd byte, data []byte, wait time.Duration) (byte, byte, []byte, error) {
	if err := d.writeCommand(addr, cmd, data); err != nil {
		return 0, 0, nil, err
	}
	time.Sleep(wait)
	return d.readPacket()
}

// writeCommand 组装并发送命令。
func (d *Device) writeCommand(addr, cmd byte, data []byte) error {
	pkt := buildPacket(addr, cmd, data)
	d.debugf("TX cmd=0x%02X %X", cmd, pkt)
	_, err := d.port.Write(pkt)
	return err
}

// readPacket 读取一个完整报文。
func (d *Device) readPacket() (byte, byte, []byte, error) {
	addr, cmd, data, err := readPacket(d.port)
	if err == nil {
		d.debugf("RX addr=0x%02X cmd=0x%02X len=%d", addr, cmd, len(data))
	}
	return addr, cmd, data, err
}

// stopScanLocked 停止扫描（内部使用）。
func (d *Device) stopScanLocked() error {
	if err := d.writeCommand(AddrAll, CmdStopScan, nil); err != nil {
		return err
	}
	time.Sleep(waitStopScan)
	// 扫描中串口可能残留点云帧，循环读取直到收到停止应答
	for i := 0; i < 200; i++ {
		_, cmd, _, err := d.readPacket()
		if err != nil {
			return err
		}
		if cmd == CmdStopScan {
			d.scanning = false
			return nil
		}
	}
	return fmt.Errorf("未收到停止扫描应答")
}

func (d *Device) debugf(format string, args ...any) {
	d.logger.Printf(format, args...)
}
