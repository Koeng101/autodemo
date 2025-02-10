package libb

type Script struct {
	ID       string         `json:"id"`
	Commands []CommandGroup `json:"commands"`
}

type Pipette struct {
	Name string `json:"pipette"`
	Side string `json:"side"` // "left" or "right"
}

type LabwareLocation struct {
	Labware  string `json:"labware"`
	DeckSlot string `json:"deck_slot"`
	Address  string `json:"address"`
}

type MoveTo struct {
	Pipette  Pipette  `json:"pipette"`
	Labware  string   `json:"labware"`
	DeckSlot string   `json:"deck_slot"`
	Address  string   `json:"address"`
	X        *float64 `json:"x,omitempty"`
	Y        *float64 `json:"y,omitempty"`
	Z        *float64 `json:"z,omitempty"`
	Module   *string  `json:"module,omitempty"`
}

type ThermocyclerStep struct {
	Temperature     float64 `json:"temperature"`
	HoldTimeSeconds int     `json:"hold_time_seconds"`
}

// Command types for different operations
type MoveToCommand struct {
	Type    string `json:"type"` // "move_to"
	Payload MoveTo `json:"payload"`
}

type AspirateDispensePayload struct {
	Pipette Pipette `json:"pipette"`
	Volume  float64 `json:"vol"`
	MoveTo  *MoveTo `json:"move_to,omitempty"`
}

type AspirateDispenseCommand struct {
	Type    string                  `json:"type"` // "aspirate" or "dispense"
	Payload AspirateDispensePayload `json:"payload"`
}

type PickUpTipPayload struct {
	Pipette  Pipette `json:"pipette"`
	Labware  string  `json:"labware"`
	DeckSlot string  `json:"deck_slot"`
	Address  string  `json:"address"`
}

type PickUpTipCommand struct {
	Type    string           `json:"type"` // "pick_up_tip"
	Payload PickUpTipPayload `json:"payload"`
}

type DropTipPayload struct {
	Pipette  Pipette `json:"pipette"`
	Labware  *string `json:"labware,omitempty"`
	DeckSlot *string `json:"deck_slot,omitempty"`
	Address  *string `json:"address,omitempty"`
	Trash    bool    `json:"trash"`
}

type DropTipCommand struct {
	Type    string         `json:"type"` // "drop_tip"
	Payload DropTipPayload `json:"payload"`
}

type HomeCommand struct {
	Type string `json:"type"` // "home"
}

// Thermocycler commands
type TCSetLidTempPayload struct {
	Temperature float64 `json:"temperature"`
}

type TCSetLidTempCommand struct {
	Type    string              `json:"type"` // "tc_set_lid_temp"
	Payload TCSetLidTempPayload `json:"payload"`
}

type TCSetBlockTempPayload struct {
	Temperature     float64  `json:"temperature"`
	HoldTimeMinutes *float64 `json:"hold_time_minutes,omitempty"`
	HoldTimeSeconds *float64 `json:"hold_time_seconds,omitempty"`
	BlockMaxVolume  *float64 `json:"block_max_volume,omitempty"`
}

type TCSetBlockTempCommand struct {
	Type    string                `json:"type"` // "tc_set_block_temp"
	Payload TCSetBlockTempPayload `json:"payload"`
}

type TCExecuteProfilePayload struct {
	Steps          []ThermocyclerStep `json:"steps"`
	Repetitions    int                `json:"repetitions"`
	BlockMaxVolume *float64           `json:"block_max_volume,omitempty"`
}

type TCExecuteProfileCommand struct {
	Type    string                  `json:"type"` // "tc_execute_profile"
	Payload TCExecuteProfilePayload `json:"payload"`
}

type TCSimpleCommand struct {
	Type string `json:"type"` // "tc_open_lid", "tc_close_lid", "tc_deactivate_lid", "tc_deactivate_block"
}

// Human commands
type QuantifyPayload struct {
	Labware   string `json:"labware"`
	DeckSlot  string `json:"deck_slot"`
	Address   string `json:"address"`
	ReturnKey string `json:"return_key"`
}

type QuantifyCommand struct {
	Type    string          `json:"type"` // "quantify"
	Payload QuantifyPayload `json:"payload"`
}

// Command groups
type OpentronsCommand struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload,omitempty"`
}

type CommandGroup struct {
	CommandType string        `json:"command_type"` // "opentrons" or "human"
	Payload     []interface{} `json:"payload"`
}


// GetReturnKeys extracts all return keys from a Script's commands
func (s *Script) GetReturnKeys() map[string]string {
    returnKeys := make(map[string]string) // scriptID -> returnKey

    for _, commandGroup := range s.Commands {
        if commandGroup.CommandType == "human" {
            for _, payloadInterface := range commandGroup.Payload {
                // Convert the interface{} to a map to check command type
                if payloadMap, ok := payloadInterface.(map[string]interface{}); ok {
                    if cmdType, ok := payloadMap["type"].(string); ok {
                        switch cmdType {
                        case "quantify":
                            // Extract return key from quantify command
                            if payload, ok := payloadMap["payload"].(map[string]interface{}); ok {
                                if returnKey, ok := payload["return_key"].(string); ok {
                                    returnKeys[s.ID] = returnKey
                                }
                            }
                        // Add other human command types that have return keys here
                        }
                    }
                }
            }
        }
    }

    return returnKeys
}

// HasAllData checks if all required return data is present in the provided data map
func (s *Script) HasAllData(data map[string]map[string]string) bool {
    returnKeys := s.GetReturnKeys()
    
    for scriptID, returnKey := range returnKeys {
        if _, ok := data[scriptID]; !ok {
            return false
        }
        if _, ok := data[scriptID][returnKey]; !ok {
            return false
        }
    }
    
    return true
}