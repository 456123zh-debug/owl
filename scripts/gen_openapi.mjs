import fs from "node:fs";

const S = {};
const obj = (properties, required = []) => ({ type: "object", properties, ...(required.length ? { required } : {}) });
const str = (description = "") => ({ type: "string", ...(description ? { description } : {}) });
const int = (format = "int64", description = "") => ({ type: "integer", format, ...(description ? { description } : {}) });
const num = (description = "") => ({ type: "number", format: "double", ...(description ? { description } : {}) });
const bool = (description = "") => ({ type: "boolean", ...(description ? { description } : {}) });
const arr = (items) => ({ type: "array", items });
const ref = (name) => ({ $ref: `#/components/schemas/${name}` });
const response = (schema, description = "成功") => ({ description, content: { "application/json": { schema } } });
const binResponse = (contentType, description = "二进制内容") => ({ description, content: { [contentType]: { schema: { type: "string", format: "binary" } } } });
const body = (schema, required = false) => ({ required, content: { "application/json": { schema } } });
const p = (name, in_, schema, required = false, description = "") => ({ name, in: in_, required, schema, ...(description ? { description } : {}) });
const pagers = [
  p("page", "query", int("int32", "页码，从 1 开始")),
  p("size", "query", int("int32", "每页数量")),
  p("sort", "query", str("排序字段，支持前缀 - 表示倒序")),
];
const secured = (operation) => ({ ...operation, security: [{ bearerAuth: [] }] });
const op = (summary, responses, extra = {}) => ({ summary, responses: { "200": responses, "400": { $ref: "#/components/responses/BadRequest" }, ...extra }, ...extra.security ? {} : {} });

