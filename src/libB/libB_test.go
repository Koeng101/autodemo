package libb

import (
	_ "embed"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	lua "github.com/yuin/gopher-lua"
)

func TestCompileTealToLua(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "Basic compilation",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CompileTealToLua()
			if (err != nil) != tt.wantErr {
				t.Errorf("CompileTealToLua() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got == "" {
				t.Error("CompileTealToLua() returned empty string when expecting valid Lua code")
			}
		})
	}
}

func TestExecuteLua(t *testing.T) {
	tests := []struct {
		name       string
		luaCode    string
		wantOutput string
		wantErr    bool
	}{
		{
			name:       "Print test",
			luaCode:    "print('hello')",
			wantOutput: "hello\n",
			wantErr:    false,
		},
		{
			name:       "Invalid Lua code",
			luaCode:    "this is not valid lua",
			wantOutput: "",
			wantErr:    true,
		},
		{
			name:       "Empty input",
			luaCode:    "",
			wantOutput: "",
			wantErr:    false,
		},
		{
			name:       "Multiple prints",
			luaCode:    "print('hello') print('world')",
			wantOutput: "hello\nworld\n",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExecuteLua(tt.luaCode)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExecuteLua() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.wantOutput {
				t.Errorf("ExecuteLua() = %q, want %q", got, tt.wantOutput)
			}
		})
	}
}

