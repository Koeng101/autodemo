-- spec/primers_spec.tl
local libB = require("libB")
local primers = libB.primers

describe("Primers", function()
  describe("marmur_doty", function()
    it("calculates correct melting temperature", function()
      local test_seq = "ACGTCCGGACTT"
      local expected_tm = 31.0
      local calc_tm = primers.marmur_doty(test_seq)
      assert.are.equal(expected_tm, calc_tm)
    end)
  end)

  describe("santa_lucia", function()
    it("calculates correct melting temperature within 2% tolerance", function()
      local test_seq = "ACGATGGCAGTAGCATGC"
      local test_c_primer = 0.1e-6    -- 0.1 µM
      local test_c_na = 350e-3        -- 350 mM
      local test_c_mg = 0.0           -- 0 mM
      local expected_tm = 62.7
      
      local calc_tm = primers.santa_lucia(test_seq, test_c_primer, test_c_na, test_c_mg)
      local relative_error = math.abs(expected_tm - calc_tm) / expected_tm
      
      assert.is_true(relative_error < 0.02, 
        string.format("SantaLucia has changed on test. Got %f instead of %f", calc_tm, expected_tm))
    end)
  end)

  describe("melting_temp", function()
    it("calculates correct melting temperature within 2% tolerance", function()
      local test_seq = "GTAAAACGACGGCCAGT" -- M13 fwd
      local expected_tm = 52.8
      
      local calc_tm = primers.melting_temp(test_seq)
      local relative_error = math.abs(expected_tm - calc_tm) / expected_tm
      
      assert.is_true(relative_error < 0.02,
        string.format("MeltingTemp has changed on test. Got %f instead of %f", calc_tm, expected_tm))
    end)
  end)
end)