S.DefaultOutput = obj({ code: int("int32"), msg: str() });
S.AIWebhookOutput = ref("DefaultOutput");
S.Health = obj({ version: str(), start_at: { type: "string", format: "date-time" }, git_branch: str(), git_hash: str() });
S.KV = obj({ Key: str(), Value: int() });
S.Metrics = obj({ real_time_requests: int(), total_requests: int(), total_responses: int(), request_top10: arr(ref("KV")), status_code_top10: arr(ref("KV")), goroutines: int("int32"), num_gc: int("int64"), sys_alloc: int("int64"), start_at: str() });
S.LoginInput = obj({ data: str("RSA-OAEP 加密后的用户名密码 JSON，Base64") }, ["data"]);
S.LoginOutput = obj({ token: str(), user: str() });
S.PublicKey = obj({ key: str("Base64 编码的 PKIX 公钥") });
S.CredentialsInput = obj({ data: str("RSA-OAEP 加密后的凭据 JSON，Base64") }, ["data"]);
S.Pager = obj({ page: int("int32"), size: int("int32"), sort: str() });
S.DeviceExt = obj({ manufacturer: str(), model: str(), firmware: str(), name: str(), gb_version: str(), zones: arr(ref("Zone")), enabled_ai: bool(), analysis_interval: num(), record_mode: str("always、ai、none") });
S.Zone = obj({ name: str(), coordinates: arr({ type: "number", format: "float" }), color: str(), labels: arr(str()) });
S.StreamConfig = obj({ is_auth_disabled: bool(), session: str(), pushed_at: { type: "string", format: "date-time", nullable: true }, stopped_at: { type: "string", format: "date-time", nullable: true }, media_server_id: str(), push_addr: str(), source_url: str(), transport: int("int32"), timeout_s: int("int32"), enabled_audio: bool(), enabled_remove_none_reader: bool(), enabled_disabled_none_reader: bool(), stream_key: str(), enabled: bool() });
S.Channel = obj({ id: str(), did: str(), device_id: str(), channel_id: str(), name: str(), ptz: int("int32"), is_online: bool(), is_playing: bool(), ext: ref("DeviceExt"), created_at: { type: "string", format: "date-time" }, updated_at: { type: "string", format: "date-time" }, type: str(), app: str(), stream: str(), config: ref("StreamConfig"), has_recording: bool() });
S.Device = obj({ id: str(), type: str(), device_id: str(), name: str(), transport: str(), stream_mode: int("int32"), ip: str(), port: int("int32"), is_online: bool(), registered_at: { type: "string", format: "date-time" }, keepalive_at: { type: "string", format: "date-time" }, keepalives: int("int32"), expires: int("int32"), channels: int("int32"), created_at: { type: "string", format: "date-time" }, updated_at: { type: "string", format: "date-time" }, password: str(), address: str(), ext: ref("DeviceExt"), username: str(), children: arr(ref("Channel")) });
S.AddDevice = obj({ device_id: str(), username: str(), ip: str(), port: int("int32"), name: str(), password: str(), type: str("ONVIF 或 GB28181") });
S.EditDevice = obj({ device_id: str(), name: str(), password: str(), stream_mode: int("int32"), username: str(), ip: str(), port: int("int32") });
S.AddChannel = obj({ type: str("RTMP 或 RTSP"), name: str(), device_id: str(), device_name: str(), app: str(), stream: str(), config: ref("StreamConfig") });
S.EditChannel = obj({ device_id: str(), name: str(), ptztype: int("int32"), is_online: bool(), ext: ref("DeviceExt"), app: str(), stream: str(), config: ref("StreamConfig") });
S.AddZone = obj({ name: str(), coordinates: arr({ type: "number", format: "float" }), color: str(), labels: arr(str()) });
S.PlayOutput = obj({ app: str(), stream: str(), items: arr(obj({ type: str(), url: str(), label: str(), protocol: str(), token: str() })) });
S.SnapshotLink = obj({ link: str() });
S.RecordMode = obj({ id: str(), record_mode: str() });
S.PTZ = obj({ action: str("continuous、stop、absolute、relative、preset"), direction: str(), speed: num(), x: num(), y: num(), zoom: num(), preset_id: str(), preset_op: str("goto、set、remove") });
S.Recording = obj({ id: int(), cid: str(), app: str(), stream: str(), started_at: { type: "string", format: "date-time" }, ended_at: { type: "string", format: "date-time" }, duration: num(), path: str(), size: int(), object_count: int("int32"), delete_flag: bool(), created_at: { type: "string", format: "date-time" }, updated_at: { type: "string", format: "date-time" } });
S.MonthlyStats = obj({ year: int("int32"), month: int("int32"), days: int("int32"), has_video: str() });
S.Event = obj({ id: int(), did: str(), cid: str(), started_at: { type: "string", format: "date-time" }, ended_at: { type: "string", format: "date-time" }, label: str(), score: num(), zones: str(), image_path: str(), model: str(), created_at: { type: "string", format: "date-time" }, updated_at: { type: "string", format: "date-time" } });
S.StreamPush = obj({ id: str(), created_at: { type: "string", format: "date-time" }, updated_at: { type: "string", format: "date-time" }, name: str(), pushed_at: { type: "string", format: "date-time", nullable: true }, stopped_at: { type: "string", format: "date-time", nullable: true }, app: str(), stream: str(), media_server_id: str(), server_id: str(), status: str(), is_auth_disabled: bool(), push_addrs: arr(str()) });
S.StreamProxy = obj({ id: str(), created_at: { type: "string", format: "date-time" }, updated_at: { type: "string", format: "date-time" }, app: str(), stream: str(), media_server_id: str(), source_url: str(), timeout_s: int("int32"), transport: int("int32"), enabled: bool(), enabled_audio: bool(), enabled_remove_none_reader: bool(), enabled_disabled_none_reader: bool(), stream_key: str(), pulling: bool() });
S.MediaServer = obj({ id: str(), ip: str(), hook_ip: str(), sdp_ip: str(), stream_ip: str(), ports: obj({ http: int("int32"), https: int("int32"), rtsp: int("int32"), rtmp: int("int32"), rtp: int("int32") }), auto_config: bool(), secret: str(), hook_alive_interval: int("int32"), rtpenable: bool(), status: bool(), rtpport_range: str(), send_rtpport_range: str(), record_assist_port: int("int32"), last_keepalive_at: { type: "string", format: "date-time" }, is_default: bool(), record_day: int("int32"), record_path: str(), type: str(), transcode_suffix: str() });
S.SIP = obj({ id: str(), ip: str(), port: int("int32"), username: str(), password: str(), domain: str(), serial: str() });
S.ConfigInfo = obj({ sip: ref("SIP") });
S.Metadata = obj({ id: str(), created_at: { type: "string", format: "date-time" }, updated_at: { type: "string", format: "date-time" }, created_by: str(), last_updated_by: str(), ext: str() });
S.AIStats = obj({ active_streams: int("int32"), total_detections: int(), uptime_seconds: int() });
S.AIKeepalive = obj({ timestamp: int(), stats: ref("AIStats"), message: str() });
S.AIStarted = obj({ timestamp: int(), message: str() });
S.AIStopped = obj({ camera_id: str(), timestamp: int(), reason: str(), message: str() });
S.AIBoundingBox = obj({ x_min: int("int32"), y_min: int("int32"), x_max: int("int32"), y_max: int("int32") });
S.AINormBox = obj({ x: num(), y: num(), w: num(), h: num() });
S.AIDetection = obj({ label: str(), confidence: num(), box: ref("AIBoundingBox"), area: int("int32"), norm_box: ref("AINormBox") });
S.AIDetectionInput = obj({ camera_id: str(), timestamp: int(), detections: arr(ref("AIDetection")), snapshot: str(), snapshot_width: int("int32"), snapshot_height: int("int32") });
S.WebhookForward = obj({ did: str(), cid: str(), started_at: int(), ended_at: int(), label: str(), score: num(), zones: str(), image_base64: str(), image_path: str(), model: str() });
S.ZLMStreamChanged = obj({ regist: bool(), aliveSecond: int("int32"), app: str(), bytesSpeed: int(), createStamp: int(), mediaServerId: str(), originSock: obj({ identifier: str(), local_ip: str(), local_port: int("int32"), peer_ip: str(), peer_port: int("int32") }), originType: int("int32"), originTypeStr: str(), originUrl: str(), readerCount: int("int32"), schema: str(), stream: str(), totalReaderCount: int("int32"), tracks: arr(obj({ channels: int("int32"), codec_id: int("int32"), codec_id_name: str(), codec_type: int("int32"), ready: bool(), sample_bit: int("int32"), sample_rate: int("int32"), fps: num(), height: int("int32"), width: int("int32") })), vhost: str() });
S.ZLMPublish = obj({ mediaServerId: str(), app: str(), id: str(), ip: str(), params: str(), port: int("int32"), schema: str(), stream: str(), vhost: str() });
S.ZLMNoneReader = obj({ app: str(), schema: str(), stream: str(), vhost: str(), mediaServerId: str() });
S.ZLMRTPTimeout = obj({ local_port: int("int32"), re_use_port: bool(), ssrc: int("int64"), stream_id: str(), tcp_mode: int("int32"), mediaServerId: str() });
S.ZLMRecordMP4 = obj({ mediaServerId: str(), app: str(), file_name: str(), file_path: str(), file_size: int(), folder: str(), start_time: int(), stream: str(), time_len: num(), url: str(), vhost: str() });

