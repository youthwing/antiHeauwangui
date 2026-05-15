# 河南农业大学晚归系统 API 逆向文档（学生侧）

> 站点: `https://xhbcs.henau.edu.cn`
> API BaseURL: `https://xhbcs.henau.edu.cn/api`
> 认证方式: Bearer Token (JWT)
> JWT 结构: `{"iss": "<userId>", "exp": <timestamp>}`

---

## 通用说明

### 请求头

```
Authorization: Bearer <token>
Content-Type: application/json
```

### 响应格式

```json
{ "code": 200, "message": "操作成功", "data": {} }
```

| code | 含义 |
|------|------|
| 200  | 成功 |
| 400  | 请求错误/系统错误/无权限 |
| 401  | 未认证，需重新登录 |

---

## 1. 用户信息

### 1.1 获取当前用户信息

```
GET /api/auth/user
```

```json
{
  "code": 200,
  "data": {
    "accountStatus": 1,
    "userClass": "数科23-7",
    "userStatus": 0,
    "gender": 0,
    "roles": [{ "roleId": 2, "roleCode": "STUDENT", "roleName": "学生" }],
    "userAvatarUrl": "https://thirdwx.qlogo.cn/...",
    "userSection": "软件学院",
    "userName": "姚依涛",
    "userNumber": "2321211204"
  }
}
```

### 1.2 获取权限列表

```
GET /api/auth/permissions
```

---

## 2. 签到模块

### 2.1 获取可用签到规则

```
GET /api/checkin/available-rules
```

```json
{
  "code": 200,
  "data": [
    {
      "ruleId": 1,
      "ruleName": "晚归签到考勤规则",
      "startTime": "22:00:00",
      "endTime": "22:30:00",
      "description": "每晚22:00-23:59晚归签到时间"
    }
  ]
}
```

### 2.2 获取签到状态

```
GET /api/checkin/status?ruleId={ruleId}
```

```json
{
  "code": 200,
  "data": {
    "canCheckin": false,
    "hasCheckedIn": null,
    "isExempt": null,
    "exemptReason": null,
    "message": "当前不在考勤时间范围内",
    "currentRule": null,
    "minutesRemaining": null,
    "todayRecord": null,
    "isBoarding": false
  }
}
```

### 2.3 执行签到

```
POST /api/checkin
```

**完整请求体:**

```json
{
  "ruleId": 1,
  "latitude": 34.756842,
  "longitude": 113.665412,
  "deviceModel": "iPhone",
  "deviceSystem": "iOS",
  "locationAddress": "河南省郑州市金水区文化路95号河南农业大学",
  "city": "郑州市",
  "road": "文化路",
  "poi": "河南农业大学"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| ruleId | int | 考勤规则ID，从 `/checkin/available-rules` 获取 |
| latitude | double | 纬度 |
| longitude | double | 经度 |
| deviceModel | string | 设备型号，`"iPhone"` 或 `"Android"` |
| deviceSystem | string | 操作系统，`"iOS"` 或 `"Android"` |
| locationAddress | string | 完整地址，天地图逆地理编码结果 |
| city | string | 城市名 |
| road | string | 道路名 |
| poi | string | 兴趣点 |

**安全分析:**

所有定位数据均由前端采集后直接提交，服务端无二次校验：
- 无 GPS 数据签名/加密机制
- 无服务端反向验证坐标真实性
- 无 IP 地址与 GPS 坐标的交叉校验
- 地址字段均为前端自行调天地图 API 拼装，非服务端计算
- `deviceModel`/`deviceSystem` 根据 UA 硬编码，可任意伪造

**curl 签到示例:**

```bash
curl -X POST "https://xhbcs.henau.edu.cn/api/checkin" \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "ruleId": 1,
    "latitude": 34.756842,
    "longitude": 113.665412,
    "deviceModel": "iPhone",
    "deviceSystem": "iOS",
    "locationAddress": "河南省郑州市金水区文化路95号河南农业大学",
    "city": "郑州市",
    "road": "文化路",
    "poi": "河南农业大学"
  }'
```

**限制条件:**

- 签到时间窗口：`22:00 ~ 22:30`（描述写 22:00-23:59），不在时间段内返回 `canCheckin: false`
- 外宿学生（`isBoarding: true`）跳过签到
- 请假中（`isExempt: true`）跳过签到
- 已签到不可重复（`hasCheckedIn: true`）

### 2.4 获取我的签到记录

```
GET /api/checkin/records?ruleId={ruleId}&page={page}&size={size}
```

---

## 接口汇总

| # | 方法 | 路径 | 说明 |
|---|------|------|------|
| 1 | GET | `/api/auth/user` | 用户信息 |
| 2 | GET | `/api/auth/permissions` | 权限列表 |
| 3 | GET | `/api/checkin/available-rules` | 签到规则 |
| 4 | GET | `/api/checkin/status` | 签到状态 |
| 5 | POST | `/api/checkin` | **执行签到** |
| 6 | GET | `/api/checkin/records` | 签到记录 |
