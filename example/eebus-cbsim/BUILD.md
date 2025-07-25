# EEBUS Control Box Simulator - 编译指南

## 概述
EEBUS Control Box Simulator 是一个用于测试和开发EEBUS协议的模拟器。本文档介绍如何编译和使用该模拟器。

## 编译方法

### 1. 基本编译（当前平台）
从项目根目录运行：
```bash
make eebus-cbsim
```
这将在 `example/eebus-cbsim/` 目录下生成 `controlbox` 二进制文件。

### 2. 多平台编译
```bash
make eebus-cbsim-all
```
这将在 `example/eebus-cbsim/dist/` 目录下生成以下平台的二进制文件：
- `controlbox-linux-amd64` - Linux x86_64
- `controlbox-linux-arm64` - Linux ARM64
- `controlbox-darwin-amd64` - macOS Intel
- `controlbox-darwin-arm64` - macOS Apple Silicon
- `controlbox-windows-amd64.exe` - Windows x86_64

### 3. 清理编译文件
```bash
make clean-eebus-cbsim
```
这将删除所有编译生成的二进制文件。

## 使用方法

### 基本用法
```bash
# 使用自动生成的证书
./controlbox <端口号>

# 指定远程SKI（使用自动生成的证书）
./controlbox <端口号> <远程SKI>

# 使用自定义证书
./controlbox <端口号> <远程SKI> <证书文件> <私钥文件>
```

### 示例
```bash
# 在端口4711上启动模拟器，连接到指定的远程SKI
./controlbox 4711 5b1f1545ceed57ea0ceb2baeebc1f01d0033be0b

# 使用自定义证书
./controlbox 4711 5b1f1545ceed57ea0ceb2baeebc1f01d0033be0b cert.pem key.pem
```

## 功能特性

### 日志功能
模拟器提供详细的日志输出，包括：
- 🎯 目标设置和服务启动
- 🔍 mDNS服务发现
- ✅ 连接状态
- 🔐 配对和信任过程
- 🚢 SHIP协议事件

### Web界面
模拟器在端口7071提供Web管理界面：
```
http://localhost:7071
```

### 支持的用例
- LPC (Load Control Obligation)
- LPP (Load Control Production)
- MGCP (Monitoring Grid Connection Point)

## 故障排除

### 常见问题
1. **编译失败**：确保Go版本 >= 1.24.0
2. **依赖问题**：运行 `go mod tidy` 更新依赖
3. **证书问题**：检查证书文件路径和格式
4. **网络问题**：确保防火墙允许指定端口

### 调试模式
查看详细日志输出来诊断连接问题：
```bash
./controlbox 4711 <远程SKI> 2>&1 | tee simulator.log
```

## 开发信息

### 项目结构
- `main.go` - 主程序入口
- `frontend/` - Web界面源码
- `cert.pem`, `key.pem` - 默认证书文件
- `simulator.log` - 运行日志

### 依赖库
- `github.com/enbility/eebus-go` - EEBUS协议实现
- `github.com/enbility/ship-go` - SHIP协议实现
- `github.com/enbility/spine-go` - SPINE协议实现
- `github.com/gorilla/websocket` - WebSocket支持

## 许可证
请参考项目根目录的LICENSE文件。