const page = (item) => obj({ items: arr(item), total: int() });
const schemas = Object.fromEntries(Object.entries(S).map(([k, v]) => [k, v]));
schemas.DeviceList = page(ref("Device"));
schemas.ChannelList = page(ref("Channel"));
schemas.EventList = page(ref("Event"));
schemas.RecordingList = page(ref("Recording"));
schemas.StreamPushList = page(ref("StreamPush"));
schemas.StreamProxyList = page(ref("StreamProxy"));
schemas.MediaServerList = page(ref("MediaServer"));
schemas.ZoneList = obj({ items: arr(ref("Zone")) });
schemas.Timeline = obj({ items: arr(obj({ start_ms: int(), end_ms: int(), count: int("int32") })) });
schemas.Stat = obj({ mem: obj({}), cpu: obj({}), disk: arr(obj({ name: str(), used: int(), total: int() })), net: obj({}) });
schemas.Profiles = obj({ count: int("int32"), profiles: arr(obj({})) });
schemas.Version = obj({ version: str(), remark: str() });
schemas.Msg = obj({ msg: str() });
schemas.EventsResponse = ref("DefaultOutput");
schemas.MediaInfo = obj({ app: str(), stream: str(), schema: str(), vhost: str(), isRecordingMP4: bool(), isRecordingHLS: bool(), originType: int("int32"), originTypeStr: str(), originUrl: str(), readerCount: int("int32"), totalReaderCount: int("int32"), aliveSecond: int("int32"), tracks: arr(obj({ codec_id: int("int32"), codec_id_name: str(), codec_type: int("int32"), ready: bool(), fps: num(), width: int("int32"), height: int("int32"), channels: int("int32"), sample_bit: int("int32"), sample_rate: int("int32"), frames: int(), key_frames: int(), loss: num(), duration: int() })) });

