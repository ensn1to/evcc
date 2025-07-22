#!/bin/bash

echo "=== EEBUS Host配置诊断 ==="

echo "1. 系统hostname信息："
echo "   hostname: $(hostname)"
echo "   scutil --get LocalHostName: $(scutil --get LocalHostName)"
echo "   scutil --get ComputerName: $(scutil --get ComputerName)"

echo ""
echo "2. 网络接口信息："
ifconfig | grep -E "(inet |inet6 )" | head -5

echo ""
echo "3. mDNS相关信息："
echo "   检查.local域名解析："
nslookup MacBook-Pro-2.local 2>/dev/null || echo "   无法解析MacBook-Pro-2.local"

echo ""
echo "4. 启动EEBUS服务并监控mDNS广播："
echo "   启动服务..."
./controlbox 8184 test_ski cert.pem key.pem &
CONTROLBOX_PID=$!

echo "   等待服务启动..."
sleep 3

echo ""
echo "5. 检查mDNS服务广播："
# 使用dns-sd命令查看mDNS广播（如果可用）
if command -v dns-sd >/dev/null 2>&1; then
    echo "   查看_ship._tcp服务广播（5秒）："
    timeout 5 dns-sd -B _ship._tcp local. 2>/dev/null || echo "   无法查看mDNS广播"
else
    echo "   dns-sd命令不可用"
fi

echo ""
echo "6. 检查服务监听端口："
lsof -i :8184 2>/dev/null || echo "   端口8184未被监听"

echo ""
echo "7. 停止测试服务："
kill $CONTROLBOX_PID 2>/dev/null
wait $CONTROLBOX_PID 2>/dev/null

echo ""
echo "=== 诊断完成 ==="
echo ""
echo "分析："
echo "- 如果host显示为'MacBook-Pro-2.local.local'，可能的原因："
echo "  1. EEBUS库在处理hostname时添加了额外的.local后缀"
echo "  2. mDNS服务发现过程中的域名解析问题"
echo "  3. 系统网络配置问题"
echo ""
echo "解决方案："
echo "- 检查EEBUS库的配置选项"
echo "- 修改系统hostname配置"
echo "- 在代码中手动设置host地址"
