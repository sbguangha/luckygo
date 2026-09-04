import { useEffect, useMemo, useState, type FormEvent } from "react";
import { api } from "../api";
import { joinErrorText, newId } from "../live/id";
import "./join.css";

const UID_KEY = "lg_join_uid";
const JOINED_KEY = "lg_joined_name";

function isWeChat() {
  return /MicroMessenger/i.test(navigator.userAgent);
}

function deviceId() {
  let id = localStorage.getItem(UID_KEY);
  if (!id) {
    id = newId();
    localStorage.setItem(UID_KEY, id);
  }
  return id;
}

export default function Join() {
  const [name, setName] = useState("");
  const [staffNo, setStaffNo] = useState("");
  const [busy, setBusy] = useState(false);
  const [checking, setChecking] = useState(true);
  const [hint, setHint] = useState("");
  const [doneName, setDoneName] = useState("");
  const [wechatOn, setWechatOn] = useState(false);

  const wxFail = useMemo(() => new URLSearchParams(window.location.search).get("wx") === "fail", []);

  useEffect(() => {
    const uid = deviceId();
    Promise.all([
      api.lotteryMe(uid).catch(() => ({ inPool: false, won: false, name: "" })),
      api.lotterySession().catch(() => ({
        wechatEnabled: false,
        nickname: "",
        wechatBound: false,
        count: 0,
      })),
    ])
      .then(([me, s]) => {
        setWechatOn(!!s.wechatEnabled);
        if (s.nickname && !name) setName(s.nickname);
        if (me.inPool && me.name) {
          localStorage.setItem(JOINED_KEY, me.name);
          setDoneName(me.name);
          return;
        }
        localStorage.removeItem(JOINED_KEY);
        if (me.won) {
          setHint("你已经参与过抽奖，把机会留给同事吧");
        }
        if (s.wechatEnabled && isWeChat() && !s.wechatBound && !wxFail && !me.won) {
          window.location.replace("/api/lottery/wechat/login");
        }
      })
      .finally(() => setChecking(false));
  }, []);

  async function submit(e: FormEvent) {
    e.preventDefault();
    const n = name.trim();
    if (n.length < 2 || n.length > 16) {
      setHint("请填写 2 到 16 个字的真实姓名");
      return;
    }
    setBusy(true);
    setHint("");
    try {
      const r = await api.lotteryJoin({
        user_id: deviceId(),
        user_name: n,
        staff_no: staffNo.trim(),
      });
      const shown = r.name || n;
      localStorage.setItem(JOINED_KEY, shown);
      setDoneName(shown);
    } catch (err: unknown) {
      localStorage.removeItem(JOINED_KEY);
      setHint(joinErrorText(err));
    } finally {
      setBusy(false);
    }
  }

  if (checking) {
    return (
      <div className="join-page">
        <div className="join-card">
          <p className="join-brand">LuckyGo 年会抽奖</p>
          <p className="join-lead">正在确认报名状态…</p>
        </div>
      </div>
    );
  }

  if (doneName) {
    return (
      <div className="join-page">
        <div className="join-card">
          <p className="join-brand">LuckyGo 年会抽奖</p>
          <h1>你已在大屏上</h1>
          <p className="join-done-name">{doneName}</p>
          <p className="join-lead">请看前方屏幕，主持人开始后会从名单里抽取。不要关闭微信。</p>
        </div>
      </div>
    );
  }

  return (
    <div className="join-page">
      <form className="join-card" onSubmit={submit}>
        <p className="join-brand">LuckyGo 年会抽奖</p>
        <h1>扫码加入</h1>
        <p className="join-lead">
          {isWeChat() ? "填写真实姓名，提交后名字会出现在现场大屏上。" : "建议用微信扫描大屏二维码打开本页；现在也可以直接填写姓名加入。"}
        </p>
        {wxFail ? <p className="join-hint">微信授权没有完成，请手动填写真实姓名参加。</p> : null}
        {wechatOn && isWeChat() ? <p className="join-soft">已识别微信，请确认或改成工牌上的姓名。</p> : null}
        <label className="join-label" htmlFor="real-name">真实姓名</label>
        <input
          id="real-name"
          className="join-input"
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="与工牌一致"
          autoComplete="name"
          maxLength={16}
        />
        <label className="join-label" htmlFor="staff-no">工号（选填）</label>
        <input
          id="staff-no"
          className="join-input"
          value={staffNo}
          onChange={(e) => setStaffNo(e.target.value)}
          placeholder="便于核对，可不填"
          maxLength={20}
        />
        {hint ? <p className="join-hint">{hint}</p> : null}
        <button className="join-submit" type="submit" disabled={busy}>
          {busy ? "提交中…" : "加入抽奖"}
        </button>
      </form>
    </div>
  );
}
