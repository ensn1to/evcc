#!/bin/bash

# EEBUS连接调试脚本
echo "=== EEBUS连接调试工具 ==="
echo

# 检查目标主机
TARGET_HOST="raspberrypi.local"
TARGET_IP="172.17.0.1"
TARGET_PORT="4711"
TARGET_SKI="6b395afa4bee11215df0dfa96d5dc759f9b80ee5"

echo "🔍 1. DNS解析检查"
echo "检查 $TARGET_HOST 的DNS解析..."
if nslookup $TARGET_HOST > /dev/null 2>&1; then
    echo "✅ DNS解析成功"
    nslookup $TARGET_HOST
else
    echo "❌ DNS解析失败"
    echo "尝试使用IP地址: $TARGET_IP"
fi
echo

echo "🌐 2. 网络连通性检查"
echo "Ping测试 $TARGET_HOST..."
if ping -c 3 $TARGET_HOST > /dev/null 2>&1; then
    echo "✅ Ping成功"
else
    echo "❌ Ping失败，尝试IP地址"
    if ping -c 3 $TARGET_IP > /dev/null 2>&1; then
        echo "✅ IP Ping成功"
    else
        echo "❌ IP Ping也失败"
    fi
fi
echo

echo "🔌 3. 端口连接检查"
echo "检查端口 $TARGET_PORT 是否开放..."
if nc -z $TARGET_IP $TARGET_PORT 2>/dev/null; then
    echo "✅ 端口 $TARGET_PORT 开放"
else
    echo "❌ 端口 $TARGET_PORT 无法连接"
fi
echo

echo "🔐 4. TLS连接检查"
echo "检查TLS握手..."
timeout 5 openssl s_client -connect $TARGET_IP:$TARGET_PORT -verify_return_error 2>/dev/null | head -20
echo

echo "📡 5. mDNS服务发现"
echo "搜索EEBUS服务..."
if command -v avahi-browse >/dev/null 2>&1; then
    timeout 10 avahi-browse -t _ship._tcp
elif command -v dns-sd >/dev/null 2>&1; then
    timeout 10 dns-sd -B _ship._tcp
else
    echo "⚠️ 未找到mDNS工具 (avahi-browse 或 dns-sd)"
fi
echo

echo "🚢 6. SHIP服务检查"
echo "尝试HTTP连接到SHIP端点..."
curl -v --connect-timeout 5 http://$TARGET_IP:$TARGET_PORT/ship/ 2>&1 | head -10
echo

echo "=== 调试完成 ==="
echo "如果发现问题，请检查："
echo "1. 目标设备是否运行在正确的IP和端口"
echo "2. 防火墙设置是否允许连接"
echo "3. 证书配置是否正确"
echo "4. SKI是否匹配"
