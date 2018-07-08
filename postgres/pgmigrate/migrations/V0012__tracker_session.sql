
CREATE TABLE buf.tracker_session (
  id VARCHAR PRIMARY KEY DEFAULT gen_random_uuid(),
  session VARCHAR,
  tracker_name VARCHAR,

  db_add_time TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX buf_i_tracker_session_tracker_name ON buf.tracker_session(tracker_name);
CREATE INDEX buf_i_tracker_session_session ON buf.tracker_session(session);
