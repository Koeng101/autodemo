-- In order to get type checking, we load the json.lua as a string, while we
-- have the definition file outside as json.
local record json_type
    type Array = {Value}
    type Object = {string:Value}

    -- Workaround for not being able to union Array and Object
    type Mixed = {string|integer:Value}

    type Value = number | string | boolean | Mixed

    -- For convenience, encode takes any type and may throw an error at runtime.
    -- You can enforce correctness yourself by specifying types on variables or records.
    --
    -- For example:
    --     local obj: json.Object = {value = 'data'}
    --     local encoded = json.encode(obj)

    encode: function(value: any): string
    -- encode: function(value: Value): string

    decode: function(str: string): Value
end

local json_func = load([[
--
-- json.lua
--
-- Copyright (c) 2020 rxi
--
-- Permission is hereby granted, free of charge, to any person obtaining a copy of
-- this software and associated documentation files (the "Software"), to deal in
-- the Software without restriction, including without limitation the rights to
-- use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies
-- of the Software, and to permit persons to whom the Software is furnished to do
-- so, subject to the following conditions:
--
-- The above copyright notice and this permission notice shall be included in all
-- copies or substantial portions of the Software.
--
-- THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
-- IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
-- FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
-- AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
-- LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
-- OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
-- SOFTWARE.
--

local json = { _version = "0.1.2" }

-------------------------------------------------------------------------------
-- Encode
-------------------------------------------------------------------------------

local encode

local escape_char_map = {
  [ "\\" ] = "\\",
  [ "\"" ] = "\"",
  [ "\b" ] = "b",
  [ "\f" ] = "f",
  [ "\n" ] = "n",
  [ "\r" ] = "r",
  [ "\t" ] = "t",
}

local escape_char_map_inv = { [ "/" ] = "/" }
for k, v in pairs(escape_char_map) do
  escape_char_map_inv[v] = k
end


local function escape_char(c)
  return "\\" .. (escape_char_map[c] or string.format("u%04x", c:byte()))
end


local function encode_nil(val)
  return "null"
end


local function encode_table(val, stack)
  local res = {}
  stack = stack or {}

  -- Circular reference?
  if stack[val] then error("circular reference") end

  stack[val] = true

  if rawget(val, 1) ~= nil or next(val) == nil then
    -- Treat as array -- check keys are valid and it is not sparse
    local n = 0
    for k in pairs(val) do
      if type(k) ~= "number" then
        error("invalid table: mixed or invalid key types")
      end
      n = n + 1
    end
    if n ~= #val then
      error("invalid table: sparse array")
    end
    -- Encode
    for i, v in ipairs(val) do
      table.insert(res, encode(v, stack))
    end
    stack[val] = nil
    return "[" .. table.concat(res, ",") .. "]"

  else
    -- Treat as an object
    for k, v in pairs(val) do
      if type(k) ~= "string" then
        error("invalid table: mixed or invalid key types")
      end
      table.insert(res, encode(k, stack) .. ":" .. encode(v, stack))
    end
    stack[val] = nil
    return "{" .. table.concat(res, ",") .. "}"
  end
end


local function encode_string(val)
  return '"' .. val:gsub('[%z\1-\31\\"]', escape_char) .. '"'
end


local function encode_number(val)
  -- Check for NaN, -inf and inf
  if val ~= val or val <= -math.huge or val >= math.huge then
    error("unexpected number value '" .. tostring(val) .. "'")
  end
  return string.format("%.14g", val)
end


local type_func_map = {
  [ "nil"     ] = encode_nil,
  [ "table"   ] = encode_table,
  [ "string"  ] = encode_string,
  [ "number"  ] = encode_number,
  [ "boolean" ] = tostring,
}


encode = function(val, stack)
  local t = type(val)
  local f = type_func_map[t]
  if f then
    return f(val, stack)
  end
  error("unexpected type '" .. t .. "'")
end


function json.encode(val)
  return ( encode(val) )
end


-------------------------------------------------------------------------------
-- Decode
-------------------------------------------------------------------------------

local parse

local function create_set(...)
  local res = {}
  for i = 1, select("#", ...) do
    res[ select(i, ...) ] = true
  end
  return res
end

local space_chars   = create_set(" ", "\t", "\r", "\n")
local delim_chars   = create_set(" ", "\t", "\r", "\n", "]", "}", ",")
local escape_chars  = create_set("\\", "/", '"', "b", "f", "n", "r", "t", "u")
local literals      = create_set("true", "false", "null")

local literal_map = {
  [ "true"  ] = true,
  [ "false" ] = false,
  [ "null"  ] = nil,
}


local function next_char(str, idx, set, negate)
  for i = idx, #str do
    if set[str:sub(i, i)] ~= negate then
      return i
    end
  end
  return #str + 1
end


local function decode_error(str, idx, msg)
  local line_count = 1
  local col_count = 1
  for i = 1, idx - 1 do
    col_count = col_count + 1
    if str:sub(i, i) == "\n" then
      line_count = line_count + 1
      col_count = 1
    end
  end
  error( string.format("%s at line %d col %d", msg, line_count, col_count) )
end


local function codepoint_to_utf8(n)
  -- http://scripts.sil.org/cms/scripts/page.php?site_id=nrsi&id=iws-appendixa
  local f = math.floor
  if n <= 0x7f then
    return string.char(n)
  elseif n <= 0x7ff then
    return string.char(f(n / 64) + 192, n % 64 + 128)
  elseif n <= 0xffff then
    return string.char(f(n / 4096) + 224, f(n % 4096 / 64) + 128, n % 64 + 128)
  elseif n <= 0x10ffff then
    return string.char(f(n / 262144) + 240, f(n % 262144 / 4096) + 128,
                       f(n % 4096 / 64) + 128, n % 64 + 128)
  end
  error( string.format("invalid unicode codepoint '%x'", n) )
end


local function parse_unicode_escape(s)
  local n1 = tonumber( s:sub(1, 4),  16 )
  local n2 = tonumber( s:sub(7, 10), 16 )
   -- Surrogate pair?
  if n2 then
    return codepoint_to_utf8((n1 - 0xd800) * 0x400 + (n2 - 0xdc00) + 0x10000)
  else
    return codepoint_to_utf8(n1)
  end
end


local function parse_string(str, i)
  local res = ""
  local j = i + 1
  local k = j

  while j <= #str do
    local x = str:byte(j)

    if x < 32 then
      decode_error(str, j, "control character in string")

    elseif x == 92 then -- `\`: Escape
      res = res .. str:sub(k, j - 1)
      j = j + 1
      local c = str:sub(j, j)
      if c == "u" then
        local hex = str:match("^[dD][89aAbB]%x%x\\u%x%x%x%x", j + 1)
                 or str:match("^%x%x%x%x", j + 1)
                 or decode_error(str, j - 1, "invalid unicode escape in string")
        res = res .. parse_unicode_escape(hex)
        j = j + #hex
      else
        if not escape_chars[c] then
          decode_error(str, j - 1, "invalid escape char '" .. c .. "' in string")
        end
        res = res .. escape_char_map_inv[c]
      end
      k = j + 1

    elseif x == 34 then -- `"`: End of string
      res = res .. str:sub(k, j - 1)
      return res, j + 1
    end

    j = j + 1
  end

  decode_error(str, i, "expected closing quote for string")
end


local function parse_number(str, i)
  local x = next_char(str, i, delim_chars)
  local s = str:sub(i, x - 1)
  local n = tonumber(s)
  if not n then
    decode_error(str, i, "invalid number '" .. s .. "'")
  end
  return n, x
end


local function parse_literal(str, i)
  local x = next_char(str, i, delim_chars)
  local word = str:sub(i, x - 1)
  if not literals[word] then
    decode_error(str, i, "invalid literal '" .. word .. "'")
  end
  return literal_map[word], x
end


local function parse_array(str, i)
  local res = {}
  local n = 1
  i = i + 1
  while 1 do
    local x
    i = next_char(str, i, space_chars, true)
    -- Empty / end of array?
    if str:sub(i, i) == "]" then
      i = i + 1
      break
    end
    -- Read token
    x, i = parse(str, i)
    res[n] = x
    n = n + 1
    -- Next token
    i = next_char(str, i, space_chars, true)
    local chr = str:sub(i, i)
    i = i + 1
    if chr == "]" then break end
    if chr ~= "," then decode_error(str, i, "expected ']' or ','") end
  end
  return res, i
end


local function parse_object(str, i)
  local res = {}
  i = i + 1
  while 1 do
    local key, val
    i = next_char(str, i, space_chars, true)
    -- Empty / end of object?
    if str:sub(i, i) == "}" then
      i = i + 1
      break
    end
    -- Read key
    if str:sub(i, i) ~= '"' then
      decode_error(str, i, "expected string for key")
    end
    key, i = parse(str, i)
    -- Read ':' delimiter
    i = next_char(str, i, space_chars, true)
    if str:sub(i, i) ~= ":" then
      decode_error(str, i, "expected ':' after key")
    end
    i = next_char(str, i + 1, space_chars, true)
    -- Read value
    val, i = parse(str, i)
    -- Set
    res[key] = val
    -- Next token
    i = next_char(str, i, space_chars, true)
    local chr = str:sub(i, i)
    i = i + 1
    if chr == "}" then break end
    if chr ~= "," then decode_error(str, i, "expected '}' or ','") end
  end
  return res, i
end


local char_func_map = {
  [ '"' ] = parse_string,
  [ "0" ] = parse_number,
  [ "1" ] = parse_number,
  [ "2" ] = parse_number,
  [ "3" ] = parse_number,
  [ "4" ] = parse_number,
  [ "5" ] = parse_number,
  [ "6" ] = parse_number,
  [ "7" ] = parse_number,
  [ "8" ] = parse_number,
  [ "9" ] = parse_number,
  [ "-" ] = parse_number,
  [ "t" ] = parse_literal,
  [ "f" ] = parse_literal,
  [ "n" ] = parse_literal,
  [ "[" ] = parse_array,
  [ "{" ] = parse_object,
}


parse = function(str, idx)
  local chr = str:sub(idx, idx)
  local f = char_func_map[chr]
  if f then
    return f(str, idx)
  end
  decode_error(str, idx, "unexpected character '" .. chr .. "'")
end


function json.decode(str)
  if type(str) ~= "string" then
    error("expected argument of type string, got " .. type(str))
  end
  local res, idx = parse(str, next_char(str, 1, space_chars, true))
  idx = next_char(str, idx, space_chars, true)
  if idx <= #str then
    decode_error(str, idx, "trailing garbage")
  end
  return res
end


return json

]]) as function(): json_type
local json = json_func()

