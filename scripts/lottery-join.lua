wrk.method = "POST"
wrk.headers["Content-Type"] = "application/json"

function request()
  local id = tostring(math.random(100000000, 999999999))
  local body = string.format('{"user_id":"%s","user_name":"员工%s"}', id, id)
  return wrk.format(nil, nil, nil, body)
end
