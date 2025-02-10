package libb

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"

	lua "github.com/yuin/gopher-lua"
)

//go:embed tl.lua
var tlCompiler string

//go:embed libB.tl
var tealContent string

// CompileTealToLua compiles Teal code to Lua using the embedded tl compiler
func CompileTealToLua() (string, error) {
	L := lua.NewState()
	defer L.Close()

	// Load the Teal compiler
	if err := L.DoString(tlCompiler); err != nil {
		return "", fmt.Errorf("failed to load teal compiler: %v", err)
	}
	L.SetGlobal("tl", L.Get(-1))
	L.Pop(1)

	// Compile the Teal code to Lua
	modifiedTealContent := strings.Replace(tealContent, "local json_func = load([[", "local json_func = loadstring([[", 1)
	err := L.DoString(fmt.Sprintf("return tl.gen([========[%s]========])", modifiedTealContent))
	if err != nil {
		return "", fmt.Errorf("failed to compile teal code: %v", err)
	}

	// Get the compiled Lua code
	compiledLua := L.Get(1).String()
	if compiledLua == "" {
		return "", fmt.Errorf("compilation produced empty output")
	}

	return compiledLua, nil
}

// customPrint is a function that mimics Lua's print function but writes to an io.Writer
func customPrint(writer io.Writer) func(L *lua.LState) int {
	return func(L *lua.LState) int {
		top := L.GetTop()
		for i := 1; i <= top; i++ {
			str := L.ToString(i)
			if i > 1 {
				io.WriteString(writer, "\t")
			}
			io.WriteString(writer, str)
		}
		io.WriteString(writer, "\n")
		return 0
	}
}

// ExecuteLua executes the provided Lua code with the compiled libB available
func ExecuteLua(code string) (string, error) {
	// First, compile the Teal code to Lua
	compiledLibB, err := CompileTealToLua()
	if err != nil {
		return "", fmt.Errorf("failed to compile libB: %v", err)
	}

	L := lua.NewState()
	defer L.Close()

	// Add stdout
	var buffer strings.Builder
	L.SetGlobal("print", L.NewFunction(customPrint(&buffer)))

	// Load the compiled library content
	if err := L.DoString(compiledLibB); err != nil {
		return "", fmt.Errorf("failed to load compiled libB: %v", err)
	}

	// Store returned table as global
	L.SetGlobal("libB", L.Get(-1))
	L.Pop(1)

	// Execute the user's code
	if err := L.DoString(code); err != nil {
		return "", err
	}

	return buffer.String(), nil
}

/*
 */
type ProtocolState struct {
	Status     int
	Comments   string
	NextFunc   string
	Script     *Script
	DataNeeded string
}

type MockDB struct {
	states     map[string]*ProtocolState    // script_id -> state
	data       map[string]map[string]string // script_id -> data_id -> json
	scriptChan chan ScriptExecution         // channel for new script results
	mu         sync.RWMutex
}

type ScriptExecution struct {
	ScriptID string
	DataID   string
	Result   string // JSON result
}

// Create a new mock database
func NewMockDB() *MockDB {
	return &MockDB{
		states:     make(map[string]*ProtocolState),
		data:       make(map[string]map[string]string),
		scriptChan: make(chan ScriptExecution, 100),
	}
}

