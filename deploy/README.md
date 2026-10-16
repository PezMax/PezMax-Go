# Edge 出口带宽

`edge` 使用 `Dockerfile.edge` 构建。启动脚本先在容器唯一的外部接口 `eth0` 上安装 TBF 共享带宽池，再挂接有内存上限的 fq_codel 队列，成功后才启动 Nginx。不同 Token、客户端 IP、连接和 Nginx worker 均共享这一个出口预算。缺少 `NET_ADMIN`、内核队列支持或配置错误时，启动失败。

根目录 `.env` 可配置：

```dotenv
KADMIN_EDGE_EGRESS_RATE=8mbit
KADMIN_EDGE_EGRESS_BURST_BYTES=32768
```

`8mbit` 是每秒 8,000,000 bit，即约 1,000,000 bytes，为 10Mbps 的物理上行保留约 20% 余量。速率只接受正整数加 `bit`、`kbit`、`mbit`、`gbit` 单位；突发大小单位为 bytes，范围为 1500–1048576。单桶允许有限突发，因此窗口内的出流上界为「速率 × 时间 + 突发预算」，不是每个毫秒都匀速发送。

启用或更新：

```powershell
docker compose --profile edge up -d --build edge
docker exec pezmax-go-edge tc -s qdisc show dev eth0
docker exec pezmax-go-edge nginx -t
```

默认从 Alpine 官方源安装 `tc`，保持包签名校验。官方源连接超时时，可在根目录 `.env` 设置 `KADMIN_EDGE_APK_MIRROR=https://mirrors.aliyun.com/alpine` 后重新构建；该变量只影响构建时的依赖下载。

应同时看到 root `tbf 1:` 和 child `fq_codel 10:`。`nginx -s reload` 只更新 Nginx 配置，不会重置共享队列；修改总速率后用上面的 Compose 命令重建容器。镜像健康检查会确认共享队列和本地 Nginx 状态端点正常。

现有 `KADMIN_EDGE_DL_RATE_FAST/MID/LOW` 仍是每请求速率，`KADMIN_EDGE_DL_MAXCONN` 是下载并发数；它们用于分配流量，总出口上限以 `KADMIN_EDGE_EGRESS_RATE` 为准。TBF 覆盖 edge 的所有出流，包括普通 API、静态响应、IPv4/IPv6、TCP 重传、向后端发送的代理请求及上传转发。fq_codel 按网络 flow 分配，增加连接仍可能改变各客户端的份额，但不会增加总预算。

预算范围是此 edge 容器的出口。直接访问 `9033` 后端或 `29000` MinIO 的流量不经过该队列；生产部署应将这些服务限制到回环或私网。整台主机或多个 edge 实例合计的带宽限制，需要在共同的宿主机出口设置队列。

从仓库根目录运行 `deploy/tests/test-edge-egress.ps1` 可执行独立回归测试，使用临时容器和不同客户端 IP、Token，不改动实际 edge、数据库或文件。
