-- name: CreateProject :exec
INSERT INTO project (id) VALUES (?);

-- name: GetProjectByID :one
SELECT * FROM project WHERE id = ?;

-- name: AddMessageHistory :one
INSERT INTO project_message_history (project_id, content) 
VALUES (?, ?) 
RETURNING id, created_at;

-- name: GetMessageHistoryByID :one
SELECT * FROM project_message_history WHERE id = ?;

-- name: GetLastMessageForProject :one
SELECT pmh.id, pmh.content, pmh.created_at
FROM project_message_history AS pmh
WHERE pmh.project_id = ?
ORDER BY pmh.created_at DESC
LIMIT 1;

-- name: CreateCode :one
INSERT INTO code(project_message_history_id, code) VALUES (?, ?) RETURNING id;

-- name: GetCode :one
SELECT * FROM code WHERE id = ?;

-- name: CreateCodeStep :one
INSERT INTO code_step(code, status, step_comment, next_function, script, data_passthrough) VALUES (?, ?, ?, ?, ?, ?) RETURNING id;

-- name: GetCodeStep :one
SELECT * FROM code_step WHERE id = ?;

-- name: GetAllStepsForCodeFromProjectHistoryID :many
SELECT cs.id, cs.code, cs.status, cs.step_comment, cs.next_function, cs.script, cs.data_passthrough, cs.data
FROM code_step AS cs
JOIN code AS c ON c.id = cs.code
WHERE c.project_message_history_id = ?;

-- name: GetDataForStep :one
SELECT data FROM code_step WHERE id = ?;

-- name: UpdateStepData :exec
UPDATE code_step SET data = ? WHERE id = ?;

-- name: UpdateStepStatus :exec
UPDATE code_step SET status = ? WHERE id = ?;

-- name: GetLatestStepsForProject :many
SELECT cs.* FROM code_step cs
JOIN code c ON cs.code = c.id
JOIN project_message_history pmh ON c.project_message_history_id = pmh.id
WHERE pmh.project_id = ? 
ORDER BY cs.id DESC;
