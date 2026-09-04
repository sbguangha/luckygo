import { useEffect, useState } from "react";
import { Button, Card, Form, Input, Modal, Tabs, Typography, message } from "antd";
import { useParams } from "react-router-dom";
import { api, getAccount, getRole, getToken, logout, setSession, type ActivityDetail, type DrawResp, type Winner } from "../api";

function uuid() {
  return crypto.randomUUID();
}

function nextWheelDeg(current: number, index: number, n: number) {
  const slice = 360 / n;
  const target = (360 - (index + 0.5) * slice) % 360;
  const now = ((current % 360) + 360) % 360;
  let delta = (target - now + 360) % 360;
  if (delta < 40) delta += 360;
  return current + 360 * 5 + delta;
}

const WHEEL_COLORS = ["#c5a059", "#f4efe6", "#9b2c2c", "#1f3a5f", "#d4b06a", "#6b4f3a"];

export default function Play() {
  const { publicId } = useParams();
  const [act, setAct] = useState<ActivityDetail | null>(null);
  const [feed, setFeed] = useState<Winner[]>([]);
  const [spinning, setSpinning] = useState(false);
  const [deg, setDeg] = useState(0);
  const [result, setResult] = useState<DrawResp | null>(null);
  const [authed, setAuthed] = useState(!!getToken() && getRole() === "user");
  const [account, setAccount] = useState(getAccount());

  async function load() {
    if (!publicId) return;
    setAct(await api.publicActivity(publicId));
    try {
      setFeed((await api.feed(publicId)).list || []);
    } catch {
      setFeed([]);
    }
  }
  useEffect(() => {
    load().catch((e) => message.error(e.message));
  }, [publicId]);

  useEffect(() => {
    if (!publicId) return;
    const t = window.setInterval(() => {
      api.feed(publicId).then((r) => setFeed(r.list || [])).catch(() => undefined);
    }, 2000);
    return () => window.clearInterval(t);
  }, [publicId]);

  const prizes = act?.prizes || [];

  async function doDraw() {
    if (!publicId) return;
    setSpinning(true);
    try {
      const r = await api.draw({ publicId, idempotencyKey: uuid() });
      const idx = Math.max(
        0,
        prizes.findIndex((p) => p.id === r.prizeId || p.name === r.prizeName),
      );
      setDeg((d) => nextWheelDeg(d, idx, Math.max(prizes.length, 1)));
      setTimeout(() => {
        setResult(r);
        setSpinning(false);
        load();
      }, 3200);
    } catch (e: any) {
      setSpinning(false);
      message.error(e.message);
    }
  }

  function onLogout() {
    logout();
    setAuthed(false);
    setAccount("");
    message.success("已退出，可以换账号登录");
  }

  if (!act) return <div className="play-page play-loading">加载中…</div>;

  return (
    <div className="play-page">
      <div className="play-topbar">
        <span className="play-brand">LuckyGo</span>
        {authed ? (
          <div className="play-user">
            <span>当前账号 {account || "已登录"}</span>
            <Button onClick={onLogout}>退出</Button>
          </div>
        ) : (
          <span className="play-hint">抽奖前请先注册或登录参与者账号</span>
        )}
      </div>

      <div className="play-hero">
        <Typography.Title level={2} className="play-title">{act.title}</Typography.Title>
        <p className="play-meta">
          状态 <b>{act.status}</b>
          <i />
          已参与 <b>{act.participantN}</b>
          <i />
          已中奖 <b>{act.winN}</b>
        </p>
      </div>

      {!authed && (
        <Card className="play-auth-card" bordered={false}>
          <Typography.Paragraph type="secondary" style={{ marginBottom: 8 }}>
            商家后台没有参与者账号。第一次来请先注册（不要用商家账号）。
          </Typography.Paragraph>
          <Tabs
            items={[
              {
                key: "reg",
                label: "注册",
                children: (
                  <Form
                    layout="vertical"
                    onFinish={async (v) => {
                      try {
                        const r = await api.registerUser({
                          publicId,
                          account: v.account,
                          password: v.password,
                          nickname: v.nickname || v.account,
                        });
                        setSession(r.token, r.role, v.account);
                        setAccount(v.account);
                        setAuthed(true);
                        message.success("注册成功，可以抽奖了");
                      } catch (e: any) {
                        message.error(e.message);
                      }
                    }}
                  >
                    <Form.Item name="account" label="新账号" rules={[{ required: true, min: 3, message: "至少 3 个字符" }]}>
                      <Input placeholder="例如 player1" />
                    </Form.Item>
                    <Form.Item name="password" label="密码" rules={[{ required: true, min: 6, message: "至少 6 位" }]}>
                      <Input.Password />
                    </Form.Item>
                    <Form.Item name="nickname" label="昵称（选填）"><Input /></Form.Item>
                    <Button type="primary" htmlType="submit" block>注册并进入</Button>
                  </Form>
                ),
              },
              {
                key: "login",
                label: "登录",
                children: (
                  <Form
                    layout="vertical"
                    initialValues={{ tenantName: act.tenantName }}
                    onFinish={async (v) => {
                      try {
                        const r = await api.login({ tenantName: v.tenantName, account: v.account, password: v.password });
                        if (r.role !== "user") {
                          message.error("这是商家账号，请切到「注册」新建一个参与者账号");
                          return;
                        }
                        setSession(r.token, r.role, v.account);
                        setAccount(v.account);
                        setAuthed(true);
                      } catch (e: any) {
                        message.error(e.message || "登录失败，没有账号请先注册");
                      }
                    }}
                  >
                    <Form.Item name="tenantName" hidden><Input /></Form.Item>
                    <Form.Item name="account" label="账号" rules={[{ required: true }]}><Input /></Form.Item>
                    <Form.Item name="password" label="密码" rules={[{ required: true }]}><Input.Password /></Form.Item>
                    <Button type="primary" htmlType="submit" block>登录</Button>
                  </Form>
                ),
              },
            ]}
          />
        </Card>
      )}

      <div className="wheel-stage">
        <div className="wheel-glow" />
        <div className="pointer">
          <span className="pointer-head" />
          <span className="pointer-stem" />
        </div>
        <div className="wheel-ring">
          <svg className="wheel" viewBox="0 0 340 340" style={{ transform: `rotate(${deg}deg)` }}>
            {prizes.map((p, i) => {
              const name = p.name;
              const n = prizes.length;
              const a0 = (i / n) * Math.PI * 2 - Math.PI / 2;
              const a1 = ((i + 1) / n) * Math.PI * 2 - Math.PI / 2;
              const r = 158;
              const cx = 170;
              const cy = 170;
              const x0 = cx + r * Math.cos(a0);
              const y0 = cy + r * Math.sin(a0);
              const x1 = cx + r * Math.cos(a1);
              const y1 = cy + r * Math.sin(a1);
              const large = n > 1 && (a1 - a0) > Math.PI ? 1 : 0;
              const mid = (a0 + a1) / 2;
              const tx = cx + r * 0.62 * Math.cos(mid);
              const ty = cy + r * 0.62 * Math.sin(mid);
              const fill = WHEEL_COLORS[i % WHEEL_COLORS.length];
              const dark = i % 2 === 0;
              return (
                <g key={name + i}>
                  <path d={`M${cx} ${cy} L${x0} ${y0} A${r} ${r} 0 ${large} 1 ${x1} ${y1} Z`} fill={fill} />
                  <text
                    x={tx}
                    y={ty}
                    fill={dark ? "#fff8e7" : "#5a3d12"}
                    fontSize={n > 6 ? 12 : 15}
                    fontWeight="600"
                    textAnchor="middle"
                    dominantBaseline="middle"
                    transform={`rotate(${(mid * 180) / Math.PI + 90}, ${tx}, ${ty})`}
                  >
                    {name}
                  </text>
                </g>
              );
            })}
            <circle cx="170" cy="170" r="158" fill="none" stroke="rgba(255,255,255,.35)" strokeWidth="2" />
          </svg>
          <div className="wheel-hub">
            <em>GO</em>
          </div>
        </div>
      </div>

      {act.mode === "instant" ? (
        <Button className="draw-btn" type="primary" size="large" disabled={!authed || spinning} onClick={doDraw}>
          {spinning ? "开奖中" : "立即抽奖"}
        </Button>
      ) : (
        <Button
          className="draw-btn"
          type="primary"
          size="large"
          disabled={!authed}
          onClick={() => api.enroll(publicId!).then(() => message.success("报名成功")).catch((e) => message.error(e.message))}
        >
          报名等待开奖
        </Button>
      )}

      <Card className="feed-card" title="实时播报" bordered={false}>
        {feed.length > 0 ? (
          <div className="marquee">
            <div className="marquee-track">
              {[...feed, ...feed].map((it, i) => (
                <span key={i} className={it.kind === "thank_you" ? "feed-thanks" : "feed-win"}>
                  {it.nickname} {it.kind === "thank_you" ? "抽中" : "获得"} {it.prizeName}
                </span>
              ))}
            </div>
          </div>
        ) : (
          <p className="feed-empty">还没有抽奖记录，转一次就会出现在这里</p>
        )}
      </Card>

      <Modal open={!!result} onOk={() => setResult(null)} onCancel={() => setResult(null)} title="抽奖结果" centered>
        {result && (
          <div>
            <p className="result-name">
              {result.kind === "thank_you" ? `未中奖，结果是「${result.prizeName}」` : `恭喜获得 ${result.prizeName}`}
            </p>
            {result.redeemCode && <p>兑换码（只显示一次，请立刻保存）：<b>{result.redeemCode}</b></p>}
            <p>剩余次数 {result.remainDraws}</p>
          </div>
        )}
      </Modal>
    </div>
  );
}