const paths = {};
const folderFor = (path) => {
  if (path === "/login" || path === "/login/key" || path === "/users") return "认证";
  if (path.startsWith("/devices") || path === "/gb28181/snapshot") return "设备";
  if (path.startsWith("/channels")) return "通道";
  if (path.startsWith("/recordings") || path.startsWith("/static/recordings")) return "录像";
  if (path.startsWith("/events")) return "事件";
  if (path.startsWith("/media_servers") || path.startsWith("/proxy/sms")) return "流媒体";
  if (path.startsWith("/configs")) return "配置";
  if (path.startsWith("/metadatas")) return "元数据";
  if (path.startsWith("/webhook") || path.startsWith("/ai")) return "Webhook";
  if (path.startsWith("/onvif")) return "ONVIF";
  if (path === "/ws") return "实时通信";
  return "系统";
};
const add = (path, method, summary, request = {}, resp = ref("DefaultOutput"), security = false, tags = []) => {
  const success = resp && resp.description && resp.content ? resp : response(resp);
  const folder = folderFor(path);
  const operation = { summary, tags: tags.length ? tags : [folder], "x-apifox-folder": folder, parameters: request.parameters || [], ...(request.requestBody ? { requestBody: request.requestBody } : {}), responses: { "200": success, "400": { $ref: "#/components/responses/BadRequest" } } };
  if (security) operation.security = [{ bearerAuth: [] }];
  paths[path] ??= {};
  paths[path][method] = operation;
};
const pathId = (name = "id", schema = str()) => ({ parameters: [p(name, "path", schema, true)] });
const json = (schema, required = false) => ({ requestBody: body(schema, required) });
const query = (...params) => ({ parameters: [...params] });
const merge = (...xs) => ({ parameters: xs.flatMap(x => x.parameters || []), ...(xs.some(x => x.requestBody) ? { requestBody: xs.find(x => x.requestBody).requestBody } : {}) });

