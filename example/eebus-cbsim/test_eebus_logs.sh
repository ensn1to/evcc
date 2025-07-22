#!/bin/bash

echo "=== 测试EEBUS连接日志功能 ==="

# 停止现有的进程
echo "停止现有的EEBUS进程..."
pkill -f "main.*8181" 2>/dev/null || true
sleep 2

# 启动新的EEBUS服务，使用不同的端口避免冲突
echo "启动EEBUS服务（端口8182）..."
./controlbox 8182 dummy_remote_ski cert.pem key.pem &
CONTROLBOX_PID=$!

echo "EEBUS服务已启动，PID: $CONTROLBOX_PID"
echo "等待5秒让服务完全启动..."
sleep 5

# 检查进程是否还在运行
if kill -0 $CONTROLBOX_PID 2>/dev/null; then
    echo "✅ EEBUS服务正在运行"
    
    # 显示最近的日志
    echo ""
    echo "=== 查看启动日志 ==="
    echo "应该看到以下类型的日志："
    echo "- 🎯 [EEBUS] Registering target remote SKI"
    echo "- 🚀 [EEBUS] Starting EEBUS service"
    echo "- ✅ [EEBUS] EEBUS service started successfully"
    echo ""
    
    # 等待一段时间观察mDNS发现日志
    echo "等待10秒观察mDNS发现和连接日志..."
    sleep 10
    
    # 优雅关闭
    echo ""
    echo "发送SIGINT信号进行优雅关闭..."
    kill -INT $CONTROLBOX_PID
    
    # 等待进程结束
    sleep 3
    
    if kill -0 $CONTROLBOX_PID 2>/dev/null; then
        echo "进程未正常结束，强制终止..."
        kill -KILL $CONTROLBOX_PID
    else
        echo "✅ 进程已优雅关闭"
    fi
else
    echo "❌ EEBUS服务启动失败"
fi

echo ""
echo "=== 测试完成 ==="
echo ""
echo "新增的日志功能："
echo "🔍 [mDNS] - mDNS服务发现日志"
echo "✅ [EEBUS] - EEBUS连接状态日志"
echo "🔐 [PAIRING] - 配对和信任状态日志"
echo "🚢 [SHIP] - SHIP协议相关日志"
echo ""
echo "这些日志将帮助诊断其他端无法连接的问题。"
