export type Role = "admin" | "user";

const TOKEN = "lg_token";
const ROLE = "lg_role";
const ACCOUNT = "lg_account";

export function getToken() {
  return localStorage.getItem(TOKEN) || "";
}
export function setSession(token: string, role: Role, account?: string) {
  localStorage.setItem(TOKEN, token);
  localStorage.setItem(ROLE, role);
  if (account) localStorage.setItem(ACCOUNT, account);
}
export function getRole(): Role {
  return (localStorage.getItem(ROLE) as Role) || "user";
}
export function getAccount() {
  return localStorage.getItem(ACCOUNT) || "";
}
export function logout() {
  localStorage.removeItem(TOKEN);
  localStorage.removeItem(ROLE);
  localStorage.removeItem(ACCOUNT);
}

async function req<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  if (!headers.has("Content-Type") && !(init.body instanceof FormData)) {
    headers.set("Content-Type", "application/json");
  }
  const t = getToken();
  if (t) headers.set("Authorization", `Bearer ${t}`);
  const res = await fetch(path, { credentials: "include", ...init, headers });
  const body = await res.json().catch(() => ({}));
  if (!res.ok || (body.code && body.code !== 0)) {
    throw new Error(body.msg || "请求失败");
  }
  return (body.data ?? body) as T;
}

const post = <T>(path: string, d: object = {}) =>
  req<T>(path, { method: "POST", body: JSON.stringify(d) });
const put = <T>(path: string, d: object = {}) =>
  req<T>(path, { method: "PUT", body: JSON.stringify(d) });
const del = <T>(path: string) => req<T>(path, { method: "DELETE" });

export const api = {
  // ---- 认证（公开） ----
  registerTenant: (d: { tenantName: string; account: string; password: string; nickname?: string }) =>
    post<{ token: string; role: Role; nickname: string; tenant: string }>("/api/v1/auth/register-tenant", d),
  login: (d: { tenantName: string; account: string; password: string }) =>
    post<{ token: string; role: Role; nickname: string; tenant: string }>("/api/v1/auth/login", d),
  registerUser: (d: { publicId: string; account: string; password: string; nickname?: string }) =>
    post<{ token: string; role: Role }>("/api/v1/auth/register-user", d),

  // ---- 管理端（admin JWT） ----
  listActivities: () => req<{ list: Activity[] }>("/api/v1/admin/activities"),
  createActivity: (d: CreateActivityReq) => post<Activity>("/api/v1/admin/activities", d),
  updateActivity: (id: string, d: CreateActivityReq) => put<Activity>(`/api/v1/admin/activities/${id}`, d),
  getActivity: (id: string) => req<ActivityDetail>(`/api/v1/admin/activities/${id}`),
  publish: (id: string) => post(`/api/v1/admin/activities/${id}/publish`),
  pause: (id: string) => post(`/api/v1/admin/activities/${id}/pause`),
  resume: (id: string) => post(`/api/v1/admin/activities/${id}/resume`),
  cancel: (id: string) => post(`/api/v1/admin/activities/${id}/cancel`),
  forceDraw: (id: string) => post(`/api/v1/admin/activities/${id}/draw`),
  updateUiConfig: (id: string, config: UiConfig) =>
    put(`/api/v1/admin/activities/${id}/ui-config`, { config }),
  listParticipants: (id: string) => req<{ list: Participant[] }>(`/api/v1/admin/activities/${id}/participants`),
  importParticipants: (id: string, rows: ParticipantInput[]) =>
    post<{ total: number; failed: number }>(`/api/v1/admin/activities/${id}/participants/import`, { rows }),
  deleteParticipant: (id: string, pid: string) =>
    del(`/api/v1/admin/activities/${id}/participants/${pid}`),
  liveDraw: (id: string, prizeId: string, idempotencyKey: string) =>
    post<LiveDrawResp>(`/api/v1/admin/activities/${id}/live-draw`, { prizeId, idempotencyKey }),
  liveDrawUndo: (id: string, drawId: string) =>
    post(`/api/v1/admin/activities/${id}/live-draw/undo`, { drawId }),
  adminWinners: (id: string) => req<{ list: AdminWinner[] }>(`/api/v1/admin/activities/${id}/winners`),
  offlineRedeem: (id: string, prizeToken: string) =>
    post(`/api/v1/admin/activities/${id}/offline-redeem`, { prizeToken }),
  redeem: (code: string) => post("/api/v1/admin/redeem", { code }),
  upload: (file: File) => {
    const fd = new FormData();
    fd.append("file", file);
    return req<{ url: string }>("/api/v1/admin/upload", { method: "POST", body: fd });
  },

  // ---- C 端公开 ----
  publicActivity: (pid: string) => req<ActivityDetail>(`/api/v1/play/${pid}`),
  feed: (pid: string) => req<{ list: Winner[] }>(`/api/v1/play/${pid}/feed`),
  winners: (pid: string) => req<{ list: Winner[] }>(`/api/v1/play/${pid}/winners`),

  // ---- C 端用户 ----
  enroll: (publicId: string) => post("/api/v1/enroll", { publicId }),
  myPrizes: () => req<{ list: MyPrize[] }>("/api/v1/me/prizes"),
  fillAddress: (d: { prizeToken: string; contactName: string; contactPhone: string; address: string }) =>
    post("/api/v1/me/address", d),

  lotteryJoin: (d: { user_id: string; user_name: string; staff_no?: string }) =>
    post<{ already?: boolean; won?: boolean; name?: string }>("/api/lottery/join", d),
  lotteryParticipants: () =>
    req<{ names: string[]; count?: number; publicJoinUrl?: string }>("/api/lottery/participants"),
  lotteryDraw: (n = 3) => post<{ winners: string[]; remaining?: number }>("/api/lottery/draw", { n }),
  lotterySeedMock: (count = 30) => post<{ added: number; count: number }>("/api/lottery/seed-mock", { count }),
  lotteryMe: (userId: string) =>
    req<{ inPool: boolean; won: boolean; name: string }>(`/api/lottery/me?user_id=${encodeURIComponent(userId || "")}`),
  lotterySession: () =>
    req<{
      wechatEnabled: boolean;
      nickname: string;
      wechatBound: boolean;
      count: number;
      publicJoinUrl?: string;
    }>("/api/lottery/session"),
};

