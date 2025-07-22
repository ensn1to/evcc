#!/bin/bash

echo "=== EEBUS服务诊断 ==="

# 检查端口监听状态
echo "1. 检查端口8181监听状态："
lsof -i :8181

echo ""
echo "2. 检查网络连接状态："
netstat -an | grep 8181

echo ""
echo "3. 检查EEBUS进程："
ps aux | grep main | grep 8181

echo ""
echo "4. 测试本地连接到8181端口："
timeout 3 telnet localhost 8181 2>/dev/null || echo "连接失败或超时"

echo ""
echo "5. 测试HTTPS连接到8181端口："
curl -k -v --max-time 5 https://localhost:8181/ 2>&1 | head -20

echo ""
echo "6. 检查证书文件："
if [ -f "eebus.crt" ]; then
    echo "eebus.crt 存在"
    openssl x509 -in eebus.crt -text -noout | grep -E "(Subject:|Issuer:|Not Before:|Not After:|DNS:|IP Address:)" 2>/dev/null || echo "证书解析失败"
else
    echo "eebus.crt 不存在"
fi

echo ""
echo "7. 检查网络接口："
ifconfig | grep -E "(inet |inet6 )" | head -10

echo ""
echo "8. 检查防火墙状态（如果适用）："
sudo pfctl -s rules 2>/dev/null | grep 8181 || echo "未找到8181端口的防火墙规则"

echo ""
echo "=== 诊断完成 ==="
