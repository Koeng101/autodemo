package autodemo

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/koeng101/autodemo/src/autodemosql"
)

const testProtocol = `
function main()
    local script_id = "script1"
    local data_id = "data1"
    
    -- Return format: status, comment, next_func, script, data_passthrough
    return 2, "Starting DNA test", "process_dna", '{"id":"script1"}', '{"script_id":"script1","data_id":"data1"}'
end

function process_dna(data_passthrough)
    if DATA == nil then
        return 1, "No data available", "", "", ""
    end
    
    local data = libB.json.decode(data_passthrough)
    local reading = libB.json.decode(DATA[data.script_id][data.data_id])
    
    if reading.ng_per_ul > 25 then
        return 0, "High DNA concentration", "", "", ""
    else
        return 1, "Low DNA concentration", "", "", ""
    end
end
`

func TestProtocolExecution(t *testing.T) {
	dbPath := "test.db"
	db, wdb := MakeTestDatabase(dbPath)
	defer db.Close()
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	ctx := context.Background()
	projectID := "test-project-1"

	// Create project using WriteDB
	err := wdb.CreateProject(ctx, projectID)
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Add message history using WriteDB
	var historyID int64
	queries := autodemosql.New(db)
	result, _, err := wdb.AddMessageHistory(ctx, projectID, "test Message")
	if err != nil {
		t.Fatalf("Failed to create message history: %v", err)
	}
	historyID = result

	// Create protocol runner and watcher
	runner := NewProtocolRunner(wdb)
	watcher := NewStepWatcher(runner)

	// Start protocol
	err = runner.StartProtocol(ctx, historyID, testProtocol)
	if err != nil {
		t.Fatalf("Failed to start protocol: %v", err)
	}

	// Get steps using regular db connection
	allSteps, err := queries.GetAllStepsForCode(ctx, historyID)
	if err != nil {
		t.Fatalf("Failed to get steps: %v", err)
	}
	if len(allSteps) == 0 {
		t.Fatal("No steps found")
	}
	initialStep := allSteps[len(allSteps)-1]

	if initialStep.Status != 2 {
		t.Errorf("Expected status 2 (continuation), got %d", initialStep.Status)
	}

	// Start watching the step
	watcher.WatchStep(ctx, initialStep.ID)

	// Simulate external system providing data
	testData := `{
		"script1": {
			"data1": "{\"ng_per_ul\": 30}"
		}
	}`

	// Update the step with new data
	err = watcher.UpdateStep(initialStep.ID, testData)
	if err != nil {
		t.Fatalf("Failed to update step: %v", err)
	}

	// Give the watcher time to process
	time.Sleep(500 * time.Millisecond)

	// Read final state
	allSteps, err = queries.GetAllStepsForCode(ctx, historyID)
	if err != nil {
		t.Fatalf("Failed to get final steps: %v", err)
	}

	if len(allSteps) < 2 {
		t.Fatal("Expected at least 2 steps")
	}
	finalStep := allSteps[len(allSteps)-1]

	if finalStep.Status != 0 {
		t.Errorf("Expected status 0 (success), got %d", finalStep.Status)
	}

	if finalStep.StepComment != "High DNA concentration" {
		t.Errorf("Expected comment 'High DNA concentration', got %s", finalStep.StepComment)
	}
}