// ExecuteLuaStep executes a single step of the protocol
func ExecuteLuaStep(code string, funcName string, inputData string, data map[string]map[string]string) (*ProtocolState, error) {
	compiledLibB, err := CompileTealToLua()
	if err != nil {
		return nil, fmt.Errorf("failed to compile libB: %v", err)
	}

	L := lua.NewState()
	defer L.Close()

	// Set up DATA table
	dataTable := L.NewTable()
	for scriptID, innerMap := range data {
		scriptTable := L.NewTable()
		for dataID, jsonStr := range innerMap {
			L.SetField(scriptTable, dataID, lua.LString(jsonStr))
		}
		L.SetField(dataTable, scriptID, scriptTable)
	}
	L.SetGlobal("DATA", dataTable)

	// Load libB
	if err := L.DoString(compiledLibB); err != nil {
		return nil, fmt.Errorf("failed to load compiled libB: %v", err)
	}
	L.SetGlobal("libB", L.Get(-1))
	L.Pop(1)

	// Load protocol code
	if err := L.DoString(code); err != nil {
		return nil, fmt.Errorf("failed to load protocol code: %v", err)
	}

	// Execute function
	fn := L.GetGlobal(funcName)
	if fn.Type() != lua.LTFunction {
		return nil, fmt.Errorf("function %s not found", funcName)
	}

	L.Push(fn)
	if inputData != "" {
		L.Push(lua.LString(inputData))
	}

	numArgs := 0
	if inputData != "" {
		numArgs = 1
	}

	if err := L.PCall(numArgs, 5, nil); err != nil {
		return nil, fmt.Errorf("error calling %s: %v", funcName, err)
	}

	// Parse return values
	state := &ProtocolState{
		Status:     int(L.CheckNumber(-5)),
		Comments:   L.CheckString(-4),
		DataNeeded: L.CheckString(-1),
	}

	if nextFunc := L.Get(-3); nextFunc.Type() != lua.LTNil {
		state.NextFunc = nextFunc.String()
	}

	scriptJSON := L.CheckString(-2)
	if scriptJSON != "" {
		var script Script
		if err := json.Unmarshal([]byte(scriptJSON), &script); err != nil {
			return nil, fmt.Errorf("failed to parse script JSON: %v", err)
		}
		state.Script = &script
	}

	L.Pop(5)
	return state, nil
}

// MockDB methods
func (db *MockDB) StartProtocol(code string) error {
	state, err := ExecuteLuaStep(code, "main", "", make(map[string]map[string]string))
	if err != nil {
		return err
	}

	if state.Script != nil {
		db.mu.Lock()
		db.states[state.Script.ID] = state
		db.mu.Unlock()

		// Start goroutine to watch for script completion
		go db.watchScriptCompletion(code, state)
	}

	return nil
}

func (db *MockDB) watchScriptCompletion(code string, initialState *ProtocolState) {
	for execution := range db.scriptChan {
		log.Printf("Received execution result for script %s, data %s",
			execution.ScriptID, execution.DataID)

		db.mu.Lock()
		state, exists := db.states[execution.ScriptID]
		if !exists {
			log.Printf("No state found for script %s", execution.ScriptID)
			db.mu.Unlock()
			continue
		}

		// Update data store
		if _, exists := db.data[execution.ScriptID]; !exists {
			db.data[execution.ScriptID] = make(map[string]string)
		}
		db.data[execution.ScriptID][execution.DataID] = execution.Result
		log.Printf("Updated data store for script %s, data %s",
			execution.ScriptID, execution.DataID)

		// If we have a script object, check if we have all required data
		if state.Script != nil {
			hasAllData := state.Script.HasAllData(db.data)
			log.Printf("Checking data completion for script %s: %v", state.Script.ID, hasAllData)

			if hasAllData {
				log.Printf("All required data present, continuing execution")
				// Continue execution
				newState, err := ExecuteLuaStep(code, state.NextFunc, state.DataNeeded, db.data)
				if err != nil {
					log.Printf("Error continuing execution: %v", err)
					db.mu.Unlock()
					continue
				}

				log.Printf("New state: %+v", newState)

				// Check if we're in a terminal state (status 0 or 1)
				if newState.Status <= 1 {
					log.Printf("Reached terminal state (status %d), cleaning up script %s",
						newState.Status, execution.ScriptID)
					delete(db.states, execution.ScriptID)
					db.mu.Unlock()
					return
				}

				// Only continue watching if we have a new script to watch
				if newState.Script != nil {
					db.states[newState.Script.ID] = newState
					db.mu.Unlock()
					go db.watchScriptCompletion(code, newState)
				} else {
					db.mu.Unlock()
				}
			} else {
				log.Printf("Still waiting for required data")
				db.mu.Unlock()
			}
		} else {
			log.Printf("No script object in state, skipping")
			db.mu.Unlock()
		}
	}
}

func (db *MockDB) IsRunning(scriptID string) bool {
	db.mu.RLock()
	defer db.mu.RUnlock()
	_, exists := db.states[scriptID]
	return exists
}

func (db *MockDB) GetState(scriptID string) *ProtocolState {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.states[scriptID]
}
