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
  headers.set("Content-Type", "application/json");
  const t = getToken();
  if (t) headers.set("Authorization", `Bearer ${t}`);
  const res = await fetch(path, { ...init, headers });
  const body = await res.json().catch(() => ({}));
  if (!res.ok || (body.code && body.code !== 0)) {
    throw new Error(body.msg || "请求失败");
  }
  return (body.data ?? body) as T;
}

export const api = {
  registerTenant: (d: object) => req<{ token: string; role: Role; nickname: string; tenant: string }>("/api/v1/auth/register-tenant", { method: "POST", body: JSON.stringify(d) }),
  login: (d: object) => req<{ token: string; role: Role; nickname: string; tenant: string }>("/api/v1/auth/login", { method: "POST", body: JSON.stringify(d) }),
  registerUser: (d: object) => req<{ token: string; role: Role }>("/api/v1/auth/register-user", { method: "POST", body: JSON.stringify(d) }),
  listActivities: () => req<{ list: Activity[] }>("/api/v1/admin/activities"),
  createActivity: (d: object) => req<Activity>("/api/v1/admin/activities", { method: "POST", body: JSON.stringify(d) }),
  getActivity: (id: string) => req<ActivityDetail>(`/api/v1/admin/activities/${id}`),
  publish: (id: string) => req("/api/v1/admin/activities/" + id + "/publish", { method: "POST", body: "{}" }),
  pause: (id: string) => req("/api/v1/admin/activities/" + id + "/pause", { method: "POST", body: "{}" }),
  resume: (id: string) => req("/api/v1/admin/activities/" + id + "/resume", { method: "POST", body: "{}" }),
  forceDraw: (id: string) => req("/api/v1/admin/activities/" + id + "/draw", { method: "POST", body: "{}" }),
  redeem: (code: string) => req("/api/v1/admin/redeem", { method: "POST", body: JSON.stringify({ code }) }),
  publicActivity: (pid: string) => req<ActivityDetail>("/api/v1/play/" + pid),
  feed: (pid: string) => req<{ list: Winner[] }>("/api/v1/play/" + pid + "/feed"),
  winners: (pid: string) => req<{ list: Winner[] }>("/api/v1/play/" + pid + "/winners"),
  draw: (d: object) => req<DrawResp>("/api/v1/draw", { method: "POST", body: JSON.stringify(d) }),
  enroll: (publicId: string) => req("/api/v1/enroll", { method: "POST", body: JSON.stringify({ publicId }) }),
  myPrizes: () => req<{ list: MyPrize[] }>("/api/v1/me/prizes"),
};

export type Activity = {
  id: string;
  publicId: string;
  title: string;
  mode: string;
  status: string;
  startAt: number;
  endAt: number;
  maxDrawsPerUser: number;
  playUrl: string;
  tenantName?: string;
};
export type Prize = { id: string; name: string; kind: string; stock: number; weight: number; remain: number };
export type ActivityDetail = Activity & { prizes: Prize[]; participantN: number; winN: number };
export type DrawResp = { result: string; prizeId: string; prizeName: string; kind: string; prizeToken: string; redeemCode?: string; remainDraws: number };
export type Winner = { nickname: string; prizeName: string; kind?: string; wonAt: number };
export type MyPrize = { prizeName: string; kind: string; status: string; redeemCode?: string; wonAt: number; activity: string };
