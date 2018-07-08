ALTER TABLE buf.parse_queue ADD COLUMN tracker_torrent_id VARCHAR;
ALTER TABLE buf.parse_queue ADD COLUMN job_name VARCHAR;

CREATE INDEX buf_i_parse_queue_tracker_torrent_id ON buf.parse_queue(tracker_torrent_id);
CREATE INDEX buf_i_parse_queue_job_name ON buf.parse_queue(job_name);