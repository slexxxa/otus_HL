#!lua name=dialog

redis.register_function('send_message', function(keys, args)
    local from_user = args[1]
    local to_user   = args[2]
    local text      = args[3]
    local ts        = redis.call('TIME')[1]

    local pair1 = from_user
    local pair2 = to_user

    if pair1 > pair2 then
        pair1, pair2 = pair2, pair1
    end

    local dialogKey = 'dialog:' .. pair1 .. ':' .. pair2
    local versionKey = 'dialog_version:' .. pair1 .. ':' .. pair2
    local seqKey = dialogKey .. ':seq'

    local id = redis.call('INCR', seqKey)

    local msg = cjson.encode({
        id = id,
        from_user = from_user,
        to_user = to_user,
        text = text,
        created_at = ts
    })

    redis.call('RPUSH', dialogKey, msg)
    redis.call('INCR', versionKey)

    return id
end)

redis.register_function('get_dialog', function(keys, args)
    local u1 = args[1]
    local u2 = args[2]

    local pair1 = u1
    local pair2 = u2

    if pair1 > pair2 then
        pair1, pair2 = pair2, pair1
    end

    local dialogKey = 'dialog:' .. pair1 .. ':' .. pair2

    return redis.call('LRANGE', dialogKey, 0, -1)
end)
