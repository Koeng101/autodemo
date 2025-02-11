// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.28.0
// source: query.sql

package autodemosql

import (
	"context"
	"database/sql"
)

const addMessageHistory = `-- name: AddMessageHistory :one
INSERT INTO project_message_history (project_id, content) 
VALUES (?, ?) 
RETURNING id, created_at
`

type AddMessageHistoryParams struct {
	ProjectID string
	Content   string
}

type AddMessageHistoryRow struct {
	ID        int64
	CreatedAt int64
}

func (q *Queries) AddMessageHistory(ctx context.Context, arg AddMessageHistoryParams) (AddMessageHistoryRow, error) {
	row := q.db.QueryRowContext(ctx, addMessageHistory, arg.ProjectID, arg.Content)
	var i AddMessageHistoryRow
	err := row.Scan(&i.ID, &i.CreatedAt)
	return i, err
}

const createCode = `-- name: CreateCode :one
INSERT INTO code(project_message_history_id, code) VALUES (?, ?) RETURNING id
`

type CreateCodeParams struct {
	ProjectMessageHistoryID int64
	Code                    string
}

func (q *Queries) CreateCode(ctx context.Context, arg CreateCodeParams) (int64, error) {
	row := q.db.QueryRowContext(ctx, createCode, arg.ProjectMessageHistoryID, arg.Code)
	var id int64
	err := row.Scan(&id)
	return id, err
}

const createCodeStep = `-- name: CreateCodeStep :one
INSERT INTO code_step(code, status, step_comment, next_function, script, data_passthrough) VALUES (?, ?, ?, ?, ?, ?) RETURNING id
`

type CreateCodeStepParams struct {
	Code            int64
	Status          int64
	StepComment     string
	NextFunction    string
	Script          string
	DataPassthrough string
}

func (q *Queries) CreateCodeStep(ctx context.Context, arg CreateCodeStepParams) (int64, error) {
	row := q.db.QueryRowContext(ctx, createCodeStep,
		arg.Code,
		arg.Status,
		arg.StepComment,
		arg.NextFunction,
		arg.Script,
		arg.DataPassthrough,
	)
	var id int64
	err := row.Scan(&id)
	return id, err
}

const createProject = `-- name: CreateProject :exec
INSERT INTO project (id) VALUES (?)
`

func (q *Queries) CreateProject(ctx context.Context, id string) error {
	_, err := q.db.ExecContext(ctx, createProject, id)
	return err
}

const getAllStepsForCode = `-- name: GetAllStepsForCode :many
SELECT id, code, status, step_comment, next_function, script, data_passthrough, data FROM code_step WHERE code = ?
`

func (q *Queries) GetAllStepsForCode(ctx context.Context, code int64) ([]CodeStep, error) {
	rows, err := q.db.QueryContext(ctx, getAllStepsForCode, code)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []CodeStep
	for rows.Next() {
		var i CodeStep
		if err := rows.Scan(
			&i.ID,
			&i.Code,
			&i.Status,
			&i.StepComment,
			&i.NextFunction,
			&i.Script,
			&i.DataPassthrough,
			&i.Data,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getCode = `-- name: GetCode :one
SELECT id, project_message_history_id, code, complete FROM code WHERE id = ?
`

func (q *Queries) GetCode(ctx context.Context, id int64) (Code, error) {
	row := q.db.QueryRowContext(ctx, getCode, id)
	var i Code
	err := row.Scan(
		&i.ID,
		&i.ProjectMessageHistoryID,
		&i.Code,
		&i.Complete,
	)
	return i, err
}

const getCodeStep = `-- name: GetCodeStep :one
SELECT id, code, status, step_comment, next_function, script, data_passthrough, data FROM code_step WHERE id = ?
`

func (q *Queries) GetCodeStep(ctx context.Context, id int64) (CodeStep, error) {
	row := q.db.QueryRowContext(ctx, getCodeStep, id)
	var i CodeStep
	err := row.Scan(
		&i.ID,
		&i.Code,
		&i.Status,
		&i.StepComment,
		&i.NextFunction,
		&i.Script,
		&i.DataPassthrough,
		&i.Data,
	)
	return i, err
}

const getDataForStep = `-- name: GetDataForStep :one
SELECT data FROM code_step WHERE id = ?
`

func (q *Queries) GetDataForStep(ctx context.Context, id int64) (sql.NullString, error) {
	row := q.db.QueryRowContext(ctx, getDataForStep, id)
	var data sql.NullString
	err := row.Scan(&data)
	return data, err
}

const getLastMessageForProject = `-- name: GetLastMessageForProject :one
SELECT pmh.id, pmh.content, pmh.created_at
FROM project_message_history AS pmh
WHERE pmh.project_id = ?
ORDER BY pmh.created_at DESC
LIMIT 1
`

type GetLastMessageForProjectRow struct {
	ID        int64
	Content   string
	CreatedAt int64
}

func (q *Queries) GetLastMessageForProject(ctx context.Context, projectID string) (GetLastMessageForProjectRow, error) {
	row := q.db.QueryRowContext(ctx, getLastMessageForProject, projectID)
	var i GetLastMessageForProjectRow
	err := row.Scan(&i.ID, &i.Content, &i.CreatedAt)
	return i, err
}

const getMessageHistoryByID = `-- name: GetMessageHistoryByID :one
SELECT id, project_id, created_at, content FROM project_message_history WHERE id = ?
`

func (q *Queries) GetMessageHistoryByID(ctx context.Context, id int64) (ProjectMessageHistory, error) {
	row := q.db.QueryRowContext(ctx, getMessageHistoryByID, id)
	var i ProjectMessageHistory
	err := row.Scan(
		&i.ID,
		&i.ProjectID,
		&i.CreatedAt,
		&i.Content,
	)
	return i, err
}

const getProjectByID = `-- name: GetProjectByID :one
SELECT id, created_at FROM project WHERE id = ?
`

func (q *Queries) GetProjectByID(ctx context.Context, id string) (Project, error) {
	row := q.db.QueryRowContext(ctx, getProjectByID, id)
	var i Project
	err := row.Scan(&i.ID, &i.CreatedAt)
	return i, err
}

const updateStepData = `-- name: UpdateStepData :exec
UPDATE code_step SET data = ? WHERE id = ?
`

type UpdateStepDataParams struct {
	Data sql.NullString
	ID   int64
}

func (q *Queries) UpdateStepData(ctx context.Context, arg UpdateStepDataParams) error {
	_, err := q.db.ExecContext(ctx, updateStepData, arg.Data, arg.ID)
	return err
}

const updateStepStatus = `-- name: UpdateStepStatus :exec
UPDATE code_step SET status = ? WHERE id = ?
`

type UpdateStepStatusParams struct {
	Status int64
	ID     int64
}

func (q *Queries) UpdateStepStatus(ctx context.Context, arg UpdateStepStatusParams) error {
	_, err := q.db.ExecContext(ctx, updateStepStatus, arg.Status, arg.ID)
	return err
}
