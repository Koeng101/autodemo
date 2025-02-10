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
	content TEXT NOT NULL, -- the full message history, with special tokens instead of separate chat messages.
) STRICT;

-- project_data contains data from the lab that can be used from the code
CREATE TABLE project_data (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id TEXT NOT NULL REFERENCES project(id),
	created_at INTEGER NOT NULL DEFAULT (unixepoch()),
	data_name TEXT NOT NULL, -- name injected into lua table
	function_name TEXT NOT NULL, -- name of the function that will trigger after this executes
	data TEXT NOT NULL -- json data
) STRICT;
