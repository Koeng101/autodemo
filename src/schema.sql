PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL; -- https://news.ycombinator.com/item?id=34247738
PRAGMA cache_size = 20000; -- https://news.ycombinator.com/item?id=34247738
PRAGMA foreign_keys = ON;
PRAGMA strict = ON;
PRAGMA busy_timeout = 5000;
PRAGMA auto_vacuum = INCREMENTAL;

-- project is a single thread
CREATE TABLE project (
	id TEXT PRIMARY KEY, -- uuid
	created_at INTEGER NOT NULL DEFAULT (unixepoch())
) STRICT;

-- project_message_history is the history of messages
CREATE TABLE project_message_history (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id TEXT NOT NULL REFERENCES project(id),
	created_at INTEGER NOT NULL DEFAULT (unixepoch()),
	content TEXT NOT NULL -- the full message history, with special tokens instead of separate chat messages.
) STRICT;

-- code always initiates at main
CREATE TABLE code (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_message_history_id INTEGER NOT NULL REFERENCES project_message_history(id), -- the thread the code was created from
	code TEXT NOT NULL, -- lua code
	complete INTEGER NOT NULL DEFAULT FALSE -- bool
) STRICT;

CREATE TABLE code_step (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	code INTEGER NOT NULL REFERENCES code(id),
	status INTEGER NOT NULL, -- 0: success, 1: failure, 2: continuation (requires data), 3: continuation (no data)
	step_comment TEXT NOT NULL, -- comment for the user
	next_function TEXT NOT NULL,  -- next function to run
	script TEXT NOT NULL, -- the script to execute, JSON
	data_passthrough TEXT NOT NULL, -- passthrough data from lua
	data TEXT -- the data to insert into this function, JSON
) STRICT;
