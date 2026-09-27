package main

import (
	"fmt"
	"log"
	"os"

	gs5 "github.com/BinKuoLuo-go/ydlidar-gs5"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "用法: <串口名>  例如:  COM3")
		os.Exit(1)
	}
	port := os.Args[1]

	// 打开并初始化设备（默认波特率 921600，自动获取级联地址与参数）
	dev, err := gs5.Open(port)
	if err != nil {
		log.Fatalf("打开设备失败: %v", err)
	}
	defer dev.Close()

	fmt.Printf("级联设备地址: %X\n", dev.Addresses())

	// 打印版本信息
	versions, err := dev.GetVersion()
	if err != nil {
		log.Fatalf("获取版本失败: %v", err)
	}
	for _, v := range versions {
		fmt.Printf("设备 0x%02X: %s\n", v.Address, v)
	}

	// 启动扫描
	if err := dev.StartScan(); err != nil {
		log.Fatalf("启动扫描失败: %v", err)
	}

	// 读取并打印10帧点云
	for n := 0; n < 10; n++ {
		frame, err := dev.GrabFrame()
		if err != nil {
			log.Fatalf("读取点云失败: %v", err)
		}
		fmt.Printf("帧[设备0x%02X] 环境光=%d 点数=%d\n", frame.Address, frame.Env, len(frame.Points))
		for _, p := range frame.Points {
			if p.Distance > 0 {
				fmt.Printf("  S%d 角度=%.1f° 距离=%.1fmm 强度=%d\n",
					p.Index+1, p.Angle, p.Distance, p.Intensity)

			}
		}
	}

	// 停止扫描
	if err := dev.StopScan(); err != nil {
		log.Fatalf("停止扫描失败: %v", err)
	}
	fmt.Println("已停止扫描")
}
