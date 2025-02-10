package libb

import (
	_ "embed"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/xeipuuv/gojsonschema"
)

//go:embed schema.json
var protocolSchema string

func TestProtocolSerialization(t *testing.T) {
	// Execute the Lua code to get the protocol JSON
	result, err := ExecuteLua("local result = libB.generate_protocol() print(result)")
	if err != nil {
		t.Fatalf("Failed to execute Lua code: %v", err)
	}

	// Trim any whitespace/newlines from the result
	result = strings.TrimSpace(result)

	// Step 1: Validate against JSON schema
	schemaLoader := gojsonschema.NewStringLoader(protocolSchema)
	documentLoader := gojsonschema.NewStringLoader(result)

	validation, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		t.Fatalf("Error validating JSON: %v", err)
	}

	if !validation.Valid() {
		for _, desc := range validation.Errors() {
			t.Errorf("JSON validation error: %s", desc)
		}
		t.Fatal("JSON schema validation failed")
	}

	// Step 2: Deserialize into Go structs
	var script Script
	if err := json.Unmarshal([]byte(result), &script); err != nil {
		t.Fatalf("Failed to unmarshal JSON into Go structs: %v", err)
	}

	// Step 3: Serialize back to JSON
	reserializedJSON, err := json.Marshal(script)
	if err != nil {
		t.Fatalf("Failed to marshal Go structs back to JSON: %v", err)
	}

	// Step 4: Validate reserialized JSON against schema
	documentLoader = gojsonschema.NewStringLoader(string(reserializedJSON))
	validation, err = gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		t.Fatalf("Error validating reserialized JSON: %v", err)
	}

	if !validation.Valid() {
		for _, desc := range validation.Errors() {
			t.Errorf("Reserialized JSON validation error: %s", desc)
		}
		t.Fatal("Reserialized JSON schema validation failed")
	}

	// Step 5: Deep compare original and reserialized JSON
	var originalParsed, reserializedParsed interface{}
	if err := json.Unmarshal([]byte(result), &originalParsed); err != nil {
		t.Fatalf("Failed to parse original JSON: %v", err)
	}
	if err := json.Unmarshal(reserializedJSON, &reserializedParsed); err != nil {
		t.Fatalf("Failed to parse reserialized JSON: %v", err)
	}

	if !reflect.DeepEqual(originalParsed, reserializedParsed) {
		t.Error("Reserialized JSON differs from original")
		// Print detailed comparison for debugging
		t.Errorf("\nOriginal: %+v\nReserialized: %+v", originalParsed, reserializedParsed)
	}
}

// Helper function to test individual command serialization
func TestCommandSerialization(t *testing.T) {
	tests := []struct {
		name    string
		command interface{}
		jsonStr string
	}{
		{
			name: "MoveToCommand",
			command: MoveToCommand{
				Type: "move_to",
				Payload: MoveTo{
					Pipette: Pipette{
						Name: "p20_single_gen2",
						Side: "right",
					},
					Labware:  "opentrons_96_tiprack_20ul",
					DeckSlot: "1",
					Address:  "A1",
				},
			},
			jsonStr: `{"type":"move_to","payload":{"pipette":{"pipette":"p20_single_gen2","side":"right"},"labware":"opentrons_96_tiprack_20ul","deck_slot":"1","address":"A1"}}`,
		},
		{
			name: "AspirateCommand",
			command: AspirateDispenseCommand{
				Type: "aspirate",
				Payload: AspirateDispensePayload{
					Pipette: Pipette{
						Name: "p20_single_gen2",
						Side: "right",
					},
					Volume: 10.0,
				},
			},
			jsonStr: `{"type":"aspirate","payload":{"pipette":{"pipette":"p20_single_gen2","side":"right"},"vol":10}}`,
		},
		// Add more test cases for other command types
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test serialization
			serialized, err := json.Marshal(tt.command)
			if err != nil {
				t.Errorf("Failed to marshal %s: %v", tt.name, err)
				return
			}

			if string(serialized) != tt.jsonStr {
				t.Errorf("Serialized JSON doesn't match expected.\nGot:  %s\nWant: %s", string(serialized), tt.jsonStr)
			}

			// Test deserialization
			commandType := reflect.TypeOf(tt.command)
			deserialized := reflect.New(commandType).Interface()

			if err := json.Unmarshal([]byte(tt.jsonStr), deserialized); err != nil {
				t.Errorf("Failed to unmarshal %s: %v", tt.name, err)
				return
			}

			// Get the actual value (dereference the pointer)
			deserializedValue := reflect.ValueOf(deserialized).Elem().Interface()

			if !reflect.DeepEqual(deserializedValue, tt.command) {
				t.Errorf("Deserialized value doesn't match original.\nGot:  %+v\nWant: %+v", deserializedValue, tt.command)
			}
		})
	}
}
