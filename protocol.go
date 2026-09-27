package gs5

import (
	"encoding/binary"
	"fmt"
	"io"
)

// checksum 计算校验码：对包头 4 字节与校验码本身之外的所有字节单字节累加
// （即地址 + 命令 + 数据长度 + 数据段），见开发手册 4 校验码解析。
func checksum(b []byte) byte {
	var sum byte
	for _, v := range b {
		sum += v
	}
	return sum
}

// buildPacket 组装完整报文。
func buildPacket(addr, cmd byte, data []byte) []byte {
	pkt := make([]byte, 0, PacketHeaderLen+len(data)+1)
	pkt = append(pkt, headerByte, headerByte, headerByte, headerByte)
	pkt = append(pkt, addr, cmd)
	pkt = append(pkt, byte(len(data)&0xFF), byte(len(data)>>8))
	pkt = append(pkt, data...)
	pkt = append(pkt, checksum(pkt[HeaderLen:]))
	return pkt
}

// readPacket 从串口同步包头并读取一个完整报文，校验校验码。
// 返回设备地址、命令码、数据段。
func readPacket(r io.Reader) (addr, cmd byte, data []byte, err error) {
	var one [1]byte
	// 1) 同步包头 0xA5A5A5A5
	matched := 0
	for matched < HeaderLen {
		if _, err = io.ReadFull(r, one[:]); err != nil {
			return 0, 0, nil, fmt.Errorf("同步包头失败: %w", err)
		}
		if one[0] == headerByte {
			matched++
		} else {
			matched = 0
		}
	}
	// 2) 跳过可能多出的包头字节，读取设备地址
	for {
		if _, err = io.ReadFull(r, one[:]); err != nil {
			return 0, 0, nil, fmt.Errorf("读取设备地址失败: %w", err)
		}
		if one[0] != headerByte {
			break
		}
	}
	addr = one[0]
	// 3) 命令码 + 数据长度
	hdr := make([]byte, 3)
	if _, err = io.ReadFull(r, hdr); err != nil {
		return 0, 0, nil, fmt.Errorf("读取报文头失败: %w", err)
	}
	cmd = hdr[0]
	dataLen := binary.LittleEndian.Uint16(hdr[1:3])
	// 4) 数据段 + 校验码
	body := make([]byte, dataLen+1)
	if _, err = io.ReadFull(r, body); err != nil {
		return 0, 0, nil, fmt.Errorf("读取数据段失败: %w", err)
	}
	data = body[:dataLen]
	got := body[dataLen]
	// 5) 校验码校验
	check := make([]byte, 0, 1+1+2+len(data))
	check = append(check, addr, cmd)
	check = append(check, hdr[1:3]...)
	check = append(check, data...)
	if sum := checksum(check); sum != got {
		return 0, 0, nil, fmt.Errorf("校验码不匹配: 计算 0x%02X, 实际 0x%02X", sum, got)
	}
	return addr, cmd, data, nil
}

// parseParams 解析设备参数数据段（9 字节）。
func parseParams(data []byte) (*DeviceParams, error) {
	if len(data) < 9 {
		return nil, fmt.Errorf("参数数据段长度不足: 期望 9, 实际 %d", len(data))
	}
	return &DeviceParams{
		K0:   float64(binary.LittleEndian.Uint16(data[0:2])) / 10000.0,
		B0:   float64(binary.LittleEndian.Uint16(data[2:4])) / 10000.0,
		K1:   float64(binary.LittleEndian.Uint16(data[4:6])) / 10000.0,
		B1:   float64(binary.LittleEndian.Uint16(data[6:8])) / 10000.0,
		Bias: float64(int8(data[8])) / 10.0,
	}, nil
}

// parseVersion 解析版本信息数据段（19 字节 = 硬件版本 1 + 固件版本 2 + 序列号 16）。
func parseVersion(addr byte, data []byte) (*VersionInfo, error) {
	if len(data) < 19 {
		return nil, fmt.Errorf("版本数据段长度不足: 期望 19, 实际 %d", len(data))
	}
	sn := make([]byte, 16)
	copy(sn, data[3:19])
	return &VersionInfo{
		Address:   addr,
		HWVersion: data[0],
		// 注意：固件版本号为大端序（高字节在前），与官方 SDK 的字节交换处理一致。
		FWVersion: binary.BigEndian.Uint16(data[1:3]),
		SerialNo:  sn,
	}, nil
}
