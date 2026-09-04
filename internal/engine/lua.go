package engine

// metaLua 原子翻转活动在 Redis 中的状态。
const metaLua = `
local meta = KEYS[1]
local status = ARGV[1]
redis.call('HSET', meta, 'status', status)
return {'OK'}
`

// liveDrawLua 现场大屏批量抽取：
// 幂等命中 -> 状态/时间窗校验 -> 名单池版本校验 -> SPOP 原子弹出 N 个参与者 -> 记录幂等结果。
// 名单池由 Go 侧在 STALE 时从 MySQL 重建（lg:live:poolbuilt 记录构建时的名单版本）。
const liveDrawLua = `
local pool      = KEYS[1]
local meta      = KEYS[2]
local idemp     = KEYS[3]
local built     = KEYS[4]
local now       = tonumber(ARGV[1])
local idempTTL  = tonumber(ARGV[2])
local count     = tonumber(ARGV[3])
local rosterVer = ARGV[4]
local drawId    = ARGV[5]

local existing = redis.call('GET', idemp)
if existing then
  return {'IDEM', existing}
end

local status = redis.call('HGET', meta, 'status')
if status ~= 'running' then
  if status == 'paused' then
    return {'PAUSED'}
  end
  if status == 'ended' or status == 'drawn' or status == 'cancelled' then
    return {'ENDED'}
  end
  return {'STATUS', status or ''}
end

local start_at = tonumber(redis.call('HGET', meta, 'start_at') or '0')
local end_at = tonumber(redis.call('HGET', meta, 'end_at') or '0')
if now < start_at then
  return {'NOT_STARTED'}
end
if now >= end_at then
  return {'ENDED'}
end

if (redis.call('GET', built) or '') ~= rosterVer then
  return {'STALE'}
end

local size = redis.call('SCARD', pool)
if size < count then
  return {'INSUFFICIENT', tostring(size)}
end

local winners = redis.call('SPOP', pool, count)
redis.call('SET', idemp, drawId .. ':' .. table.concat(winners, ','), 'EX', idempTTL)
return {'OK', drawId, unpack(winners)}
`

// liveUndoLua 取消一批大屏抽取：校验幂等值属于该批次且未取消后，改写为 UNDONE 标记（保留原 TTL）。
const liveUndoLua = `
local idemp  = KEYS[1]
local drawId = ARGV[1]
local v = redis.call('GET', idemp)
if not v then
  return {'NONE'}
end
if v == drawId .. ':UNDONE' then
  return {'UNDONE'}
end
if string.sub(v, 1, #drawId + 1) ~= drawId .. ':' then
  return {'MISMATCH'}
end
local ttl = redis.call('TTL', idemp)
if ttl > 0 then
  redis.call('SET', idemp, drawId .. ':UNDONE', 'EX', ttl)
else
  redis.call('SET', idemp, drawId .. ':UNDONE')
end
return {'OK'}
`
