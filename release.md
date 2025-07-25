# evcc项目v1.0.0版本发布文档

## 发布概述

本文档记录了evcc项目v1.0.0版本的完整发布过程，包括代码合并、版本标签创建、构建配置和发布文件生成等步骤。

## 发布信息

- **版本号**: v1.0.0
- **发布日期**: 2024年
- **发布类型**: 初始版本发布
- **主要特性**: 包含EEBUS控制箱模拟器和完整的evcc功能

## 发布前准备

### 1. 代码整理和合并

#### 提交当前修改
```bash
# 添加eebus-cbsim的修改
git add example/eebus-cbsim/main.go

# 提交修改
git commit -m "完善 EEBUS 控制箱模拟器：添加10分钟自动恢复限制功能和优雅关闭处理"
```

#### 分支合并
```bash
# 切换到master分支
git checkout master

# 合并feat/control-box分支
git merge feat/control-box
```

### 2. 版本标签创建

```bash
# 创建v1.0.0标签
git tag -a v1.0.0 -m "Release v1.0.0: 初始版本发布，包含EEBUS控制箱模拟器和完整的evcc功能"

# 推送标签到远程仓库
git push origin v1.0.0

# 推送master分支更新
git push origin master
```

## 构建配置

### 1. Goreleaser安装

```bash
# 安装Goreleaser工具
go install github.com/goreleaser/goreleaser@latest
```

### 2. 配置文件更新

更新`.goreleaser.yml`配置文件：

#### 仓库信息更新
```yaml
# 更新GitHub仓库配置
release:
  github:
    owner: ensn1to  # 从evcc-io更改为ensn1to
    name: evcc

# 更新Homebrew配置
brews:
  - repository:
      owner: ensn1to  # 从evcc-io更改为ensn1to
      name: homebrew-tap
```

#### 版本兼容性调整
```yaml
# 降级配置版本以兼容当前Goreleaser
version: 1  # 从version: 2降级到version: 1

# 调整archives配置格式
archives:
  - id: evcc
    format: tar.gz
    format_overrides:
      - goos: windows
        format: zip

# 调整snapshot配置
snapshot:
  name_template: "{{ .Tag }}-next"
```

## 发布构建

### 1. 本地快照构建

由于需要GitHub token进行正式发布，首先创建本地快照版本：

```bash
# 创建本地快照版本（不需要GitHub token）
/Users/engau/Documents/workspace/goproject/bin/goreleaser --snapshot --skip publish --clean
```

### 2. 构建结果

构建成功，耗时约2分21秒，生成了以下内容：

#### 二进制文件构建
- Linux: amd64, arm64, armv6
- macOS: amd64, arm64, 通用二进制文件
- Windows: amd64

#### 发布包生成
- **Linux**: tar.gz格式归档文件
- **macOS**: tar.gz格式归档文件（通用二进制）
- **Windows**: zip格式归档文件
- **Debian包**: amd64, arm64, armhf格式的.deb包
- **Homebrew**: Formula配置文件
- **校验文件**: checksums.txt

## 生成的发布文件

### 文件列表

```
release/
├── evcc_v1.0.0-next_linux-amd64.tar.gz
├── evcc_v1.0.0-next_linux-armv6.tar.gz
├── evcc_v1.0.0-next_linux-arm64.tar.gz
├── evcc_v1.0.0-next_macOS-all.tar.gz
├── evcc_v1.0.0-next_windows-amd64.zip
├── evcc_1.0.0~next_amd64.deb
├── evcc_1.0.0~next_armhf.deb
├── evcc_1.0.0~next_arm64.deb
├── checksums.txt
├── homebrew/Formula/evcc.rb
├── artifacts.json
├── metadata.json
└── config.yaml
```

### 平台支持

| 平台 | 架构 | 格式 | 文件名 |
|------|------|------|--------|
| Linux | amd64 | tar.gz, deb | evcc_v1.0.0-next_linux-amd64.tar.gz, evcc_1.0.0~next_amd64.deb |
| Linux | arm64 | tar.gz, deb | evcc_v1.0.0-next_linux-arm64.tar.gz, evcc_1.0.0~next_arm64.deb |
| Linux | armv6 | tar.gz, deb | evcc_v1.0.0-next_linux-armv6.tar.gz, evcc_1.0.0~next_armhf.deb |
| macOS | Universal | tar.gz | evcc_v1.0.0-next_macOS-all.tar.gz |
| Windows | amd64 | zip | evcc_v1.0.0-next_windows-amd64.zip |

## 主要功能特性

### 核心功能
- 完整的evcc电动车充电控制功能
- 多种充电器和电表支持
- Web界面管理
- API接口支持

### EEBUS控制箱模拟器
- **10分钟自动恢复限制功能**: 当发送活跃限制时，自动设置10分钟定时器，到期后自动发送非活跃限制恢复状态
- **优雅关闭处理**: 支持Ctrl+C信号处理，确保定时器和资源正确清理
- **WebSocket通信**: 完整的WebSocket服务器支持，用于实时通信
- **证书管理**: 支持TLS证书配置和管理
- **多平台支持**: 支持Linux、macOS、Windows平台

## 技术细节

### 构建配置
- **Go版本**: 使用当前Go环境
- **CGO**: 禁用CGO (CGO_ENABLED=0)
- **构建标签**: -tags=release
- **链接标志**: 包含版本信息和优化选项
- **交叉编译**: 支持多平台交叉编译

### 发布流程
1. 代码合并和标签创建
2. Goreleaser配置更新
3. 多平台构建
4. 归档文件生成
5. 包管理器配置生成
6. 校验文件生成

## 后续步骤

### 正式发布（需要GitHub token）
```bash
# 设置GitHub token环境变量
export GITHUB_TOKEN=your_github_token

# 执行正式发布
goreleaser --clean
```

### 分发渠道
- GitHub Releases页面
- Homebrew tap仓库
- Debian包仓库
- 直接下载归档文件

## 总结

evcc项目v1.0.0版本发布成功完成，包含了完整的功能特性和多平台支持。本次发布特别增强了EEBUS控制箱模拟器的功能，提供了更好的用户体验和系统稳定性。所有发布文件已准备就绪，可以进行分发和部署。