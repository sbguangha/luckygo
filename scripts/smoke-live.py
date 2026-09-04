# -*- coding: utf-8 -*-
"""LuckyGo live 模式端到端冒烟（Windows 控制台友好：全程 UTF-8 字节处理）。"""
import json, time, urllib.request, urllib.error, uuid, sys, io, zlib, struct

sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding="utf-8")
BASE = "http://127.0.0.1:8888/api/v1"

def call(method, path, body=None, token=None, raw=None, ctype=None):
    data = None
    headers = {}
    if body is not None:
        data = json.dumps(body).encode("utf-8")
        headers["Content-Type"] = "application/json"
    if raw is not None:
        data = raw
        headers["Content-Type"] = ctype
    if token:
        headers["Authorization"] = "Bearer " + token
    req = urllib.request.Request(BASE + path, data=data, method=method, headers=headers)
    try:
        with urllib.request.urlopen(req) as r:
            return json.loads(r.read().decode("utf-8"))
    except urllib.error.HTTPError as e:
        return json.loads(e.read().decode("utf-8"))

def ok(resp, what):
    if resp.get("code") != 0:
        print(f"!! {what} FAILED: {resp}")
        sys.exit(1)
    print(f"-- {what}: ok")
    return resp.get("data")

# 1. 注册新租户（保证 UTF-8 干净数据）
suffix = uuid.uuid4().hex[:6]
tenant = "冒烟公司" + suffix
tok = ok(call("POST", "/auth/register-tenant",
              {"tenantName": tenant, "account": "admin" + suffix, "password": "123456", "nickname": "老板"}),
         "注册租户")["token"]

# 2. 创建 live 活动
now = int(time.time())
act = ok(call("POST", "/admin/activities", {
    "title": "年会现场抽奖", "mode": "live", "rosterSource": "both",
    "startAt": now - 60, "endAt": now + 86400,
    "prizes": [
        {"name": "三等奖", "kind": "virtual", "stock": 3, "perRound": 2},
        {"name": "一等奖", "kind": "physical", "stock": 1, "perRound": 1},
    ]}, tok), "创建活动")
aid = act["id"]
print("   activity id =", aid, "publicId =", act["publicId"])

# 3. 导入名单
rows = [{"uid": f"E{i:03d}", "name": f"员工{i:03d}", "department": "技术部" if i % 2 else "市场部"} for i in range(1, 11)]
imp = ok(call("POST", f"/admin/activities/{aid}/participants/import", {"rows": rows}, tok), "导入10人")
assert imp["total"] == 10 and imp["failed"] == 0, imp

ps = ok(call("GET", f"/admin/activities/{aid}/participants", token=tok), "名单列表")
assert len(ps["list"]) == 10, len(ps["list"])

# 4. 发布
ok(call("POST", f"/admin/activities/{aid}/publish", {}, tok), "发布")

det = ok(call("GET", f"/admin/activities/{aid}", token=tok), "活动详情")
assert det["status"] == "running" and det["participantN"] == 10, det
p3 = [p for p in det["prizes"] if p["stock"] == 3][0]
p1 = [p for p in det["prizes"] if p["stock"] == 1][0]
print("   三等奖 id=", p3["id"], "remain", p3["remain"], "| 一等奖 id=", p1["id"])

# 5. 抽三等奖（一次2人）+ 幂等重放
r1 = ok(call("POST", f"/admin/activities/{aid}/live-draw", {"prizeId": p3["id"], "idempotencyKey": "batch-0001"}, tok), "三等奖抽2人")
assert len(r1["winners"]) == 2 and r1["remain"] == 1, r1
w1 = sorted(w["participantId"] for w in r1["winners"])
r1b = ok(call("POST", f"/admin/activities/{aid}/live-draw", {"prizeId": p3["id"], "idempotencyKey": "batch-0001"}, tok), "同幂等键重放")
w1b = sorted(w["participantId"] for w in r1b["winners"])
assert w1 == w1b, (w1, w1b)
print("   重放名单一致:", [w["name"] for w in r1["winners"]])

# 6. 取消 -> 重放报已取消 -> 重抽
ok(call("POST", f"/admin/activities/{aid}/live-draw/undo", {"drawId": "batch-0001"}, tok), "取消批次")
r1c = call("POST", f"/admin/activities/{aid}/live-draw", {"prizeId": p3["id"], "idempotencyKey": "batch-0001"}, tok)
assert r1c.get("code") != 0, r1c
print("-- 已取消批次重放被拒: ok ->", r1c.get("msg"))

