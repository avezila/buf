
CREATE TABLE buf.parse_queue (
  id VARCHAR PRIMARY KEY DEFAULT gen_random_uuid(),
  job_type VARCHAR NOT NULL,
  "priority" FLOAT4 NOT NULL DEFAULT 1.0,
  done BOOLEAN NOT NULL DEFAULT FALSE,
  done_time TIMESTAMP,
  defered TIMESTAMP NOT NULL DEFAULT now(),
  duration FLOAT4,
  add_time TIMESTAMP DEFAULT now(),
  modified TIMESTAMP DEFAULT now(),
  tryed int NOT NULL DEFAULT 0,
  log VARCHAR[] DEFAULT ARRAY[]::VARCHAR[],

  href VARCHAR,
  "offset" VARCHAR,
  tracker_name VARCHAR,
  ip VARCHAR
);

CREATE INDEX buf_i_parse_queue_job_type ON buf.parse_queue(job_type);
CREATE INDEX buf_i_parse_queue_priority ON buf.parse_queue(priority);


CREATE TRIGGER update_parse_queue_modtime BEFORE UPDATE ON buf.parse_queue FOR EACH ROW EXECUTE PROCEDURE  update_modified_column();
