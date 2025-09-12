package redislock

// LuaCheckAndDeleteDistributionLock 判断是否拥有分布式锁的归属权，是则删除
// KEYS[1]: Redis EVAL 命令传入的第一个键参数
// ARGV[1]: Redis EVAL 命令传入的第一个非键参数
// redis.call('get',lockerKey): lua 执行 redis 命令的方式
const LuaCheckAndDeleteDistributionLock = `
  local lockerKey = KEYS[1]
  local targetToken = ARGV[1]
  local getToken = redis.call('get',lockerKey) 
  if (not getToken or getToken ~= targetToken) then 
	return 0
	else
		return redis.call('del',lockerKey)
  end
`

// LuaCheckAndExpireDistributionLock 判断是否拥有分布式锁的归属权，是则续期
const LuaCheckAndExpireDistributionLock = `
  local lockerKey = KEYS[1] 
  local targetToken = ARGV[1]
  local duration = ARGV[2]
  local getToken = redis.call('get',lockerKey)
  if (not getToken or getToken ~= targetToken) then
    return 0
	else
		return redis.call('expire',lockerKey,duration)
  end
`
