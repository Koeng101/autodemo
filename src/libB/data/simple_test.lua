-- return format: status, comment, next_func, script, data
function main()
    local script_id = "script1"
    local data_id = "data1"

    local script = libB.Script.new(script_id)
    local human_commands = libB.HumanCommands.new()
    human_commands:quantify(data_id, "nest_96_wellplate_100ul_pcr_full_skirt", "7", "A1")
    script:add_commands(human_commands)
    local script_json = script:to_json()

    -- Define what data we want to retrieve later
    local data = libB.json.encode({
		script_id = script_id,
		data_id = data_id
    })

    return 2, "Requesting DNA quantification in well", "process_dna", script_json, data
end

function process_dna(input_data)
    local data = libB.json.decode(input_data)
    -- Now we can directly use the data table since keys match our DATA structure
    local dna_ng = libB.json.decode(DATA[data["script_id"]][data["data_id"]])
    if dna_ng["ng_per_ul"] > 25 then
        return 0, "High DNA concentration", "", "", "" 
    else
        return 1, "Low DNA concentration", "", "", ""
    end
end