add("/health", "get", "健康检查", {}, ref("Health"));
add("/app/metrics/api", "get", "获取运行指标", {}, ref("Metrics"));
add("/app/version/check", "get", "检查版本更新", {}, obj({ has_new_version: bool(), current_version: str(), new_version: str(), description: str() }));
add("/app/upgrade", "post", "升级应用（SSE）", {}, { type: "string" }, true);
add("/version", "get", "获取数据库版本", {}, ref("Version"), true);
add("/stats", "get", "获取系统统计", {}, ref("Stat"));
add("/login/key", "get", "获取登录公钥", {}, ref("PublicKey"));
add("/login", "post", "登录", json(ref("LoginInput"), true), ref("LoginOutput"));
add("/users", "put", "修改登录凭据", json(ref("CredentialsInput"), true), ref("Msg"), true);
add("/devices", "get", "设备列表", query(...pagers, p("key", "query", str())), ref("DeviceList"), true);
add("/devices/channels", "get", "设备及通道列表", query(...pagers, p("key", "query", str())), ref("DeviceList"), true);
add("/devices", "post", "添加设备", json(ref("AddDevice"), true), ref("Device"), true);
add("/devices/{id}", "get", "设备详情", pathId(), ref("Device"), true);
add("/devices/{id}", "put", "修改设备", merge(pathId(), json(ref("EditDevice"))), ref("Device"), true);
add("/devices/{id}", "delete", "删除设备", pathId(), ref("Device"), true);
add("/devices/{id}/catalog", "post", "查询设备目录", pathId(), ref("Msg"), true);
add("/gb28181/snapshot", "post", "GB28181 快照回调", {}, ref("Msg"));
add("/onvif/discover", "get", "ONVIF 设备发现（SSE）", {}, { type: "string" });
add("/onvif/profiles", "get", "获取 ONVIF Profiles", {}, ref("Profiles"));
add("/onvif/device_service", "post", "ONVIF Device SOAP 服务", {}, { type: "string" });
add("/onvif/media_service", "post", "ONVIF Media SOAP 服务", {}, { type: "string" });
add("/channels", "get", "通道列表", query(...pagers, p("did", "query", str()), p("device_id", "query", str()), p("key", "query", str()), p("is_online", "query", str()), p("type", "query", str()), p("app", "query", str()), p("stream", "query", str()), p("has_recording", "query", str())), ref("ChannelList"), true);
add("/channels", "post", "添加通道", json(ref("AddChannel"), true), ref("Channel"), true);
add("/channels/{id}", "put", "修改通道", merge(pathId(), json(ref("EditChannel"))), ref("Channel"), true);
add("/channels/{id}", "delete", "删除通道", pathId(), ref("Channel"), true);
add("/channels/{id}/play", "post", "获取播放地址", pathId(), ref("PlayOutput"), true);
add("/channels/{id}/stop", "post", "停止播放", pathId(), ref("Msg"), true);
add("/channels/{id}/snapshot", "post", "刷新通道快照", merge(pathId(), json(obj({ within_seconds: int() }))), ref("SnapshotLink"), true);
add("/channels/{id}/snapshot", "get", "获取通道快照图片", pathId(), binResponse("image/jpeg"), true);
add("/channels/{id}/zones", "post", "添加检测区域", merge(pathId(), json(ref("AddZone"))), ref("ZoneList"), true);
add("/channels/{id}/zones", "get", "获取检测区域", pathId(), ref("ZoneList"), true);
add("/channels/{id}/zones/{name}", "delete", "删除检测区域", merge(pathId(), pathId("name")), ref("ZoneList"), true);
add("/channels/{id}/ai/enable", "post", "启用 AI 检测", pathId(), obj({ enabled: bool(), message: str(), source_width: int("int32"), source_height: int("int32"), source_fps: num() }), true);
add("/channels/{id}/ai/disable", "post", "禁用 AI 检测", pathId(), obj({ enabled: bool(), message: str() }), true);
add("/channels/{id}/record_mode", "post", "设置录像模式", merge(pathId(), json(obj({ mode: str("always、ai、none") }), true)), ref("RecordMode"), true);
add("/channels/{id}/ptz/control", "post", "云台控制", merge(pathId(), json(ref("PTZ"), true)), ref("Msg"), true);
add("/channels/{id}/media_info", "get", "获取流媒体信息", pathId(), ref("MediaInfo"), true);
add("/recordings", "get", "录像列表", query(...pagers, p("cid", "query", str()), p("app", "query", str()), p("stream", "query", str()), p("start_ms", "query", int()), p("end_ms", "query", int())), ref("RecordingList"), true);
add("/recordings/timeline", "get", "录像时间轴", query(p("cid", "query", str()), p("start_ms", "query", int()), p("end_ms", "query", int())), ref("Timeline"), true);
add("/recordings/monthly", "get", "月度录像统计", query(p("cid", "query", str()), p("year", "query", int("int32")), p("month", "query", int("int32"))), ref("MonthlyStats"), true);
add("/recordings/{id}", "get", "录像详情", pathId("id", int()), ref("Recording"), true);
add("/recordings/{id}", "put", "更新录像", merge(pathId("id", int()), json(obj({ object_count: int("int32") }))), ref("Recording"), true);
add("/recordings/{id}", "delete", "删除录像", pathId("id", int()), ref("Recording"), true);
add("/recordings/{id}/download", "get", "下载录像文件", pathId("id", int()), binResponse("video/mp4"), true);
add("/recordings/channels/{cid}/index.m3u8", "get", "获取录像 HLS 播放列表", merge(pathId("cid"), query(p("start_ms", "query", int(), true), p("end_ms", "query", int(), true), p("token", "query", str()))), { type: "string" }, true);
add("/static/recordings/{path}", "get", "访问录像静态文件", { parameters: [p("path", "path", str(), true), p("token", "query", str())] }, binResponse("video/mp4"), true);
add("/media_servers", "get", "流媒体服务器列表", query(...pagers, p("ip", "query", str()), p("type", "query", str()), p("status", "query", bool())), ref("MediaServerList"), true);
add("/media_servers/{id}", "put", "更新流媒体服务器", merge(pathId(), json(obj({ ip: str(), hook_ip: str(), sdp_ip: str(), secret: str(), type: str() }))), ref("MediaServer"), true);
add("/configs/info", "get", "获取系统配置摘要", {}, ref("ConfigInfo"), true);
add("/configs/info/sip", "put", "更新 SIP 配置", json(ref("SIP"), true), ref("Msg"), true);
add("/metadatas/{id}", "get", "获取元数据", pathId(), ref("Metadata"));
add("/metadatas/{id}", "post", "保存元数据", merge(pathId(), json(obj({ id: str(), ext: str() }), true)), ref("Metadata"), true);
add("/events", "get", "事件列表", query(...pagers, p("did", "query", str()), p("cid", "query", str()), p("label", "query", str()), p("start_ms", "query", int()), p("end_ms", "query", int())), ref("EventList"), true);
add("/events/{id}", "get", "事件详情", pathId("id", int()), ref("Event"), true);
add("/events/{id}", "put", "更新事件", merge(pathId("id", int()), json(obj({ ended_at: int() }))), ref("Event"), true);
add("/events/{id}", "delete", "删除事件", pathId("id", int()), ref("Event"), true);
add("/events/image/{path}", "get", "获取事件图片", { parameters: [p("path", "path", str(), true)] }, binResponse("image/jpeg"));
add("/proxy/sms/{path}", "get", "反向代理流媒体资源", { parameters: [p("path", "path", str(), true), p("token", "query", str())] }, { type: "string" });
add("/ws", "get", "WebSocket 实时连接", {}, { type: "string" });
for (const [path, schema, summary] of [
  ["/webhook/on_server_started", "DefaultOutput", "ZLM 服务启动回调"],
  ["/webhook/on_server_keepalive", "DefaultOutput", "ZLM 服务心跳回调"],
  ["/webhook/on_stream_changed", "DefaultOutput", "ZLM 流状态变化回调"],
  ["/webhook/on_publish", "DefaultOutput", "ZLM 推流鉴权回调"],
  ["/webhook/on_play", "DefaultOutput", "ZLM 播放回调"],
  ["/webhook/on_stream_none_reader", "DefaultOutput", "ZLM 无人观看回调"],
  ["/webhook/on_rtp_server_timeout", "DefaultOutput", "ZLM RTP 超时回调"],
  ["/webhook/on_stream_not_found", "DefaultOutput", "ZLM 流不存在回调"],
  ["/webhook/on_record_mp4", "DefaultOutput", "ZLM MP4 录制回调"],
]) add(path, "post", summary, json(ref(path.includes("stream_changed") ? "ZLMStreamChanged" : path.includes("publish") || path.includes("play") ? "ZLMPublish" : path.includes("none_reader") ? "ZLMNoneReader" : path.includes("rtp_server") ? "ZLMRTPTimeout" : path.includes("record_mp4") ? "ZLMRecordMP4" : "DefaultOutput")), ref(schema));
add("/webhook/events", "post", "接收告警事件", json(ref("WebhookForward"), true), ref("DefaultOutput"));
add("/ai/keepalive", "post", "AI 服务心跳", json(ref("AIKeepalive")), ref("AIWebhookOutput"));
add("/ai/started", "post", "AI 服务启动通知", json(ref("AIStarted")), ref("AIWebhookOutput"));
add("/ai/stopped", "post", "AI 任务停止通知", json(ref("AIStopped")), ref("AIWebhookOutput"));
add("/ai/events", "post", "AI 检测事件", json(ref("AIDetectionInput"), true), ref("DefaultOutput"));

const doc = {
  openapi: "3.0.1",
  info: { title: "默认模块", description: "gowvp/owl HTTP API", version: "1.0.0" },
  tags: [
    { name: "系统" }, { name: "认证" }, { name: "设备" }, { name: "通道" }, { name: "录像" }, { name: "事件" },
    { name: "流媒体" }, { name: "配置" }, { name: "元数据" }, { name: "Webhook" }, { name: "ONVIF" }, { name: "实时通信" },
  ],
  paths,
  components: {
    schemas,
    responses: {
      BadRequest: response(obj({ reason: str(), msg: str(), details: obj({}), trace_id: str() }), "请求参数错误"),
    },
    securitySchemes: { bearerAuth: { type: "http", scheme: "bearer", bearerFormat: "JWT" } },
  },
  servers: [{ url: "/" }],
};
fs.writeFileSync("默认模块.openapi.json", JSON.stringify(doc, null, 2) + "\n", "utf8");
console.log(`generated ${Object.keys(paths).length} paths and ${Object.keys(schemas).length} schemas`);
