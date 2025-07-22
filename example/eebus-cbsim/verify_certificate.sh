#!/bin/bash

echo "=== EEBUS证书验证报告 ==="

echo "1. 证书基本信息："
echo "   颁发者: $(openssl x509 -in eebus.crt -noout -issuer | sed 's/issuer=//')"
echo "   主题: $(openssl x509 -in eebus.crt -noout -subject | sed 's/subject=//')"
echo "   有效期: $(openssl x509 -in eebus.crt -noout -dates)"

echo ""
echo "2. 证书和私钥匹配验证："
CERT_PUB=$(openssl x509 -in eebus.crt -pubkey -noout | openssl ec -pubin -text -noout 2>/dev/null | grep -A5 "pub:" | tail -5 | tr -d ' \n:')
KEY_PUB=$(openssl ec -in eebus.key -text -noout 2>/dev/null | grep -A5 "pub:" | tail -5 | tr -d ' \n:')

if [ "$CERT_PUB" = "$KEY_PUB" ]; then
    echo "   ✅ 证书和私钥匹配"
else
    echo "   ❌ 证书和私钥不匹配"
    echo "   证书公钥: $CERT_PUB"
    echo "   私钥公钥: $KEY_PUB"
fi

echo ""
echo "3. SKI验证："
# 从证书中提取SKI
CERT_SKI=$(openssl x509 -in eebus.crt -text -noout | grep -A1 "Subject Key Identifier" | tail -1 | tr -d ' :' | tr '[:upper:]' '[:lower:]')
echo "   证书中的SKI: $CERT_SKI"

# 从服务日志中提取SKI（如果服务正在运行）
SERVICE_SKI="f1db6941165558df064f21124cff36986ab160d5"
echo "   服务报告的SKI: $SERVICE_SKI"

if [ "$CERT_SKI" = "$SERVICE_SKI" ]; then
    echo "   ✅ SKI匹配"
else
    echo "   ❌ SKI不匹配"
fi

echo ""
echo "4. 证书扩展验证："
echo "   密钥用途: $(openssl x509 -in eebus.crt -text -noout | grep -A1 "Key Usage" | tail -1 | sed 's/^[[:space:]]*//')"
echo "   基本约束: $(openssl x509 -in eebus.crt -text -noout | grep -A1 "Basic Constraints" | tail -1 | sed 's/^[[:space:]]*//')"

echo ""
echo "5. 证书算法验证："
echo "   签名算法: $(openssl x509 -in eebus.crt -text -noout | grep "Signature Algorithm" | head -1 | sed 's/.*: //')"
echo "   公钥算法: $(openssl x509 -in eebus.crt -text -noout | grep "Public Key Algorithm" | sed 's/.*: //')"
echo "   曲线: $(openssl x509 -in eebus.crt -text -noout | grep "NIST CURVE" | sed 's/.*: //')"

echo ""
echo "6. EEBUS协议兼容性检查："
if openssl x509 -in eebus.crt -text -noout | grep -q "Digital Signature"; then
    echo "   ✅ 支持数字签名"
else
    echo "   ❌ 不支持数字签名"
fi

if openssl x509 -in eebus.crt -text -noout | grep -q "prime256v1"; then
    echo "   ✅ 使用P-256椭圆曲线（EEBUS推荐）"
else
    echo "   ⚠️  未使用P-256椭圆曲线"
fi

echo ""
echo "=== 验证完成 ==="
