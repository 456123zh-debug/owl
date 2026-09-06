owl 视频监控平台 - Linux 免安装版 (x86_64)
==========================================

【快速开始】
1. 解压：tar xzf owl-linux-amd64.tar.gz && cd owl-linux-amd64
2. 安装启动：./install.sh
3. 只启动：./start.sh
4. 浏览器访问 http://<本机IP>:15123，默认账号 admin / admin
5. 停止：./stop.sh
6. 卸载：./uninstall.sh

【组件说明】
- owl             平台主程序（GB28181 信令、Web 管理、录像）
- MediaServer/    ZLMediaKit 流媒体服务
- configs/        配置与数据库（data.db）、录像存储目录
- start.sh 会自动检测本机 IP，写入 config.toml 并触发 owl 自启动 MediaServer

【摄像头接入（GB28181）】
- SIP 端口：15060
- 服务器 ID：34010000002000000001
- 注册密码：空

【端口清单】
15123 管理界面 / 15060 SIP / 8080 ZLM HTTP-FLV / 8443 HTTPS
1935 RTMP / 10554 RTSP / 20000-20100 RTP收流 / 8000 WebRTC
10000 RTP代理 / 9000 SRT / 3000-3001 WebRTC信令
（Linux 普通用户无法绑定 1024 以下端口，RTSP 已改为 10554）

【常见问题】
1. 端口被占用导致 MediaServer 退出：改 MediaServer/config.ini 后重启
2. 防火墙：firewalld/iptables 需放行上述端口
3. 截图功能需 ffmpeg：apt/yum 安装后确保在 PATH 中
4. 数据备份：备份 configs/ 目录即可

【生产建议】
将 start.sh 配置为 systemd 服务实现开机自启；
或多实例部署时注意每个实例使用不同端口段。
