/** 微信内置浏览器（尤其 Android WebView）常常没有 crypto.randomUUID。 */
export function newId(): string {
  const c = globalThis.crypto;
  if (c && typeof c.randomUUID === "function") {
    return c.randomUUID();
  }
  if (c && typeof c.getRandomValues === "function") {
    const buf = new Uint8Array(16);
    c.getRandomValues(buf);
    buf[6] = (buf[6] & 0x0f) | 0x40;
    buf[8] = (buf[8] & 0x3f) | 0x80;
    const hex = Array.from(buf, (b) => b.toString(16).padStart(2, "0")).join("");
    return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
  }
  return `u-${Date.now().toString(16)}-${Math.random().toString(16).slice(2, 10)}`;
}

export function joinErrorText(err: unknown): string {
  const raw = err instanceof Error ? err.message : "";
  if (/randomUUID|TypeError|Failed to fetch|NetworkError|Load failed/i.test(raw)) {
    return "报名没成功，请检查网络后稍后再试";
  }
  if (raw) return raw;
  return "报名没成功，请稍后再试";
}
