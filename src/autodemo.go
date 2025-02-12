package autodemo

import (
	"bytes"
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/koeng101/autodemo/src/autodemosql"
	libb "github.com/koeng101/autodemo/src/libB"
	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/sashabaranov/go-openai"
)

// export API_KEY="" # get from deepinfra
// export MODEL="meta-llama/Llama-3.3-70B-Instruct-Turbo"
// export BASE_URL="https://api.deepinfra.com/v1/openai"
// export PORT=8080

/******************************************************************************

App / Chat interface.

This is the core of the application, which is the chat interface into the
autodemo. It handles the core interaction of the user with the system: ie,
asking for experiments to be written, and having those experiments written
for them.

I use lua because I can easily limit and sandbox it within the system to run
autonomously. It is likely that reasoning will later use python or the like,
but lua acts as a nice scripting language inside the system for operating
the lab.

******************************************************************************/

var LuaPrompt = `You are a helpful assistant writing biological code. Can do this in two ways: sandbox mode, or script mode. In sandbox mode, you can answer simple queries from the user using code. You should do math with this.

user: What is 8+8?
assistant: <lua_sandbox>
print(8+8)
</lua_sandbox>
tool: 16

user: How many base pairs are in "ATGC"?
assistant: <lua_sandbox>
print(#"ATGC")
<lua_sandbox>
tool: 4

The script mode is the one for creating biological protocols. It is more advanced, and comes preloaded with the libB library. Here is some example usage:

user: Home my robot for me
assistant: <lua_script>
function main() -- scripts always initiate at main
	-- setup script
	local script_id = libB.uuid.generate()
	local script = libB.Script.new(script_id)

	-- setup opentrons commands
	local commands = libB.OpentronsCommands.new()
	commands:home()
	script:add_commands(commands)

	-- there are always 5 returns from scripts:
	-- 1. a status code: 0 for success, 1 for failure, and 2 for continuation
	-- 2. a comment for the user
	-- 3. the next function to run (may be blank)
	-- 4. the script json
	-- 5. data to be passed into the next function
	return 0, "Homing the robot", "", script:to_json(), ""
end
</lua_script>

Here is a more complicated example with data processing. You can see how data is passed between functions. The main script returns "process_dna", which is the next function. The system will wait until we have input data to initiate the next function.

user: Quantify the amount of DNA from A1 of deck slot 7 (which is a nest_96_wellplate_100ul_pcr_full_skirt). Tell me if it has a low or high concentration (above 25 is high, below 25 is low).
assistant: <lua_script>
function main()
    local script_id = libB.uuid.generate()
    local data_id = libB.uuid.generate()

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
</lua_script>


`

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024 * 1024 * 16,
	WriteBufferSize: 1024 * 1024 * 16,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func (app *App) Close() error {
	app.cancel() // Cancel context to stop watchers
	if err := app.WDB.Close(); err != nil {
		return fmt.Errorf("failed to close write DB: %w", err)
	}
	if err := app.DB.Close(); err != nil {
		return fmt.Errorf("failed to close read DB: %w", err)
	}
	return nil
}

type App struct {
	ctx     context.Context
	cancel  context.CancelFunc
	Router  *http.ServeMux
	Logger  *slog.Logger
	DB      *sql.DB
	WDB     *WriteDB
	Runner  *ProtocolRunner
	Watcher *StepWatcher
}

// Modify InitializeApp to include runner and watcher setup
func InitializeApp(dbLocation string) *App {
	ctx, cancel := context.WithCancel(context.Background())
	var app App
	app.Router = http.NewServeMux()
	app.Logger = slog.Default()
	app.Router.HandleFunc("/", app.ProjectHandler)
	app.Router.HandleFunc("/chat/{projectID}", app.ChatPageHandler)
	app.Router.HandleFunc("/chat/{projectID}/ws", app.ChatHandler)
	app.Router.HandleFunc("/status/{projectID}", app.StatusHandler)
	app.Router.HandleFunc("/upload/{stepID}", app.UploadHandler)
	app.Router.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "upload.html")
	})

	// Initialize database
	writeDB, err := sql.Open("sqlite3", dbLocation)
	if err != nil {
		panic(err)
	}
	writeDB.SetMaxOpenConns(1)
	w := InitWriteDB(writeDB, context.Background())
	app.WDB = w
	readDB, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=ro", dbLocation))
	if err != nil {
		panic(err)
	}
	app.DB = readDB

	// Initialize protocol runner and watcher
	app.Runner = NewProtocolRunner(w)
	app.Watcher = NewStepWatcher(app.Runner)

	app.ctx = ctx
	app.cancel = cancel
	return &app
}

