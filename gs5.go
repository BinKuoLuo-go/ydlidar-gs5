// Package gs5 提供 YDLIDAR GS5 固态激光雷达的 Go SDK。
//
// GS5 是一款近距离固态雷达（测距范围 70~1000mm），通过串口与外部设备交互。
// 本包实现了开发手册定义的完整系统命令（获取地址/版本/参数、扫描、停止、复位、设置波特率）
// 以及点云数据解析，可供上位机直接集成。
//
// 典型用法：
//
//	dev, err := gs5.Open("COM3")
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer dev.Close()
//
//	if err := dev.StartScan(); err != nil {
//		log.Fatal(err)
//	}
//	for {
//		frame, err := dev.GrabFrame()
//		if err != nil {
//			log.Fatal(err)
//		}
//		for _, p := range frame.Points {
//			// 处理 p.Angle / p.Distance / p.Intensity ...
//		}
//	}
//
// 注意：Device 的方法并非并发安全，串口读写应限定在单个 goroutine 中调用。
package gs5

import (
	"errors"
	"fmt"
)

// DeviceParams 设备角度参数 用于点云解析
// K、B 字段由 uint16 原始值除以 10000 得到，Bias 由 int8 原始值除以 10 得到。
type DeviceParams struct {
	K0   float64 // 相机角度参数 k0
	B0   float64 // 相机角度参数 b0
	K1   float64 // 相机角度参数 k1
	B1   float64 // 相机角度参数 b1
	Bias float64 // 相机角度偏差 bias
}

// Point 单个测距点。
type Point struct {
	Index     int     // 点序号 0~159（S1~S160：L1~L80 为左相机，R1~R80 为右相机）
	Angle     float64 // 角度（度），以模组正前方为 0°，顺时针增大
	Distance  float64 // 距离（mm）
	Intensity uint8   // 强度（高 5 位，0~31）
}

// Frame 一帧点云数据。
type Frame struct {
	Address byte    // 设备地址（0x01/0x02/0x04）
	Env     uint16  // 环境光强度（2 字节）
	Points  []Point // 160 个测距点
}

// VersionInfo 设备版本信息
type VersionInfo struct {
	Address   byte   // 设备地址
	HWVersion uint8  // 硬件版本号
	FWVersion uint16 // 固件版本号（大端序）
	SerialNo  []byte // 序列号（16 字节，可能包含结尾 0x00）
}

// SerialString 返回去除结尾 0x00 的序列号字符串。
func (v *VersionInfo) SerialString() string {
	n := len(v.SerialNo)
	for n > 0 && v.SerialNo[n-1] == 0 {
		n--
	}
	return string(v.SerialNo[:n])
}

func (v *VersionInfo) String() string {
	return fmt.Sprintf("addr=0x%02X hw=%d fw=%d sn=%q", v.Address, v.HWVersion, v.FWVersion, v.SerialString())
}

// 常见错误。
var (
	// ErrScanning 设备处于扫描模式，不能执行该操作。
	ErrScanning = errors.New("gs5: 设备处于扫描模式，请先停止扫描")
	// ErrNotScanning 设备未处于扫描模式，不能执行该操作。
	ErrNotScanning = errors.New("gs5: 设备未处于扫描模式")
)