r2 = ok(call("POST", f"/admin/activities/{aid}/live-draw", {"prizeId": p3["id"], "idempotencyKey": "batch-0002"}, tok), "取消后重抽2人")
r3 = ok(call("POST", f"/admin/activities/{aid}/live-draw", {"prizeId": p3["id"], "idempotencyKey": "batch-0003"}, tok), "三等奖最后1人")
assert r3["remain"] == 0, r3
r4 = call("POST", f"/admin/activities/{aid}/live-draw", {"prizeId": p3["id"], "idempotencyKey": "batch-0004"}, tok)
assert r4.get("code") != 0, r4
print("-- 抽完再抽被拒: ok ->", r4.get("msg"))

# 7. 一等奖：已中奖者不应再中
won3 = {w["participantId"] for w in r2["winners"]} | {w["participantId"] for w in r3["winners"]}
r5 = ok(call("POST", f"/admin/activities/{aid}/live-draw", {"prizeId": p1["id"], "idempotencyKey": "batch-0005"}, tok), "一等奖1人")
assert r5["winners"][0]["participantId"] not in won3, "重复中奖！"
print("   一等奖:", r5["winners"][0]["name"])

# 8. 名单页 isWin 标记 + 管理端中奖名单
ps2 = ok(call("GET", f"/admin/activities/{aid}/participants", token=tok), "名单 isWin")
assert sum(1 for p in ps2["list"] if p["isWin"]) == 4, ps2
aw = ok(call("GET", f"/admin/activities/{aid}/winners", token=tok), "管理端中奖名单")
assert len(aw["list"]) == 4, aw["list"]
tok0 = [w for w in aw["list"] if w["source"] == "import"][0]["prizeToken"]

# 9. 线下核销（导入名单）
ok(call("POST", f"/admin/activities/{aid}/offline-redeem", {"prizeToken": tok0}, tok), "线下核销")
aw2 = ok(call("GET", f"/admin/activities/{aid}/winners", token=tok), "核销后名单")
used = [w for w in aw2["list"] if w["prizeToken"] == tok0][0]
assert used["redeemStatus"] == "used", used

# 10. 装修配置 + 公开接口
ok(call("PUT", f"/admin/activities/{aid}/ui-config", {"config": {"topTitle": "2026 年会盛典", "cardColor": "#ff79c6", "rowCount": 17, "showPrizeList": True}}, tok), "保存装修")
pub = ok(call("GET", f"/play/{act['publicId']}"), "C端活动详情")
assert pub["uiConfig"]["topTitle"] == "2026 年会盛典", pub["uiConfig"]
assert pub["rosterSource"] == "both" and pub["winN"] == 4, pub
feed = ok(call("GET", f"/play/{act['publicId']}/feed"), "中奖播报")
assert len(feed["list"]) == 4, feed
assert all(w["nickname"].endswith("*") for w in feed["list"]), "播报应脱敏"
print("   播报样例:", feed["list"][0])

# 11. C 端用户注册报名上球 -> 大屏新奖项池包含之
pubid = act["publicId"]
acc = "user" + uuid.uuid4().hex[:6]
utok = ok(call("POST", "/auth/register-user", {"publicId": pubid, "account": acc, "password": "123456", "nickname": "现场报名王"}), "C端注册")["token"]
ok(call("POST", "/enroll", {"publicId": pubid}, utok), "C端报名")
ps3 = ok(call("GET", f"/admin/activities/{aid}/participants", token=tok), "报名后名单")
assert len(ps3["list"]) == 11 and any(p["source"] == "register" for p in ps3["list"]), ps3

# 12. 图片上传与静态访问（手工构造 1x1 PNG）
def png1x1():
    def chunk(t, d):
        return struct.pack(">I", len(d)) + t + d + struct.pack(">I", zlib.crc32(t + d))
    return (b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", 1, 1, 8, 2, 0, 0, 0))
            + chunk(b"IDAT", zlib.compress(b"\x00\xff\x00\x00")) + chunk(b"IEND", b""))

boundary = "----smoketest"
body = (f"--{boundary}\r\nContent-Disposition: form-data; name=\"file\"; filename=\"t.png\"\r\n"
        f"Content-Type: image/png\r\n\r\n").encode() + png1x1() + f"\r\n--{boundary}--\r\n".encode()
up = ok(call("POST", "/admin/upload", raw=body, ctype=f"multipart/form-data; boundary={boundary}", token=tok), "上传图片")
url = "http://127.0.0.1:8888" + up["url"]
with urllib.request.urlopen(url) as r:
    assert r.status == 200 and r.headers["Content-Type"].startswith("image/png"), r.headers
print("-- 静态访问上传图: ok ->", up["url"])

print("\n==== ALL SMOKE TESTS PASSED ====")