local record UUID
  -- Generate a new UUID v4
  generate: function(): string

  -- Validate a UUID string
  validate: function(uuid: string): boolean
end

-- Local helper function for hex conversion
local function to_hex(num: number, len: number): string
  local hex_str = string.format("%x", math.floor(num))
  return string.rep("0", math.floor(len - #hex_str)) .. hex_str
end

-- Bitwise AND operation implementation
local function band(a: number, b: number): number
  local result = 0
  local bit = 1
  for _ = 0, 7 do
    if (a % (bit * 2) >= bit) and (b % (bit * 2) >= bit) then
      result = result + bit
    end
    bit = bit * 2
  end
  return result
end

-- Bitwise OR operation implementation
local function bor(a: number, b: number): number
  local result = 0
  local bit = 1
  for _ = 0, 7 do
    if (a % (bit * 2) >= bit) or (b % (bit * 2) >= bit) then
      result = result + bit
    end
    bit = bit * 2
  end
  return result
end

-- Implementation
local uuid: UUID = {
  validate = function(uuid: string): boolean
    -- Check basic format (8-4-4-4-12 hexadecimal digits)
    local pattern = "^%x%x%x%x%x%x%x%x%-%x%x%x%x%-%x%x%x%x%-%x%x%x%x%-%x%x%x%x%x%x%x%x%x%x%x%x$"
    if not uuid:match(pattern) then
      return false
    end

    -- Check version (should be 4)
    local version = tonumber(uuid:sub(15, 15), 16)
    if version ~= 4 then
      return false
    end

    -- Check variant (should be 1)
    local variant = tonumber(uuid:sub(20, 20), 16)
    if variant < 8 or variant > 11 then
      return false
    end

    return true
  end,

  generate = function(): string
    -- Use math.random as a source of randomness
    math.randomseed(os.time())
    
    -- Generate 16 random bytes
    local bytes: {number} = {}
    for i = 1, 16 do
      bytes[i] = math.random(0, 255)
    end

    -- Set version to 4 (random UUID)
    bytes[7] = bor(band(bytes[7], 0x0f), 0x40)
    
    -- Set variant to 1 (RFC4122)
    bytes[9] = bor(band(bytes[9], 0x3f), 0x80)

    -- Format bytes into UUID string
    return string.format(
      "%s%s%s%s-%s%s-%s%s-%s%s-%s%s%s%s%s%s",
      to_hex(bytes[1], 2), to_hex(bytes[2], 2),
      to_hex(bytes[3], 2), to_hex(bytes[4], 2),
      to_hex(bytes[5], 2), to_hex(bytes[6], 2),
      to_hex(bytes[7], 2), to_hex(bytes[8], 2),
      to_hex(bytes[9], 2), to_hex(bytes[10], 2),
      to_hex(bytes[11], 2), to_hex(bytes[12], 2),
      to_hex(bytes[13], 2), to_hex(bytes[14], 2),
      to_hex(bytes[15], 2), to_hex(bytes[16], 2)
    )
  end
}

local interface CommandPayload
end

local interface Commands
	command_type: string
	payload: {CommandPayload}
end

--[[***************************************************************************

								Opentrons commands

***************************************************************************--]]


local interface OpentronsCommandPayload
end

local interface OpentronsCommand  is CommandPayload
   type: string
   payload: OpentronsCommandPayload
end

local record Pipette
    pipette: string
    side: string
end

local record MoveToPayload is OpentronsCommandPayload
    pipette: Pipette
    labware: string
    deck_slot: string
    address: string
    x: number | nil
    y: number | nil
    z: number | nil
    module: string | nil
end

local record MoveToOpentronsCommand is OpentronsCommand
    where self.type == "move_to"
    payload: MoveToPayload
end

local record AspiratePayload is OpentronsCommandPayload
    pipette: Pipette
    vol: number
    move_to: MoveToPayload | nil
end

local record AspirateOpentronsCommand is OpentronsCommand
    where self.type == "aspirate"
    payload: AspiratePayload
end

local record DispensePayload is OpentronsCommandPayload
    pipette: Pipette
    vol: number
    move_to: MoveToPayload | nil
end

local record DispenseOpentronsCommand is OpentronsCommand
    where self.type == "dispense"
    payload: DispensePayload
end

local record PickUpTipPayload is OpentronsCommandPayload
    pipette: Pipette
    labware: string
    deck_slot: string
    address: string
end

local record PickUpTipOpentronsCommand is OpentronsCommand 
    where self.type == "pick_up_tip"
    payload: PickUpTipPayload
end

local record DropTipPayload is OpentronsCommandPayload
    pipette: Pipette
    labware: string | nil
    deck_slot: string | nil
    address: string | nil
    trash: boolean
end

local record DropTipOpentronsCommand is OpentronsCommand
    where self.type == "drop_tip"
    payload: DropTipPayload
end

local record HomeOpentronsCommand is OpentronsCommand
    where self.type == "home"
end

local record ThermocyclerProfileStep
    temperature: number
    hold_time_seconds: number
end

local record TCSetLidTempPayload is OpentronsCommandPayload
    temperature: number
end

local record TCSetLidTempOpentronsCommand is OpentronsCommand
    where self.type == "tc_set_lid_temp"
    payload: TCSetLidTempPayload
end

local record TCSetBlockTempPayload is OpentronsCommandPayload
    temperature: number
    hold_time_minutes: number | nil
    hold_time_seconds: number | nil
    block_max_volume: number | nil
end

local record TCSetBlockTempOpentronsCommand is OpentronsCommand
    where self.type == "tc_set_block_temp"
    payload: TCSetBlockTempPayload
end

local record TCExecuteProfilePayload is OpentronsCommandPayload
    steps: {ThermocyclerProfileStep}
    repetitions: number
    block_max_volume: number | nil
end

local record TCExecuteProfileOpentronsCommand is OpentronsCommand
    where self.type == "tc_execute_profile"
    payload: TCExecuteProfilePayload
end

local record TCOpenLidOpentronsCommand is OpentronsCommand
    where self.type == "tc_open_lid"
end

local record TCCloseLidOpentronsCommand is OpentronsCommand
    where self.type == "tc_close_lid"
end

local record TCDeactivateLidOpentronsCommand is OpentronsCommand
    where self.type == "tc_deactivate_lid"
end

local record TCDeactivateBlockOpentronsCommand is OpentronsCommand
    where self.type == "tc_deactivate_block"
end

local record Labware
    labware: string
    deck_slot: string
	module: string | nil
end

function Labware:well(address: string): MoveToPayload
    return {
        pipette = nil as Pipette, -- Will be filled in by the command
        labware = self.labware,
        deck_slot = self.deck_slot,
        address = address,
        x = nil,
        y = nil,
        z = nil,
        module = self.module
    }
end

function Labware.new(labware: string, deck_slot: string, module?: string): Labware
    local self: Labware = setmetatable({}, { __index = Labware })
    self.labware = labware
    self.deck_slot = deck_slot
	self.module = module
    return self
end

local function copy_move_to_with_pipette(move_to: MoveToPayload | nil, pipette: Pipette): MoveToPayload | nil
    if move_to == nil then
        return nil
    end
	move_to = move_to as MoveToPayload 
    
    return {
        pipette = pipette,
        labware = move_to.labware,
        deck_slot = move_to.deck_slot,
        address = move_to.address,
        x = move_to.x,
        y = move_to.y,
        z = move_to.z,
        module = move_to.module
    }
end

-- Define OpentronsCommands record and methods
local record OpentronsCommands is Commands
	where self.command_type == "opentrons"
    payload: {OpentronsCommand}
    move_to: function(OpentronsCommands, Pipette, MoveToPayload): OpentronsCommands
    aspirate: function(OpentronsCommands, Pipette, number, ?MoveToPayload): OpentronsCommands
    dispense: function(OpentronsCommands, Pipette, number, ?MoveToPayload): OpentronsCommands
    pick_up_tip: function(OpentronsCommands, Pipette, MoveToPayload): OpentronsCommands
    drop_tip: function(OpentronsCommands, Pipette, ?MoveToPayload): OpentronsCommands
	home: function(OpentronsCommands): OpentronsCommands
    tc_set_lid_temp: function(OpentronsCommands, number): OpentronsCommands
    tc_set_block_temp: function(OpentronsCommands, number, ?number, ?number, ?number): OpentronsCommands
    tc_execute_profile: function(OpentronsCommands, {ThermocyclerProfileStep}, number, ?number): OpentronsCommands
    tc_open_lid: function(OpentronsCommands): OpentronsCommands
    tc_close_lid: function(OpentronsCommands): OpentronsCommands
    tc_deactivate_lid: function(OpentronsCommands): OpentronsCommands
    tc_deactivate_block: function(OpentronsCommands): OpentronsCommands
    to_json: function(OpentronsCommands): string
end

function OpentronsCommands.new(): OpentronsCommands
    local self: OpentronsCommands = setmetatable({}, { __index = OpentronsCommands })
	self.command_type = "opentrons"
	self.payload = {}
    return self
end

function OpentronsCommands:move_to(pipette: Pipette, move_to: MoveToPayload): OpentronsCommands
    move_to.pipette = pipette
    local command: MoveToOpentronsCommand = {
        type = "move_to",
        payload = move_to
    }
    table.insert(self.payload, command)
    return self
end

function OpentronsCommands:aspirate(pipette: Pipette, vol: number, move_to?: MoveToPayload): OpentronsCommands
    local command: AspirateOpentronsCommand = {
        type = "aspirate",
        payload = {
            pipette = pipette,
            vol = vol,
            move_to = copy_move_to_with_pipette(move_to, pipette)
        }
    }
    table.insert(self.payload, command)
    return self
end

function OpentronsCommands:dispense(pipette: Pipette, vol: number, move_to?: MoveToPayload): OpentronsCommands
    local command: DispenseOpentronsCommand = {
        type = "dispense",
        payload = {
            pipette = pipette,
            vol = vol,
            move_to = copy_move_to_with_pipette(move_to, pipette)
        }
    }
    table.insert(self.payload, command)
    return self
end

function OpentronsCommands:pick_up_tip(pipette: Pipette, move_to: MoveToPayload): OpentronsCommands
    local command: PickUpTipOpentronsCommand = {
        type = "pick_up_tip",
        payload = {
            pipette = pipette,
            labware = move_to.labware,
            deck_slot = move_to.deck_slot,
            address = move_to.address
        }
    }
    table.insert(self.payload, command)
    return self
end

function OpentronsCommands:drop_tip(pipette: Pipette, move_to?: MoveToPayload): OpentronsCommands
    local command: DropTipOpentronsCommand
    if move_to == nil then
        command = {
            type = "drop_tip",
            payload = {
                pipette = pipette,
                labware = nil,
                deck_slot = nil,
                address = nil,
                trash = true
            }
        }
    else
        command = {
            type = "drop_tip",
            payload = {
                pipette = pipette,
                labware = move_to.labware,
                deck_slot = move_to.deck_slot,
                address = move_to.address,
                trash = false
            }
        }
    end
    table.insert(self.payload, command)
    return self
end

function OpentronsCommands:home(): OpentronsCommands
    local command: HomeOpentronsCommand = {
        type = "home",
    }
    table.insert(self.payload, command)
    return self
end

function OpentronsCommands:tc_set_lid_temp(temperature: number): OpentronsCommands
    local command: TCSetLidTempOpentronsCommand = {
        type = "tc_set_lid_temp",
        payload = {
            temperature = temperature
        }
    }
    table.insert(self.payload, command)
    return self
end

function OpentronsCommands:tc_set_block_temp(temperature: number, hold_time_minutes: number, hold_time_seconds: number, block_max_volume: number): OpentronsCommands
    local command: TCSetBlockTempOpentronsCommand = {
        type = "tc_set_block_temp",
        payload = {
            temperature = temperature,
            hold_time_minutes = hold_time_minutes,
            hold_time_seconds = hold_time_seconds,
            block_max_volume = block_max_volume
        }
    }
    table.insert(self.payload, command)
    return self
end

function OpentronsCommands:tc_execute_profile(steps: {ThermocyclerProfileStep}, repetitions: number, block_max_volume: number): OpentronsCommands
    local command: TCExecuteProfileOpentronsCommand = {
        type = "tc_execute_profile",
        payload = {
            steps = steps,
            repetitions = repetitions,
            block_max_volume = block_max_volume
        }
    }
    table.insert(self.payload, command)
    return self
end

function OpentronsCommands:tc_open_lid(): OpentronsCommands
    local command: TCOpenLidOpentronsCommand = {
        type = "tc_open_lid",
    }
    table.insert(self.payload, command)
    return self
end

function OpentronsCommands:tc_close_lid(): OpentronsCommands
    local command: TCCloseLidOpentronsCommand = {
        type = "tc_close_lid",
    }
    table.insert(self.payload, command)
    return self
end

function OpentronsCommands:tc_deactivate_lid(): OpentronsCommands
    local command: TCDeactivateLidOpentronsCommand = {
        type = "tc_deactivate_lid",
    }
    table.insert(self.payload, command)
    return self
end

function OpentronsCommands:tc_deactivate_block(): OpentronsCommands
    local command: TCDeactivateBlockOpentronsCommand = {
        type = "tc_deactivate_block",
    }
    table.insert(self.payload, command)
    return self
end

function OpentronsCommands:to_json(): string
    return json.encode({ self })
end

--[[***************************************************************************

                                HumanCommands

***************************************************************************--]]

local interface HumanCommandPayload
end

local interface HumanCommand is CommandPayload
   type: string
   payload: HumanCommandPayload
end

local record QuantifyPayload is HumanCommandPayload
	return_key: string
    labware: string
    deck_slot: string
    address: string
end

local record QuantifyHumanCommand is HumanCommand
    where self.type == "quantify"
    payload: QuantifyPayload
end

local record HumanCommands is Commands
    where self.command_type == "human"
    payload: {HumanCommand}
    quantify: function(HumanCommands, string, string, string, string): HumanCommands
    to_json: function(HumanCommands): string
end

function HumanCommands.new(): HumanCommands
    local self: HumanCommands = setmetatable({}, { __index = HumanCommands })
    self.command_type = "human"
	self.payload = {}
    return self
end

function HumanCommands:quantify(return_key: string, labware: string, deck_slot: string, address: string): HumanCommands
    local command: QuantifyHumanCommand = {
        type = "quantify",
        payload = {
			return_key = return_key,
            labware = labware,
            deck_slot = deck_slot,
            address = address
        }
    }
    table.insert(self.payload, command)
    return self
end

function HumanCommands:to_json(): string
    return json.encode({ self })
end

--[[***************************************************************************

                                Script

***************************************************************************--]]

local record Script
	id: string
    commands: {Commands}
    to_json: function(Script): string
    add_commands: function(Script, Commands): Script
end

function Script.new(id: string): Script
    local self: Script = setmetatable({}, { __index = Script })
	self.id = id
    self.commands = {}
    return self
end

function Script:add_commands(commands: Commands): Script
    table.insert(self.commands, commands)
    return self
end

function Script:to_json(): string
    return json.encode(self)
end

--[[***************************************************************************

                                Examples

***************************************************************************--]]

-- Function to generate the example protocol
local function generate_protocol(): string
    -- Create a new script to hold all commands
    local script = Script.new("da50bbdd-e80c-4fd7-a27b-f0c2275e3f07")
    
    -- Create opentrons commands
    local opentrons_commands = OpentronsCommands.new()
    local pipette: Pipette = { pipette = "p20_single_gen2", side = "right" }
    local tips = Labware.new("opentrons_96_tiprack_20ul", "1")
    local source = Labware.new("opentrons_24_tuberack_nest_1.5ml_snapcap", "2")
    local destination = Labware.new("nest_96_wellplate_100ul_pcr_full_skirt", "7", "thermocycler")

    opentrons_commands:home()

    -- Add mastermix
    opentrons_commands
       :pick_up_tip(pipette, tips:well("A1"))
       :aspirate(pipette, 10, source:well("A1"))
       :dispense(pipette, 10, destination:well("A1"))
       :drop_tip(pipette)
   -- Add primers
   opentrons_commands
    :pick_up_tip(pipette, tips:well("B1"))
    :aspirate(pipette, 2, source:well("B1"))
    :dispense(pipette, 2, destination:well("A1"))
    :drop_tip(pipette)
   -- Add template
   opentrons_commands
    :pick_up_tip(pipette, tips:well("C1"))
    :aspirate(pipette, 1, source:well("C1"))
    :dispense(pipette, 1, destination:well("A1"))
    :drop_tip(pipette)
    opentrons_commands
    :pick_up_tip(pipette, tips:well("D1"))
    :aspirate(pipette, 7, source:well("D1"))
    :dispense(pipette, 7, destination:well("A1"))
    :drop_tip(pipette)

    opentrons_commands:home()

    -- Add thermocycling
    opentrons_commands
    :tc_open_lid()
    :tc_set_lid_temp(100)
    --:tc_set_block_temp(4)
    :tc_execute_profile({
        { temperature = 95, hold_time_seconds = 30 },
        { temperature = 57, hold_time_seconds = 30 },
        { temperature = 72, hold_time_seconds = 60 }
    }, 30, 50)
    :tc_deactivate_block()
    :tc_deactivate_lid()

    -- Add opentrons commands to script
    script:add_commands(opentrons_commands)

    -- Create and add human commands for quantification
    local human_commands = HumanCommands.new()
    human_commands:quantify("b6885800-f454-4f19-9c60-ff725aa2d51a", destination.labware, destination.deck_slot, "A1")
    script:add_commands(human_commands)

    -- Get JSON of entire script
    return script:to_json()
end

return {
	Script = Script,
	OpentronsCommands = OpentronsCommands,
	Labware = Labware,
	HumanCommands = HumanCommands,
	json = json,
	generate_protocol = generate_protocol,
	uuid = uuid
}
