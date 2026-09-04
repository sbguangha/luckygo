import { useEffect, useRef, useState } from "react";
import { api } from "../api";
import { LotterySphere } from "../live/sphere";
import "../live/sphere.css";

function lanHost(hostname: string) {
  return (
    hostname === "localhost" ||
    hostname === "127.0.0.1" ||
    hostname === "[::1]" ||
    hostname.startsWith("192.168.") ||
    hostname.startsWith("10.") ||
    /^172\.(1[6-9]|2\d|3[0-1])\./.test(hostname)
  );
}

function lanJoinUrl(url: string) {
  try {
    return lanHost(new URL(url).hostname);
  } catch {
    return true;
  }
}

export default function Live() {
  const boxRef = useRef<HTMLDivElement>(null);
  const sphereRef = useRef<LotterySphere | null>(null);
  const phaseRef = useRef<"table" | "sphere" | "spinning" | "result">("table");
  const [count, setCount] = useState(0);
  const [phase, setPhase] = useState<"table" | "sphere" | "spinning" | "result">("table");
  const [toast, setToast] = useState("");
  const [busy, setBusy] = useState(false);
  const [joinUrl, setJoinUrl] = useState("");
  const [localHint, setLocalHint] = useState(false);

  useEffect(() => {
    const url = `${window.location.origin}/join`;
    setJoinUrl(url);
    setLocalHint(lanJoinUrl(url));
  }, []);

  useEffect(() => {
    const sphere = new LotterySphere();
    sphereRef.current = sphere;
    if (boxRef.current) sphere.initScene(boxRef.current);

    const pull = () => {
      api
        .lotteryParticipants()
        .then((d) => {
          const names = d.names || [];
          sphere.updateBall(names);
          setCount(typeof d.count === "number" ? d.count : names.length);
          const next = d.publicJoinUrl || `${window.location.origin}/join`;
          setJoinUrl(next);
          setLocalHint(lanJoinUrl(next));
        })
        .catch(() => undefined);
    };
    pull();
    const timer = window.setInterval(pull, 1000);

    return () => {
      window.clearInterval(timer);
      sphere.destroy();
      sphereRef.current = null;
    };
  }, []);

  useEffect(() => {
    phaseRef.current = phase;
  }, [phase]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.code !== "Space") return;
      e.preventDefault();
      onHostAction();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  });

  function showToast(text: string, ms = 2200) {
    setToast(text);
    window.setTimeout(() => setToast(""), ms);
  }

  async function seedMock() {
    try {
      const r = await api.lotterySeedMock(30);
      showToast(`演示名单已加入 ${r.added} 人，同事扫码仍会进来`);
    } catch (e: unknown) {
      showToast(e instanceof Error ? e.message : "演示名单没有加上，请稍后再试");
    }
  }

  async function onHostAction() {
    const sphere = sphereRef.current;
    if (!sphere || busy) return;
    const now = phaseRef.current;
    if (now === "table") {
      sphere.switchToSphere();
      setPhase("sphere");
      showToast("进入抽奖，空格或按钮开始旋转");
      return;
    }
    if (now === "sphere" || now === "result") {
      sphere.startRotate();
      setPhase("spinning");
      showToast("球体旋转中，再按一次停止出结果");
      return;
    }
    if (now === "spinning") {
      setBusy(true);
      try {
        await sphere.stopRotate();
        const r = await api.lotteryDraw(3);
        const winners = r.winners || [];
        sphere.highlightWinners(winners);
        setPhase("result");
        const left = typeof r.remaining === "number" ? r.remaining : 0;
        if (!winners.length) {
          showToast("没有抽到人", 4000);
        } else if (left === 0) {
          showToast(`中奖：${winners.join("、")}。池里已抽完`, 5000);
        } else {
          showToast(`中奖：${winners.join("、")}。其余 ${left} 人仍在球上`, 5000);
        }
      } catch (e: unknown) {
        sphere.isLotting = false;
        setPhase("sphere");
        showToast(e instanceof Error ? e.message : "抽奖暂时不可用，请稍后再试");
      } finally {
        setBusy(false);
      }
    }
  }

  const hostLabel =
    phase === "table" ? "进入抽奖" : phase === "spinning" ? "停止" : "开始抽奖";

  return (
    <div className="live-page">
      <div className="live-top">LUCKYGO 年会大屏</div>
      <div className="live-count">在线 {count} 人</div>
      <div className="live-hint">微信扫左侧码填写真实姓名上墙。空格：进入抽奖 → 开始 → 停止。</div>
      {toast ? <div className="live-toast">{toast}</div> : null}

      <aside className="live-qr-panel">
        <div className="live-qr-title">微信扫码加入</div>
        {joinUrl ? (
          <img className="live-qr-img" src={`/api/lottery/qr.png?u=${encodeURIComponent(joinUrl)}`} alt="加入抽奖二维码" />
        ) : (
          <div className="live-qr-ph" />
        )}
        <div className="live-qr-url">{joinUrl}</div>
        {localHint ? (
          <p className="live-qr-warn">
            当前还是内网地址，5G/流量扫不开。正在开通公网通道时二维码会自动换成 https 链接。
          </p>
        ) : (
          <p className="live-qr-tip">公网已开通，没连会场 Wi‑Fi 的同事用 5G 也能扫码上墙</p>
        )}
      </aside>

      <div id="container" ref={boxRef} />
      <div className="live-menu">
        <button className="live-btn" type="button" onClick={seedMock}>
          填充演示名单
        </button>
        <button className="live-btn live-btn-draw" type="button" disabled={busy} onClick={onHostAction}>
          {hostLabel}
        </button>
      </div>
    </div>
  );
}
