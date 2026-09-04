import { useEffect, useState } from "react";
import { Button, Card, DatePicker, Form, Input, InputNumber, Select, Space, Table, Tag, Typography, message } from "antd";
import { api, logout, type Activity, type ActivityDetail } from "../api";
import { useNavigate } from "react-router-dom";

export default function Admin() {
  const nav = useNavigate();
  const [list, setList] = useState<Activity[]>([]);
  const [detail, setDetail] = useState<ActivityDetail | null>(null);
  const [code, setCode] = useState("");

  async function reload() {
    const r = await api.listActivities();
    setList(r.list || []);
  }
  useEffect(() => {
    reload().catch((e) => message.error(e.message));
  }, []);

  return (
    <div className="admin-wrap">
      <div className="admin-bar">
        <Typography.Title level={4} style={{ margin: 0 }}>LuckyGo 商家后台</Typography.Title>
        <Button onClick={() => { logout(); nav("/login"); }}>退出</Button>
      </div>
      <Space align="start" size={16} wrap>
        <Card title="创建活动" style={{ width: 420 }}>
          <Form
            layout="vertical"
            initialValues={{ mode: "instant", maxDrawsPerUser: 3 }}
            onFinish={async (v) => {
              try {
                const prizes = [
                  { name: v.p1, kind: "physical", stock: v.s1, weight: v.w1 },
                  { name: v.p2, kind: "virtual", stock: v.s2, weight: v.w2 },
                  { name: "谢谢参与", kind: "thank_you", stock: v.s3, weight: v.w3 },
                ];
                await api.createActivity({
                  title: v.title,
                  mode: v.mode,
                  startAt: v.range[0].unix(),
                  endAt: v.range[1].unix(),
                  maxDrawsPerUser: v.maxDrawsPerUser,
                  maxEnrollments: v.maxEnrollments || 0,
                  prizes,
                });
                message.success("已保存草稿，请发布才会生成奖品桶");
                reload();
              } catch (e: any) {
                message.error(e.message);
              }
            }}
          >
            <Form.Item name="title" label="标题" rules={[{ required: true }]}><Input /></Form.Item>
            <Form.Item name="mode" label="玩法">
              <Select options={[{ value: "instant", label: "即时大转盘" }, { value: "scheduled", label: "到期开奖" }]} />
            </Form.Item>
            <Form.Item name="range" label="起止（UTC 按浏览器本地选）" rules={[{ required: true }]}>
              <DatePicker.RangePicker showTime />
            </Form.Item>
            <Form.Item name="maxDrawsPerUser" label="每人限抽"><InputNumber min={1} max={100} /></Form.Item>
            <Typography.Text type="secondary">三档奖品权重之和必须 = 10000</Typography.Text>
            <Form.Item name="p1" label="一等奖名称" initialValue="一等奖"><Input /></Form.Item>
            <Space>
              <Form.Item name="s1" label="库存" initialValue={1}><InputNumber min={1} /></Form.Item>
              <Form.Item name="w1" label="权重" initialValue={500}><InputNumber min={1} /></Form.Item>
            </Space>
            <Form.Item name="p2" label="二等奖名称" initialValue="优惠券"><Input /></Form.Item>
            <Space>
              <Form.Item name="s2" label="库存" initialValue={10}><InputNumber min={1} /></Form.Item>
              <Form.Item name="w2" label="权重" initialValue={1500}><InputNumber min={1} /></Form.Item>
            </Space>
            <Space>
              <Form.Item name="s3" label="谢谢参与库存" initialValue={89}><InputNumber min={1} /></Form.Item>
              <Form.Item name="w3" label="权重" initialValue={8000}><InputNumber min={1} /></Form.Item>
            </Space>
            <Button type="primary" htmlType="submit" block>保存草稿</Button>
          </Form>
        </Card>
        <Card title="活动列表" style={{ minWidth: 520, flex: 1 }}>
          <Table
            rowKey="id"
            dataSource={list}
            size="small"
            pagination={false}
            columns={[
              { title: "标题", dataIndex: "title" },
              { title: "玩法", dataIndex: "mode" },
              { title: "状态", dataIndex: "status", render: (s: string) => <Tag>{s}</Tag> },
              {
                title: "操作",
                render: (_, r) => (
                  <Space>
                    <Button size="small" onClick={() => api.publish(r.id).then(reload).catch((e) => message.error(e.message))}>发布</Button>
                    <Button size="small" onClick={() => api.pause(r.id).then(reload).catch((e) => message.error(e.message))}>暂停</Button>
                    <Button size="small" onClick={() => api.resume(r.id).then(reload).catch((e) => message.error(e.message))}>恢复</Button>
                    <Button size="small" onClick={() => window.open(r.playUrl, "_blank")}>打开C端</Button>
                    <Button size="small" onClick={async () => setDetail(await api.getActivity(r.id))}>详情</Button>
                    {r.mode === "scheduled" && <Button size="small" onClick={() => api.forceDraw(r.id).then(() => message.success("已开奖")).catch((e) => message.error(e.message))}>开奖</Button>}
                  </Space>
                ),
              },
            ]}
          />
          <div style={{ marginTop: 16 }}>
            <Space>
              <Input placeholder="兑换码核销" value={code} onChange={(e) => setCode(e.target.value)} />
              <Button onClick={() => api.redeem(code).then(() => message.success("核销成功")).catch((e) => message.error(e.message))}>核销</Button>
            </Space>
          </div>
          {detail && (
            <pre style={{ marginTop: 12, background: "#f6f6f6", padding: 12 }}>
              {JSON.stringify(detail, null, 2)}
            </pre>
          )}
        </Card>
      </Space>
    </div>
  );
}
