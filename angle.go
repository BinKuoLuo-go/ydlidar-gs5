package gs5

import (
	"encoding/binary"
	"fmt"
	"math"
)

// GS5 机械结构固定参数
const (
	anglePX    = 1.22  // Angle_Px：激光/相机光心 X 方向机械偏移
	anglePY    = 5.315 // Angle_Py：激光/相机光心 Y 方向机械偏移
	pitchAngle = 16.0  // Angle_PAngle2：GS5 俯仰角（度）
)

const (
	deg2rad = math.Pi / 180.0
	rad2deg = 180.0 / math.Pi
)

// angleTheta 根据 K/B 参数与像素坐标计算 tempTheta（度）。
func angleTheta(k, b, pixelU float64) float64 {
	if b > 1 {
		return k*pixelU - b
	}
	return math.Atan(k*pixelU-b) * rad2deg
}

// transformPoint 将单个原始测距点由像素坐标变换为极坐标（角度 + 距离）
// rawDist 为原始距离（mm，低 11 位）；index 为点序号 0~159
func transformPoint(rawDist float64, index int, p *DeviceParams) (angle, dist float64) {
	pixelU := float64(index)
	var tempTheta, tempDist, tempX, tempY float64

	if index < PointCount/2 {
		// 左相机（L1~L80）
		pixelU = float64(PointCount/2) - pixelU
		tempTheta = angleTheta(p.K0, p.B0, pixelU)
		tempDist = (rawDist - anglePX) / math.Cos((pitchAngle+p.Bias-tempTheta)*deg2rad)
		tempTheta *= deg2rad
		a := (pitchAngle + p.Bias) * deg2rad
		tempX = math.Cos(a)*tempDist*math.Cos(tempTheta) + math.Sin(a)*(tempDist*math.Sin(tempTheta))
		tempY = -math.Sin(a)*tempDist*math.Cos(tempTheta) + math.Cos(a)*(tempDist*math.Sin(tempTheta))
		tempX += anglePX
		tempY -= anglePY
		dist = math.Sqrt(tempX*tempX + tempY*tempY)
		if tempX != 0 {
			angle = math.Atan(tempY/tempX) * rad2deg
		}
	} else {
		// 右相机（R1~R80）
		pixelU = float64(PointCount) - pixelU
		tempTheta = angleTheta(p.K1, p.B1, pixelU)
		tempDist = (rawDist - anglePX) / math.Cos((pitchAngle+p.Bias+tempTheta)*deg2rad)
		tempTheta *= deg2rad
		a := -(pitchAngle + p.Bias) * deg2rad
		tempX = math.Cos(a)*tempDist*math.Cos(tempTheta) + math.Sin(a)*(tempDist*math.Sin(tempTheta))
		tempY = -math.Sin(a)*tempDist*math.Cos(tempTheta) + math.Cos(a)*(tempDist*math.Sin(tempTheta))
		tempX += anglePX
		tempY += anglePY
		dist = math.Sqrt(tempX*tempX + tempY*tempY)
		if tempX != 0 {
			angle = math.Atan(tempY/tempX) * rad2deg
		}
	}

	if angle < 0 {
		angle += 360
	}
	return angle, dist
}

// parseFrame 解析一帧点云数据段，返回 160 个测距点。
func parseFrame(addr byte, data []byte, p *DeviceParams) (*Frame, error) {
	if len(data) != PointCloudDataLen {
		return nil, fmt.Errorf("点云数据段长度错误: 期望 %d, 实际 %d", PointCloudDataLen, len(data))
	}
	frame := &Frame{
		Address: addr,
		Env:     binary.LittleEndian.Uint16(data[0:2]),
		Points:  make([]Point, 0, PointCount),
	}
	for i := 0; i < PointCount; i++ {
		off := EnvDataLen + i*PointDataLen
		raw := binary.LittleEndian.Uint16(data[off : off+2])
		pt := Point{
			Index:     i,
			Intensity: uint8(raw >> 11), // 高 5 位强度
		}
		rawDist := float64(raw & 0x07FF) // 低 11 位距离
		if rawDist > 0 {
			pt.Angle, pt.Distance = transformPoint(rawDist, i, p)
			// 过滤左右相机越过 0° 的越界点
			if i < PointCount/2 {
				if pt.Angle <= 180 {
					pt.Distance = 0
				}
			} else {
				if pt.Angle > 180 {
					pt.Distance = 0
				}
			}
		}
		frame.Points = append(frame.Points, pt)
	}
	return frame, nil
}