func TestCustomPrint(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "Single argument",
			args:     []string{"hello"},
			expected: "hello\n",
		},
		{
			name:     "Multiple arguments",
			args:     []string{"hello", "world"},
			expected: "hello\tworld\n",
		},
		{
			name:     "Empty argument",
			args:     []string{""},
			expected: "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			L := lua.NewState()
			defer L.Close()

			var buf strings.Builder
			printFn := customPrint(&buf)

			// Push all arguments to the Lua stack
			for _, arg := range tt.args {
				L.Push(lua.LString(arg))
			}

			// Call our custom print function
			printFn(L)

			if got := buf.String(); got != tt.expected {
				t.Errorf("customPrint() output = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestLibBIntegration tests the actual integration with your Teal-compiled library
func TestLibBIntegration(t *testing.T) {
	// This test will need to be customized based on your actual libB.tl functionality
	tests := []struct {
		name       string
		luaCode    string
		wantOutput string
		wantErr    bool
	}{
		{
			name:       "Basic libB function call",
			luaCode:    "local result = libB.generate_protocol() print(result)",
			wantOutput: `{"id":"da50bbdd-e80c-4fd7-a27b-f0c2275e3f07","commands":[{"payload":[{"type":"home"},{"payload":{"pipette":{"pipette":"p20_single_gen2","side":"right"},"labware":"opentrons_96_tiprack_20ul","deck_slot":"1","address":"A1"},"type":"pick_up_tip"},{"payload":{"pipette":{"pipette":"p20_single_gen2","side":"right"},"move_to":{"pipette":{"pipette":"p20_single_gen2","side":"right"},"address":"A1","labware":"opentrons_24_tuberack_nest_1.5ml_snapcap","deck_slot":"2"},"vol":10},"type":"aspirate"},{"payload":{"pipette":{"pipette":"p20_single_gen2","side":"right"},"move_to":{"pipette":{"pipette":"p20_single_gen2","side":"right"},"module":"thermocycler","address":"A1","labware":"nest_96_wellplate_100ul_pcr_full_skirt","deck_slot":"7"},"vol":10},"type":"dispense"},{"payload":{"pipette":{"pipette":"p20_single_gen2","side":"right"},"trash":true},"type":"drop_tip"},{"payload":{"pipette":{"pipette":"p20_single_gen2","side":"right"},"labware":"opentrons_96_tiprack_20ul","deck_slot":"1","address":"B1"},"type":"pick_up_tip"},{"payload":{"pipette":{"pipette":"p20_single_gen2","side":"right"},"move_to":{"pipette":{"pipette":"p20_single_gen2","side":"right"},"address":"B1","labware":"opentrons_24_tuberack_nest_1.5ml_snapcap","deck_slot":"2"},"vol":2},"type":"aspirate"},{"payload":{"pipette":{"pipette":"p20_single_gen2","side":"right"},"move_to":{"pipette":{"pipette":"p20_single_gen2","side":"right"},"module":"thermocycler","address":"A1","labware":"nest_96_wellplate_100ul_pcr_full_skirt","deck_slot":"7"},"vol":2},"type":"dispense"},{"payload":{"pipette":{"pipette":"p20_single_gen2","side":"right"},"trash":true},"type":"drop_tip"},{"payload":{"pipette":{"pipette":"p20_single_gen2","side":"right"},"labware":"opentrons_96_tiprack_20ul","deck_slot":"1","address":"C1"},"type":"pick_up_tip"},{"payload":{"pipette":{"pipette":"p20_single_gen2","side":"right"},"move_to":{"pipette":{"pipette":"p20_single_gen2","side":"right"},"address":"C1","labware":"opentrons_24_tuberack_nest_1.5ml_snapcap","deck_slot":"2"},"vol":1},"type":"aspirate"},{"payload":{"pipette":{"pipette":"p20_single_gen2","side":"right"},"move_to":{"pipette":{"pipette":"p20_single_gen2","side":"right"},"module":"thermocycler","address":"A1","labware":"nest_96_wellplate_100ul_pcr_full_skirt","deck_slot":"7"},"vol":1},"type":"dispense"},{"payload":{"pipette":{"pipette":"p20_single_gen2","side":"right"},"trash":true},"type":"drop_tip"},{"payload":{"pipette":{"pipette":"p20_single_gen2","side":"right"},"labware":"opentrons_96_tiprack_20ul","deck_slot":"1","address":"D1"},"type":"pick_up_tip"},{"payload":{"pipette":{"pipette":"p20_single_gen2","side":"right"},"move_to":{"pipette":{"pipette":"p20_single_gen2","side":"right"},"address":"D1","labware":"opentrons_24_tuberack_nest_1.5ml_snapcap","deck_slot":"2"},"vol":7},"type":"aspirate"},{"payload":{"pipette":{"pipette":"p20_single_gen2","side":"right"},"move_to":{"pipette":{"pipette":"p20_single_gen2","side":"right"},"module":"thermocycler","address":"A1","labware":"nest_96_wellplate_100ul_pcr_full_skirt","deck_slot":"7"},"vol":7},"type":"dispense"},{"payload":{"pipette":{"pipette":"p20_single_gen2","side":"right"},"trash":true},"type":"drop_tip"},{"type":"home"},{"type":"tc_open_lid"},{"payload":{"temperature":100},"type":"tc_set_lid_temp"},{"payload":{"steps":[{"hold_time_seconds":30,"temperature":95},{"hold_time_seconds":30,"temperature":57},{"hold_time_seconds":60,"temperature":72}],"repetitions":30,"block_max_volume":50},"type":"tc_execute_profile"},{"type":"tc_deactivate_block"},{"type":"tc_deactivate_lid"}],"command_type":"opentrons"},{"payload":[{"payload":{"return_key":"b6885800-f454-4f19-9c60-ff725aa2d51a","labware":"nest_96_wellplate_100ul_pcr_full_skirt","deck_slot":"7","address":"A1"},"type":"quantify"}],"command_type":"human"}]}`,
			wantErr:    false,
		},
		// Add more test cases based on your libB functionality
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExecuteLua(tt.luaCode)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExecuteLua() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				// Trim newlines for JSON comparison
				got = strings.TrimSpace(got)

				// Parse both JSONs
				var gotJSON, wantJSON interface{}
				if err := json.Unmarshal([]byte(got), &gotJSON); err != nil {
					t.Errorf("Failed to parse actual output as JSON: %v\nOutput was: %s", err, got)
					return
				}
				if err := json.Unmarshal([]byte(tt.wantOutput), &wantJSON); err != nil {
					t.Errorf("Failed to parse expected output as JSON: %v\nOutput was: %s", err, tt.wantOutput)
					return
				}

				// Compare the parsed structures
				if !reflect.DeepEqual(gotJSON, wantJSON) {
					t.Errorf("JSON outputs are not equivalent.\nGot: %+v\nWant: %+v", gotJSON, wantJSON)
				}
			}
		})
	}
}

//go:embed data/simple_test.lua
var simple_test string

func TestExampleDB(t *testing.T) {
	db := NewMockDB()

	// Start protocol
	if err := db.StartProtocol(simple_test); err != nil {
		t.Fatalf("Failed to start protocol: %v", err)
	}

	// Simulate script execution results
	db.scriptChan <- ScriptExecution{
		ScriptID: "script1",
		DataID:   "data1",
		Result:   `{"ng_per_ul": 30}`,
	}

	// Wait a bit for processing
	time.Sleep(500 * time.Millisecond)

	// Check if specific script is still running
	if db.IsRunning("script1") {
		t.Error("Script should have completed")
	}
}
