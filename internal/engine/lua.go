package engine

const drawLua = `
local bucket = KEYS[1]
local quota  = KEYS[2]
local meta   = KEYS[3]
local idemp  = KEYS[4]
local inflight = KEYS[5]
local remain = KEYS[6]
local now = tonumber(ARGV[1])
local idempTTL = tonumber(ARGV[2])
local inflightTTL = tonumber(ARGV[3])

local existing = redis.call('GET', idemp)
if existing then
  return {'IDEM', existing}
end

local busy = redis.call('SET', inflight, '1', 'NX', 'EX', inflightTTL)
if not busy then
  return {'BUSY'}
end

local status = redis.call('HGET', meta, 'status')
if status ~= 'running' then
  redis.call('DEL', inflight)
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
  redis.call('DEL', inflight)
  return {'NOT_STARTED'}
end
if now >= end_at then
  redis.call('DEL', inflight)
  return {'ENDED'}
end

local max = tonumber(redis.call('HGET', meta, 'max_draws') or '0')
local used = tonumber(redis.call('GET', quota) or '0')
if max > 0 and used >= max then
  redis.call('DEL', inflight)
  return {'QUOTA'}
end

local item = redis.call('LPOP', bucket)
if not item then
  redis.call('DEL', inflight)
  return {'EMPTY'}
end

redis.call('INCR', quota)
redis.call('EXPIRE', quota, 86400 * 120)
redis.call('SET', idemp, item, 'EX', idempTTL)

local pid = string.match(item, '^(%d+):')
if pid then
  redis.call('HINCRBY', remain, pid, -1)
end

return {'OK', item, tostring(max - used - 1)}
`

const undoLua = `
local bucket = KEYS[1]
local quota  = KEYS[2]
local idemp  = KEYS[3]
local remain = KEYS[4]
local expect = ARGV[1]

local got = redis.call('GET', idemp)
if got ~= expect then
  return {'SKIP'}
end
redis.call('DEL', idemp)
local used = tonumber(redis.call('GET', quota) or '0')
if used > 0 then
  redis.call('DECR', quota)
end
redis.call('RPUSH', bucket, expect)
local pid = string.match(expect, '^(%d+):')
if pid then
  redis.call('HINCRBY', remain, pid, 1)
end
return {'OK'}
`

const metaLua = `
local meta = KEYS[1]
local status = ARGV[1]
redis.call('HSET', meta, 'status', status)
return {'OK'}
`
