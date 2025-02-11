package autodemo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/koeng101/autodemo/src/autodemosql"
	libb "github.com/koeng101/autodemo/src/libB"
)

// ProtocolRunner manages the execution of protocol steps in the database
type ProtocolRunner struct {
	db *WriteDB
}

func NewProtocolRunner(db *WriteDB) *ProtocolRunner {
	return &ProtocolRunner{
		db: db,
	}
}

type ProtocolState struct {
	Status          int
	Comments        string
	NextFunc        string
	Script          *libb.Script
	DataPassthrough string
}

// StartProtocol begins execution of a new protocol
func (r *ProtocolRunner) StartProtocol(ctx context.Context, messageHistoryID int64, code string) error {
	return r.db.RunTx(func(db *sql.DB, ctx context.Context) error {
		queries := autodemosql.New(db)

		codeID, err := queries.CreateCode(ctx, autodemosql.CreateCodeParams{
			ProjectMessageHistoryID: messageHistoryID,
			Code:                    code,
		})
		if err != nil {
			return fmt.Errorf("failed to create code entry: %v", err)
		}

		state, err := r.executeLuaStep(code, "main", "", "")
		if err != nil {
			return fmt.Errorf("failed to execute initial step: %v", err)
		}

		scriptJSONbytes, err := json.Marshal(state.Script)
		if err != nil {
			return fmt.Errorf("failed to marshal script")
		}

		_, err = queries.CreateCodeStep(ctx, autodemosql.CreateCodeStepParams{
			Code:            codeID,
			Status:          int64(state.Status),
			StepComment:     state.Comments,
			NextFunction:    state.NextFunc,
			Script:          string(scriptJSONbytes),
			DataPassthrough: state.DataPassthrough,
		})
		if err != nil {
			return fmt.Errorf("failed to create code step: %v", err)
		}

		return nil
	})
}

// executeStep executes a step and returns the new state - separated from transaction handling
func (r *ProtocolRunner) executeStep(db *sql.DB, ctx context.Context, stepID int64) (*libb.ProtocolState, error) {
	queries := autodemosql.New(db)

	step, err := queries.GetCodeStep(ctx, stepID)
	if err != nil {
		return nil, fmt.Errorf("failed to get code step: %v", err)
	}

	code, err := queries.GetCode(ctx, step.Code)
	if err != nil {
		return nil, fmt.Errorf("failed to get code: %v", err)
	}

	if !step.Data.Valid {
		return nil, fmt.Errorf("no data available for step")
	}

	return r.executeLuaStep(code.Code, step.NextFunction, step.DataPassthrough, step.Data.String)
}

func (r *ProtocolRunner) UpdateStepAndContinue(ctx context.Context, stepID int64, data string) error {
	return r.db.RunTx(func(db *sql.DB, ctx context.Context) error {
		queries := autodemosql.New(db)

		// Update the step with the new data
		err := queries.UpdateStepData(ctx, autodemosql.UpdateStepDataParams{
			ID:   stepID,
			Data: sql.NullString{String: data, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("failed to update step data: %v", err)
		}

		step, err := queries.GetCodeStep(ctx, stepID)
		if err != nil {
			return fmt.Errorf("failed to query step: %v", err)
		}

		// Execute the step within the same transaction
		state, err := r.executeStep(db, ctx, stepID)
		if err != nil {
			return err
		}

		scriptJSONbytes, err := json.Marshal(state.Script)
		if err != nil {
			return fmt.Errorf("failed to marshal script")
		}

		// Always create a new step when processing data
		_, err = queries.CreateCodeStep(ctx, autodemosql.CreateCodeStepParams{
			Code:            step.Code,
			Status:          int64(state.Status),
			StepComment:     state.Comments,
			NextFunction:    state.NextFunc,
			Script:          string(scriptJSONbytes),
			DataPassthrough: state.DataPassthrough,
		})
		if err != nil {
			return fmt.Errorf("failed to create new step: %v", err)
		}

		return nil
	})
}

func (r *ProtocolRunner) executeLuaStep(code string, funcName string, dataPassthrough string, dataString string) (*libb.ProtocolState, error) {
	fmt.Println(dataString)
	var data map[string]map[string]string
	if dataString != "" {
		err := json.Unmarshal([]byte(dataString), &data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse json: %v", err)
		}
	}
	fmt.Println(data)

	return libb.ExecuteLuaStep(code, funcName, dataPassthrough, data)
}

// StepWatcher remains unchanged
type StepWatcher struct {
	runner   *ProtocolRunner
	watchers map[int64]chan string
	mu       sync.RWMutex
}

func NewStepWatcher(runner *ProtocolRunner) *StepWatcher {
	return &StepWatcher{
		runner:   runner,
		watchers: make(map[int64]chan string),
	}
}

func (w *StepWatcher) WatchStep(ctx context.Context, stepID int64) {
	w.mu.Lock()
	dataChan := make(chan string, 1)
	w.watchers[stepID] = dataChan
	w.mu.Unlock()

	go func() {
		select {
		case data := <-dataChan:
			err := w.runner.UpdateStepAndContinue(ctx, stepID, data)
			if err != nil {
				fmt.Printf("Error executing step %d: %v\n", stepID, err)
			}
			w.mu.Lock()
			delete(w.watchers, stepID)
			close(dataChan)
			w.mu.Unlock()
		case <-ctx.Done():
			w.mu.Lock()
			delete(w.watchers, stepID)
			close(dataChan)
			w.mu.Unlock()
		}
	}()
}

func (w *StepWatcher) UpdateStep(stepID int64, data string) error {
	w.mu.RLock()
	dataChan, exists := w.watchers[stepID]
	w.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no watcher found for step %d", stepID)
	}

	select {
	case dataChan <- data:
		return nil
	default:
		return fmt.Errorf("watcher for step %d is not ready", stepID)
	}
}
