counter = 0
wrk.method = "PATCH"
wrk.headers["Content-Type"] = "application/json"

request = function()
    path = "asudhfaoshfaisuhfaisduhfauisdhfaisuhfaisudhfasiuhfaishdaiusih" .. counter
    wrk.body = "{\"data\":{\"type\":\"tab_xnscxln\",\"attributes\":{\"name\":\"" .. path .. " \"},\"id\":\"78ba9497-258f-4998-a480-915e4414e739\"},\"meta\":{}}"
    counter = counter + 1
    return wrk.format(nil, "/api/tab_xnscxln/78ba9497-258f-4998-a480-915e4414e739")
end