func (app *App) StatusHandler(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if projectID == "" {
		http.Error(w, "Project ID required", http.StatusBadRequest)
		return
	}

	queries := autodemosql.New(app.DB)
	steps, err := queries.GetLatestStepsForProject(r.Context(), projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(steps)
}

func (app *App) UploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	stepID := r.PathValue("stepID")
	if stepID == "" {
		http.Error(w, "Step ID required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(stepID, 10, 64)
	if err != nil {
		http.Error(w, "Invalid step ID", http.StatusBadRequest)
		return
	}

	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(r.Body)

	if err := app.WDB.CreateData(r.Context(), id, buf.String()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = app.Watcher.UpdateStep(id, buf.String())
	if err != nil {
		fmt.Println("failed to update step")
		return
	}

	w.WriteHeader(http.StatusOK)
}

//go:embed index.html
var indexHtml string

func (app *App) ChatPageHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	_, err := fmt.Fprint(w, indexHtml)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// ProjectHandler handles the initial project page load
func (app *App) ProjectHandler(w http.ResponseWriter, r *http.Request) {
	// If we're at root path, create new project
	if r.URL.Path == "/" {
		projectID := uuid.New().String()

		// Create new project in database
		writeDB := app.WDB
		err := writeDB.CreateProject(context.Background(), projectID)
		if err != nil {
			http.Error(w, "Failed to create project", http.StatusInternalServerError)
			return
		}

		// Redirect to project-specific URL
		fmt.Println(fmt.Sprintf("/chat/%s", projectID))
		http.Redirect(w, r, fmt.Sprintf("/chat/%s", projectID), http.StatusSeeOther)
		return
	}

	// Serve the chat HTML page
	http.ServeFile(w, r, "index.html")
}

func (app *App) ChatHandler(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	if projectID == "" {
		http.Error(w, "Project ID required", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	writeDB := app.WDB
	queries := autodemosql.New(app.DB)

	// Load initial history immediately after connection
	var id int64
	content, err := queries.GetLastMessageForProject(context.Background(), projectID)
	if err == nil && content.Content != "" {
		err = conn.WriteMessage(websocket.TextMessage, []byte(content.Content))
		if err != nil {
			log.Printf("Failed to write initial history: %v", err)
			return
		}
	}
	id = content.ID

	// Set up OpenAI client
	apiKey := os.Getenv("API_KEY")
	baseUrl := os.Getenv("BASE_URL")
	model := os.Getenv("MODEL")
	config := openai.DefaultConfig(apiKey)
	if baseUrl != "" {
		config.BaseURL = baseUrl
	}
	client := openai.NewClientWithConfig(config)

	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			return
		}
		msg := string(p)

		// Parse the context and get current conversation
		var messages []openai.ChatCompletionMessage
		if len(msg) > 14 && msg[0:15] == "<|begin_of_text" {
			messages = parseToMessages(msg)
		} else if strings.HasPrefix(msg, "<|execute|>") {
			msgWithoutExecute := strings.TrimPrefix(msg, "<|execute|>")
			messages = parseToMessages(msgWithoutExecute)
		} else if content.Content != "" {
			parsedMsgs := parseToMessages(content.Content)
			for _, m := range parsedMsgs {
				messages = append(messages, openai.ChatCompletionMessage{
					Role:    m.Role,
					Content: m.Content,
				})
			}
			messages = append(messages, openai.ChatCompletionMessage{
				Role:    "user",
				Content: msg,
			})
		} else {
			messages = []openai.ChatCompletionMessage{
				{
					Role:    "system",
					Content: LuaPrompt,
				},
				{
					Role:    "user",
					Content: msg,
				},
			}
		}

		// Send initial context
		contextMsg := constructConversationContext(messages)
		err = conn.WriteMessage(messageType, []byte(contextMsg))
		if err != nil {
			log.Printf("Failed to write context: %v", err)
			return
		}

		if strings.HasPrefix(msg, "<|execute|>") {
			// Execute command - only for lua_script
			lastMsg := messages[len(messages)-1].Content

			if strings.Contains(lastMsg, "<lua_script>") {
				output := app.executeLuaScript(r.Context(), id, lastMsg)
				fmt.Println(output)
				toolMsg := fmt.Sprintf("tool:\n%s", output)

				messages = append(messages, openai.ChatCompletionMessage{
					Role:    "assistant",
					Content: toolMsg,
				})

				err = conn.WriteMessage(messageType, []byte(toolMsg))
				if err != nil {
					log.Printf("Failed to write tool output: %v", err)
					return
				}
			}
		} else {
			// Normal message flow - stream LLM response
			for {
				// Stream the LLM response
				stream, err := client.CreateChatCompletionStream(
					context.Background(),
					openai.ChatCompletionRequest{
						Model:    model,
						Messages: messages,
						Stream:   true,
					},
				)
				if err != nil {
					log.Printf("ChatCompletionStream error: %v\n", err)
					return
				}

				var currentResponse strings.Builder
				// Collect the full response
				for {
					var response openai.ChatCompletionStreamResponse
					response, err = stream.Recv()
					if errors.Is(err, io.EOF) {
						break
					}
					if err != nil {
						log.Printf("Stream error: %v\n", err)
						stream.Close()
						return
					}

					if len(response.Choices) > 0 {
						token := response.Choices[0].Delta.Content
						_ = conn.WriteMessage(messageType, []byte(token))
						currentResponse.WriteString(token)
					}
				}

				stream.Close()

				// Add the response to messages and write it
				llmResponse := currentResponse.String()
				messages = append(messages, openai.ChatCompletionMessage{
					Role:    "assistant",
					Content: llmResponse,
				})

				// Check if we got a sandbox to execute
				if strings.Contains(llmResponse, "<lua_sandbox>") {
					output := app.executeLuaSandbox(llmResponse)
					toolOutput := fmt.Sprintf("tool:\n%s", output)
					toolMsg := fmt.Sprintf("\n<|eot_id|>\n<|start_header_id|>assistant<|end_header_id|>\n%s", toolOutput)

					// Add tool output to messages and send
					messages = append(messages, openai.ChatCompletionMessage{
						Role:    "assistant",
						Content: toolOutput,
					})

					_ = conn.WriteMessage(messageType, []byte(toolMsg))

					// Continue the loop to get another LLM response
					_ = conn.WriteMessage(messageType, []byte("\n<|eot_id|>\n<|start_header_id|>assistant<|end_header_id|>\n%s"))
					continue
				}

				// If no sandbox was found, we're done
				break
			}
		}

		// Add user header and save context
		userHeader := "\n<|eot_id|>\n<|start_header_id|>user<|end_header_id|>\n"
		err = conn.WriteMessage(messageType, []byte(userHeader))
		if err != nil {
			log.Printf("Failed to write user header: %v", err)
			return
		}

		fullContext := constructConversationContext(messages)
		id, _, err = writeDB.AddMessageHistory(r.Context(), projectID, fullContext)
		if err != nil {
			log.Printf("Failed to save message history: %v", err)
		}
	}
}

func (app *App) executeLuaSandbox(msg string) string {
	luaPrefix := "<lua_sandbox>"
	codeSuffix := "</lua_sandbox>"

	luaStartIndex := strings.LastIndex(msg, luaPrefix)
	if luaStartIndex == -1 {
		return "Error: Could not find lua_sandbox start tag"
	}

	remainingText := msg[luaStartIndex+len(luaPrefix):]
	luaEndIndex := strings.Index(remainingText, codeSuffix)
	if luaEndIndex == -1 {
		return "Error: Could not find lua_sandbox end tag"
	}

	luaCode := msg[luaStartIndex+len(luaPrefix) : luaStartIndex+len(luaPrefix)+luaEndIndex]
	output, err := libb.ExecuteLua(luaCode)
	if err != nil {
		return fmt.Sprintf("Got error: %s", err.Error())
	}

	return output
}

func (app *App) executeLuaScript(ctx context.Context, historyID int64, msg string) string {
	scriptPrefix := "<lua_script>"
	scriptSuffix := "</lua_script>"

	scriptStartIndex := strings.LastIndex(msg, scriptPrefix)
	if scriptStartIndex == -1 {
		return "Error: Could not find lua_script start tag"
	}

	remainingText := msg[scriptStartIndex+len(scriptPrefix):]
	scriptEndIndex := strings.Index(remainingText, scriptSuffix)
	if scriptEndIndex == -1 {
		return "Error: Could not find lua_script end tag"
	}

	scriptCode := msg[scriptStartIndex+len(scriptPrefix) : scriptStartIndex+len(scriptPrefix)+scriptEndIndex]

	fmt.Println(historyID)
	err := app.Runner.StartProtocol(ctx, historyID, scriptCode)
	if err != nil {
		return fmt.Sprintf("Failed to start protocol: %s", err.Error())
	}

	queries := autodemosql.New(app.DB)
	steps, err := queries.GetAllStepsForCodeFromProjectHistoryID(ctx, historyID)
	if err != nil {
		return fmt.Sprintf("Failed to get steps: %s", err.Error())
	}
	if len(steps) == 0 {
		return "No steps created for protocol"
	}

	initialStep := steps[len(steps)-1]
	app.Watcher.WatchStep(ctx, initialStep.ID)

	return fmt.Sprintf("Protocol started with step ID: %d\nStatus: %d\nComment: %s",
		initialStep.ID, initialStep.Status, initialStep.StepComment)
}

// constructConversationContext helper function to construct conversation context in the expected format
func constructConversationContext(messages []openai.ChatCompletionMessage) string {
	var result strings.Builder
	result.WriteString("<|begin_of_text|>")

	for i, msg := range messages {
		if i > 0 {
			result.WriteString("\n<|eot_id|>\n")
		}

		role := msg.Role
		// Map OpenAI roles back to our format
		switch role {
		case "system":
			result.WriteString("<|start_header_id|>system<|end_header_id|>\n")
		case "user":
			result.WriteString("<|start_header_id|>user<|end_header_id|>\n")
		case "assistant":
			result.WriteString("<|start_header_id|>assistant<|end_header_id|>\n")
		}

		result.WriteString(msg.Content)
	}

	result.WriteString("\n<|eot_id|>\n<|start_header_id|>assistant<|end_header_id|>\n")
	return result.String()
}

// parseToMessages parses the input string into Message structs
func parseToMessages(input string) []openai.ChatCompletionMessage {
	var messages []openai.ChatCompletionMessage

	// Split on message boundaries
	parts := strings.Split(input, "<|eot_id|>")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Find header
		headerStart := strings.Index(part, "<|start_header_id|>")
		headerEnd := strings.Index(part, "<|end_header_id|>")

		if headerStart == -1 || headerEnd == -1 {
			continue
		}

		// Extract role and content
		role := strings.TrimSpace(part[headerStart+len("<|start_header_id|>") : headerEnd])
		content := strings.TrimSpace(part[headerEnd+len("<|end_header_id|>"):])

		// Skip empty content
		if content == "" {
			continue
		}

		// Handle special cases
		switch role {
		case "system":
			// Remove begin_of_text marker if present
			content = strings.TrimPrefix(content, "<|begin_of_text|>")
		}

		content = strings.TrimSpace(content)

		// Only add message if we have both role and content
		if role != "" && content != "" {
			messages = append(messages, openai.ChatCompletionMessage{
				Role:    role,
				Content: content,
			})
		}
	}

	return messages
}

/******************************************************************************

Database/Schema.

The following section contains basic database functions, mainly for creating
databases for testing.

******************************************************************************/

//go:embed schema.sql
var Schema string

// CreateDatabase creates a new database.
func CreateDatabase(db *sql.DB) error {
	_, err := db.Exec(Schema)
	return err
}

// MakeTestDatabase creates a database for testing purposes.
func MakeTestDatabase(dbLocation string) (*sql.DB, *WriteDB) {
	readDB, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=ro", dbLocation))
	if err != nil {
		panic(err)
	}
	writeDB, err := sql.Open("sqlite3", dbLocation)
	writeDB.SetMaxOpenConns(1)
	if err != nil {
		panic(err)
	}
	err = CreateDatabase(writeDB)
	if err != nil {
		panic(err)
	}
	w := InitWriteDB(writeDB, context.Background())
	return readDB, w
}

/******************************************************************************

Write transactions

Here is the problem:
- SQLite doesn't like multiple writers.
- This multiple writers problems extends to the web service itself: If people
  are doing things on the website, it will naturally open multiple goroutines
  (because that is how go handles http servers)
- The "database/sql" interface is stateful, which means `db.Begin()` will
  create problems even though `SetMaxOpenConns(1)` - the stateful-ness means
  this won't be handled properly, with the only band-aid being busy timeout.

Basically, the way that the standard library sql database works bases on
assumptions that may not be true with SQLite! How do we solve that?

The basic idea is that we send all writes through a single channel that a
single goroutine is operating on. This application-level handling of
transactions means we never encounter a case where SQLite could error from
a busy row.


It is important that RunTx does not have any long requests: if it does, it
blocks ALL writes on the system.

******************************************************************************/

// WriteRequest is a request to run a function writing directly to the
// database. Only one WriteRequest is ever run at a time.
type WriteRequest struct {
	Func         func(db *sql.DB, ctx context.Context) error
	ResponseChan chan error
}

// WriteDB is a wrapper to run database writes sequentially.
type WriteDB struct {
	DB               *sql.DB
	CTX              context.Context
	WriteRequestChan chan WriteRequest
}

func (w *WriteDB) write(requestChan <-chan WriteRequest) error {
	for {
		select {
		case <-w.CTX.Done():
			return w.CTX.Err()

		case req, ok := <-requestChan:
			if !ok {
				return nil
			}
			req.ResponseChan <- req.Func(w.DB, w.CTX)
		}
	}
}

func InitWriteDB(db *sql.DB, ctx context.Context) *WriteDB {
	g, ctx := errgroup.WithContext(ctx)
	requestChan := make(chan WriteRequest) // explicitly NOT buffered.
	w := WriteDB{DB: db, CTX: ctx, WriteRequestChan: requestChan}
	// Slice of PRAGMA statements to execute
	pragmas := []string{
		"PRAGMA journal_mode = WAL;",
		"PRAGMA synchronous = NORMAL;",
		"PRAGMA cache_size = 20000;",
		"PRAGMA foreign_keys = ON;",
		"PRAGMA strict = ON;",
		"PRAGMA busy_timeout = 5000;",
	}
	// Execute each PRAGMA statement
	for _, pragma := range pragmas {
		_, err := db.Exec(pragma)
		if err != nil {
			_ = db.Close() // Attempt to close the database on error
			panic(fmt.Sprintf("error executing %s: %s", pragma, err))
		}
	}
	g.Go(func() error {
		return w.write(requestChan)
	})
	return &w
}

func (w *WriteDB) Close() error {
	return w.DB.Close()
}

// RunTx runs a transaction. It is vital that this is a short function without
// much compute: it blocks all write database operations for the entire
// service. If normal writes are being used, this shouldn't be a problem
// (SQLite can handle a lot of writes.)
//
// As per https://www.sqlite.org/faq.html#q19 , each insert is rather
// expensive! If possible, group together inserts into one big transaction
// like in writeDB.CreateFastq.
func (w *WriteDB) RunTx(fn func(db *sql.DB, ctx context.Context) error) error {
	responseChan := make(chan error, 1)
	req := WriteRequest{Func: fn, ResponseChan: responseChan}
	w.WriteRequestChan <- req
	err := <-responseChan
	close(req.ResponseChan)
	return err
}

// CreateProject inserts a new project into the database
func (w *WriteDB) CreateProject(ctx context.Context, projectID string) error {
	return w.RunTx(func(db *sql.DB, ctx context.Context) error {
		queries := autodemosql.New(db)
		err := queries.CreateProject(ctx, projectID)
		return err
	})
}

// AddMessageHistory adds a new message history entry and returns the created ID and timestamp
func (w *WriteDB) AddMessageHistory(ctx context.Context, projectID string, content string) (int64, int64, error) {
	var id, createdAt int64
	err := w.RunTx(func(db *sql.DB, ctx context.Context) error {
		queries := autodemosql.New(db)
		result, err := queries.AddMessageHistory(ctx, autodemosql.AddMessageHistoryParams{ProjectID: projectID, Content: content})
		if err != nil {
			return err
		}
		id = result.ID
		createdAt = result.CreatedAt
		return nil
	})
	return id, createdAt, err
}
func (w *WriteDB) CreateData(ctx context.Context, codeStepID int64, data string) error {
	err := w.RunTx(func(db *sql.DB, ctx context.Context) error {
		var err error
		queries := autodemosql.New(db)
		err = queries.UpdateStepData(ctx, autodemosql.UpdateStepDataParams{
			ID:   codeStepID,
			Data: sql.NullString{Valid: true, String: data},
		})
		return err
	})
	return err
}
