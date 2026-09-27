package gs5

import "time"

// 串口默认参数。
const (
	// DefaultBaudRate 默认串口波特率。
	DefaultBaudRate = 921600
)

// GS5 系统命令码
const (
	CmdGetAddress  byte = 0x60 // 获取设备地址
	CmdGetParams   byte = 0x61 // 获取计算参数
	CmdGetVersion  byte = 0x62 // 获取版本信息
	CmdStartScan   byte = 0x63 // 开始扫描，输出点云数据
	CmdStopScan    byte = 0x64 // 停机，停止扫描
	CmdSoftReset   byte = 0x67 // 设备软重启
	CmdSetBaudRate byte = 0x68 // 设置串口波特率
)

// GS5 设备地址 级联最大支持 3 个，地址固定为 0x01/0x02/0x04
const (
	AddrDev1 byte = 0x01 // 1 号设备
	AddrDev2 byte = 0x02 // 2 号设备
	AddrDev3 byte = 0x04 // 3 号设备
	AddrAll  byte = 0x00 // 广播地址（启动/停止所有设备）
)

// 报文结构常量 报文 = 包头(4) + 地址(1) + 命令(1) + 数据长度(2) + 数据段(N) + 校验码(1)
const (
	headerByte      byte = 0xA5                  // 包头单字节
	HeaderLen            = 4                     // 包头长度
	PacketHeaderLen      = HeaderLen + 1 + 1 + 2 // 包头+地址+命令+数据长度 = 8
)

// GS5 点云数据格式
const (
	// PointCount 单包测距点数量（S1~S160）。
	PointCount = 160
	// PointDataLen 单个测距点长度（字节，低 11 位距离 + 高 5 位强度）。
	PointDataLen = 2
	// EnvDataLen 环境数据长度（字节）。
	EnvDataLen = 2
	// PointCloudDataLen 点云数据段长度 = 环境(2) + 160 点(320) = 322 字节。
	PointCloudDataLen = EnvDataLen + PointCount*PointDataLen
	// PointCloudPacketLen 整包点云长度 = 包头(4)+地址(1)+命令(1)+长度(2)+数据段(322)+校验(1) = 331 字节。
	PointCloudPacketLen = PacketHeaderLen + PointCloudDataLen + 1
)

// 波特率代号
const (
	Baud230400  uint8 = 0 // 230400 bps
	Baud512000  uint8 = 1 // 512000 bps
	Baud921600  uint8 = 2 // 921600 bps
	Baud1500000 uint8 = 3 // 1500000 bps
)

// 各指令发送后的最大等待延时
const (
	waitGetAddress  = 800 * time.Millisecond // 获取地址
	waitGetVersion  = 100 * time.Millisecond // 获取版本
	waitGetParams   = 100 * time.Millisecond // 获取参数
	waitStartScan   = 400 * time.Millisecond // 开启扫描
	waitStopScan    = 100 * time.Millisecond // 停止扫描
	waitSetBaudRate = 800 * time.Millisecond // 设置波特率
	waitSoftReset   = 800 * time.Millisecond // 软重启
)
