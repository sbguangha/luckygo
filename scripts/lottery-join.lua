-- wrk script: POST /api/lottery/join with a unique id and a random Chinese name.
-- Each request looks like a different person scanning the QR code.

wrk.method = "POST"
wrk.headers["Content-Type"] = "application/json; charset=utf-8"

local xing = {
  "赵","钱","孙","李","周","吴","郑","王","冯","陈","褚","卫","蒋","沈","韩","杨",
  "朱","秦","尤","许","何","吕","施","张","孔","曹","严","华","金","魏","陶","姜",
  "戚","谢","邹","喻","柏","水","窦","章","云","苏","潘","葛","奚","范","彭","郎",
  "鲁","韦","昌","马","苗","凤","花","方","俞","任","袁","柳","鲍","史","唐","费",
  "薛","雷","贺","倪","汤","滕","殷","罗","毕","郝","邬","安","常","乐","于","时",
  "傅","皮","卞","齐","康","伍","余","元","顾","孟","黄","萧","尹","姚","邵","湛",
  "汪","祁","毛","禹","狄","米","贝","明","臧","计","伏","成","戴","谈","宋","茅",
  "庞","熊","纪","舒","屈","项","祝","董","梁","杜","阮","蓝","闵","季","贾","路"
}

local ming = {
  "伟","芳","娜","敏","静","丽","强","磊","洋","勇","军","杰","娟","艳","涛",
  "明","超","霞","平","刚","华","文","辉","鹏","飞","浩","婷","雪","琳","倩",
  "晨","阳","峰","波","斌","健","丹","萍","红","玲","悦","欣","睿","轩","涵",
  "宇","宁","彤","瑶","琪","萱","秀英","桂英","建华","志强","秀珍","桂芳","俊杰",
  "佳琪","雨桐","浩然","子轩","一诺","诗涵","梓萱","浩宇","欣怡","俊熙"
}

local seq = 0

function init(args)
  local seed = os.time()
  if wrk.thread then
    seed = seed + (wrk.thread.addr and 0 or 0)
  end
  -- different seed per Lua state (wrk runs one state per thread)
  math.randomseed(seed + math.floor(os.clock() * 1000000) % 100000)
  seq = math.random(10000, 99999)
end

local function pick(list)
  return list[math.random(1, #list)]
end

local function real_name()
  local n = pick(xing) .. pick(ming)
  -- about half get a 2-char given name already; others append one more char
  if #n < 6 and math.random() < 0.35 then
    n = n .. pick(ming)
    if #n > 21 then
      n = pick(xing) .. pick(ming)
    end
  end
  return n
end

function request()
  seq = seq + 1
  local id = string.format("wrk-%d-%d-%d", seq, math.random(100000, 999999), os.time())
  local name = real_name()
  local body = string.format('{"user_id":"%s","user_name":"%s"}', id, name)
  return wrk.format(nil, nil, nil, body)
end