// ---------- 类型 ----------

export type Activity = {
  id: string;
  publicId: string;
  title: string;
  mode: string; // live | scheduled
  rosterSource: string; // import | register | both
  status: string; // draft published running paused ended cancelled drawn
  startAt: number;
  endAt: number;
  playUrl: string;
  tenantName?: string;
};

export type PrizeInput = {
  name: string;
  kind: string; // virtual | physical
  stock: number;
  perRound?: number;
  isAll?: boolean;
  imageUrl?: string;
};

export type CreateActivityReq = {
  title: string;
  mode: string;
  rosterSource?: string;
  timezone?: string;
  startAt: number;
  endAt: number;
  maxEnrollments?: number;
  prizes: PrizeInput[];
};

export type Prize = {
  id: string;
  name: string;
  kind: string;
  stock: number;
  perRound: number;
  isAll: boolean;
  imageUrl: string;
  remain: number;
};

export type UiConfig = {
  topTitle?: string;
  rowCount?: number;
  cardWidth?: number;
  cardHeight?: number;
  cardColor?: string;
  luckyCardColor?: string;
  textColor?: string;
  patternColor?: string;
  background?: string;
  showAvatar?: boolean;
  showPrizeList?: boolean;
  patternList?: number[];
};

export type ActivityDetail = Activity & {
  prizes: Prize[];
  participantN: number;
  winN: number;
  uiConfig?: UiConfig;
};

export type Participant = {
  id: string;
  uid: string;
  name: string;
  department: string;
  identity: string;
  avatarUrl: string;
  source: string; // import | register
  isWin: boolean;
  createdAt: number;
};

export type ParticipantInput = {
  uid: string;
  name: string;
  department?: string;
  identity?: string;
  avatarUrl?: string;
};

export type LiveWinner = {
  participantId: string;
  uid: string;
  name: string;
  department: string;
  identity: string;
  avatarUrl: string;
};

export type LiveDrawResp = {
  drawId: string;
  prizeId: string;
  prizeName: string;
  kind: string;
  winners: LiveWinner[];
  remain: number;
};

export type AdminWinner = {
  participantId: string;
  uid: string;
  name: string;
  department: string;
  prizeName: string;
  kind: string;
  prizeToken: string;
  source: string;
  redeemStatus: string;
  wonAt: number;
};

export type Winner = { nickname: string; prizeName: string; kind?: string; wonAt: number };
export type MyPrize = {
  prizeName: string;
  kind: string;
  status: string;
  redeemCode?: string;
  wonAt: number;
  activity: string;
};
