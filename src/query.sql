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

-- name: AddProjectData :one
INSERT INTO project_data (project_id, data_name, function_name, data) 
VALUES (?, ?, ?, ?)
RETURNING id, created_at;

-- name: GetProjectDataByID :one
SELECT * FROM project_data WHERE id = ?;

-- name: GetLastMessageForProject :one
SELECT pmh.id, pmh.content, pmh.created_at
FROM project_message_history AS pmh
WHERE pmh.project_id = ?
ORDER BY pmh.created_at DESC
LIMIT 1;
