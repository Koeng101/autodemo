local libB = require("libB")
local primers = libB.primers
local m13fwd = "GTAAAACGACGGCCAGT"
local m13rev = "CAGGAAACAGCTATGAC"
local m13fwd_temp = primers.melting_temp(m13fwd)
local m13rev_temp = primers.melting_temp(m13rev)
print(string.format("M13 Forward Temp: %.1f°C", m13fwd_temp))
print(string.format("M13 Reverse Temp: %.1f°C", m13rev_temp))
