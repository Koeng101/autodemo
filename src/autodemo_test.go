package autodemo

import (
	"context"
	"os"
	"testing"

	"github.com/koeng101/autodemo/src/autodemosql"
)

func TestCreateProject(t *testing.T) {
	dbPath := "test.db"
	db, wdb := MakeTestDatabase(dbPath)
	defer db.Close()
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	ctx := context.Background()
	projectID := "test-project-1"

	// Test project creation
	err := wdb.CreateProject(ctx, projectID)
	if err != nil {
		t.Errorf("Failed to create project: %s", err)
	}

	// Verify project exists
	queries := autodemosql.New(db)
	_, err = queries.GetProjectByID(ctx, projectID)
	if err != nil {
		t.Errorf("Failed to check if project exists: %s", err)
	}

	// Test duplicate project creation (should fail)
	err = wdb.CreateProject(ctx, projectID)
	if err == nil {
		t.Error("Expected error when creating duplicate project, got nil")
	}
}

func TestAddMessageHistory(t *testing.T) {
	dbPath := "test.db"
	db, wdb := MakeTestDatabase(dbPath)
	defer db.Close()
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	ctx := context.Background()
	testContent := "Test message content"
	projectID := "test-project-1"
	_ = wdb.CreateProject(ctx, projectID)

	// Test adding message
	id, createdAt, err := wdb.AddMessageHistory(ctx, projectID, testContent)
	if err != nil {
		t.Errorf("Failed to add message history: %s", err)
	}
	if id <= 0 {
		t.Error("Expected positive ID for created message")
	}
	if createdAt <= 0 {
		t.Error("Expected positive timestamp for created message")
	}

	// Verify message exists and content matches
	queries := autodemosql.New(db)
	message, err := queries.GetMessageHistoryByID(ctx, id)
	if err != nil {
		t.Errorf("Failed to get message: %s", err)
	}
	if message.Content != testContent {
		t.Errorf("Message content mismatch. Got %s, want %s", message.Content, testContent)
	}
}

func TestAddProjectData(t *testing.T) {
	dbPath := "test.db"
	db, wdb := MakeTestDatabase(dbPath)
	defer db.Close()
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	ctx := context.Background()
	dataName := "test-data"
	functionName := "test-function"
	projectID := "test-project-1"
	_ = wdb.CreateProject(ctx, projectID)
	data := "test data content"

	// Test adding project data
	id, createdAt, err := wdb.AddProjectData(ctx, projectID, dataName, functionName, data)
	if err != nil {
		t.Errorf("Failed to add project data: %s", err)
	}
	if id <= 0 {
		t.Error("Expected positive ID for created project data")
	}
	if createdAt <= 0 {
		t.Error("Expected positive timestamp for created project data")
	}

	// Verify project data exists and content matches
	queries := autodemosql.New(db)
	projectData, err := queries.GetProjectDataByID(ctx, id)
	if err != nil {
		t.Errorf("Failed to get project data: %s", err)
	}
	if projectData.DataName != dataName {
		t.Errorf("Data name mismatch. Got %s, want %s", projectData.DataName, dataName)
	}
	if projectData.FunctionName != functionName {
		t.Errorf("Function name mismatch. Got %s, want %s", projectData.FunctionName, functionName)
	}
	if projectData.Data != data {
		t.Errorf("Data content mismatch. Got %s, want %s", projectData.Data, data)
	}
}

func TestParseToMessages(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Message
	}{
		{
			name: "Simple conversation",
			input: `<|begin_of_text|><|start_header_id|>system<|end_header_id|>
System message
<|eot_id|>
<|start_header_id|>user<|end_header_id|>
User message
<|eot_id|>
<|start_header_id|>assistant<|end_header_id|>
Assistant message`,
			expected: []Message{
				{Role: "system", Content: "System message"},
				{Role: "user", Content: "User message"},
				{Role: "assistant", Content: "Assistant message"},
			},
		},
		{
			name: "Empty message handling",
			input: `<|begin_of_text|><|start_header_id|>system<|end_header_id|>
System message
<|eot_id|>
<|start_header_id|>user<|end_header_id|>

<|eot_id|>
<|start_header_id|>assistant<|end_header_id|>
Assistant message`,
			expected: []Message{
				{Role: "system", Content: "System message"},
				{Role: "assistant", Content: "Assistant message"},
			},
		},
		{
			name:     "Invalid format handling",
			input:    `Invalid format without proper tags`,
			expected: []Message{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseToMessages(tt.input)
			if len(got) != len(tt.expected) {
				t.Errorf("parseToMessages() got %v messages, want %v messages", len(got), len(tt.expected))
				return
			}

			for i := range got {
				if got[i].Role != tt.expected[i].Role {
					t.Errorf("Message[%d] Role = %v, want %v", i, got[i].Role, tt.expected[i].Role)
				}
				if got[i].Content != tt.expected[i].Content {
					t.Errorf("Message[%d] Content = %v, want %v", i, got[i].Content, tt.expected[i].Content)
				}
			}
		})
	}
}
