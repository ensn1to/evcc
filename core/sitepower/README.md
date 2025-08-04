# SitePower 存储模块

这个模块为 evcc 项目提供了 sitePower 数据的定时存储功能，每15分钟自动将站点功率数据保存到 SQLite 数据库中。

## 功能特性

- **定时存储**: 每15分钟自动存储一次 sitePower 数据
- **数据完整性**: 存储创建时间、站点标题、功率(kW)等完整信息
- **可扩展设计**: 模块化设计，易于扩展和维护
- **API接口**: 提供 HTTP API 用于查询和管理存储的数据
- **自动清理**: 支持清理旧数据，避免数据库过大

## 数据库表结构

```sql
CREATE TABLE site_power_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at DATETIME NOT NULL,
    site_title VARCHAR(255) NOT NULL,
    power_kw REAL NOT NULL
);
```

## 核心组件

### 1. SitePowerRecord
存储站点功率数据的记录结构：
- `ID`: 主键
- `CreatedAt`: 创建时间
- `SiteTitle`: 站点标题
- `PowerKW`: 功率，单位为 kW

### 2. DB
数据库操作层，提供：
- `Save()`: 保存新记录
- `GetRecords()`: 获取指定时间范围内的记录
- `GetLatestRecord()`: 获取最新记录
- `DeleteOldRecords()`: 删除旧记录

### 3. Scheduler
定时任务调度器，负责：
- 管理15分钟间隔的定时器
- 缓存当前 sitePower 数据
- 定时触发数据保存
- 数据有效性检查

### 4. API
HTTP API 接口，提供：
- `GET /api/sitepower/records`: 查询历史记录
- `GET /api/sitepower/latest`: 获取最新记录
- `POST /api/sitepower/cleanup`: 清理旧数据

## 集成方式

### 在 Site 结构体中的集成

1. **初始化**: 在 `Boot()` 方法中初始化存储和调度器
2. **数据更新**: 在 `update()` 方法中更新 sitePower 数据
3. **优雅关闭**: 在 shutdown 时停止调度器

### 代码示例

```go
// 在 Site.Boot() 中初始化
if db.Instance != nil {
    sitePowerDB, err := sitepower.NewStore(db.Instance)
    if err != nil {
        site.log.ERROR.Printf("failed to initialize sitePower storage: %v", err)
    } else {
        site.sitePowerScheduler = sitepower.NewScheduler(sitePowerDB, 15*time.Minute)
        site.sitePowerScheduler.Start()
    }
}

// 在 Site.update() 中更新数据
if site.sitePowerScheduler != nil {
    site.sitePowerScheduler.UpdatePower(site.GetTitle(), sitePower/1000.0)
}
```

## API 使用示例

### 查询最近24小时的记录
```bash
curl "http://localhost:7070/api/sitepower/records?site=MyHome&from=$(date -d '24 hours ago' +%s)&to=$(date +%s)"
```

### 获取最新记录
```bash
curl "http://localhost:7070/api/sitepower/latest?site=MyHome"
```

### 清理30天前的数据
```bash
curl -X POST "http://localhost:7070/api/sitepower/cleanup" \
     -H "Content-Type: application/json" \
     -d '{"daysToKeep": 30}'
```

## 配置选项

- **存储间隔**: 默认15分钟，可在初始化时修改
- **数据有效期**: 调度器会检查数据是否过期（超过2个间隔周期）
- **自动清理**: 可通过 API 手动触发，建议定期清理旧数据

## 错误处理

- 数据库连接失败时会记录错误日志但不影响主程序运行
- 数据过期时会跳过保存并记录警告
- API 错误会返回适当的 HTTP 状态码和错误信息

## 性能考虑

- 使用内存缓存减少数据库访问
- 定时批量写入而非实时写入
- 支持数据清理避免数据库无限增长
- 使用 SQLite 的 WAL 模式提高并发性能

## 扩展建议

1. **数据聚合**: 可添加按小时/天聚合的统计数据
2. **数据导出**: 支持 CSV/JSON 格式的数据导出
3. **监控告警**: 基于功率数据的异常检测和告警
4. **图表展示**: 集成到前端界面显示功率趋势图
5. **数据压缩**: 对历史数据进行压缩存储