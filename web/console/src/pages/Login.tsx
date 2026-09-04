import { useState } from "react";
import { Button, Card, Form, Input, Tabs, message } from "antd";
import { useNavigate } from "react-router-dom";
import { api, setSession } from "../api";

export default function Login() {
  const nav = useNavigate();
  const [loading, setLoading] = useState(false);

  async function onLogin(v: { tenantName: string; account: string; password: string }) {
    setLoading(true);
    try {
      const r = await api.login(v);
      setSession(r.token, r.role);
      message.success("登录成功");
      nav(r.role === "admin" ? "/admin" : "/login");
    } catch (e: any) {
      message.error(e.message);
    } finally {
      setLoading(false);
    }
  }

  async function onRegister(v: { tenantName: string; account: string; password: string; nickname?: string }) {
    setLoading(true);
    try {
      const r = await api.registerTenant(v);
      setSession(r.token, r.role);
      message.success("租户已创建");
      nav("/admin");
    } catch (e: any) {
      message.error(e.message);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="center-page">
      <Card title="LuckyGo 营销抽奖" style={{ width: 420 }}>
        <Tabs
          items={[
            {
              key: "login",
              label: "商家登录",
              children: (
                <Form layout="vertical" onFinish={onLogin}>
                  <Form.Item name="tenantName" label="租户名" rules={[{ required: true }]}><Input /></Form.Item>
                  <Form.Item name="account" label="账号" rules={[{ required: true }]}><Input /></Form.Item>
                  <Form.Item name="password" label="密码" rules={[{ required: true }]}><Input.Password /></Form.Item>
                  <Button type="primary" htmlType="submit" block loading={loading}>登录</Button>
                </Form>
              ),
            },
            {
              key: "reg",
              label: "注册租户",
              children: (
                <Form layout="vertical" onFinish={onRegister}>
                  <Form.Item name="tenantName" label="租户名" rules={[{ required: true }]}><Input placeholder="例如 demo-shop" /></Form.Item>
                  <Form.Item name="account" label="管理员账号" rules={[{ required: true }]}><Input /></Form.Item>
                  <Form.Item name="password" label="密码" rules={[{ required: true, min: 6 }]}><Input.Password /></Form.Item>
                  <Form.Item name="nickname" label="昵称"><Input /></Form.Item>
                  <Button type="primary" htmlType="submit" block loading={loading}>创建并进入后台</Button>
                </Form>
              ),
            },
          ]}
        />
      </Card>
    </div>
  );
}
