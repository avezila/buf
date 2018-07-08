
CREATE TABLE buf.file (
  id VARCHAR PRIMARY KEY DEFAULT gen_random_uuid(),
  torrent_id VARCHAR REFERENCES buf.torrent(id),
  index_in_torrent INT,
  ext VARCHAR,
  basename VARCHAR,
  "path" VARCHAR,
  size BIGINT,

  is_track BOOLEAN,
  parsed BOOLEAN NOT NULL DEFAULT false,

  db_add_time TIMESTAMP NOT NULL DEFAULT now(),
  modified TIMESTAMP  NOT NULL DEFAULT now(),

  preview VARCHAR REFERENCES buf.image(id),
  images VARCHAR[] DEFAULT ARRAY[]::VARCHAR[],

  log VARCHAR[] DEFAULT ARRAY[]::VARCHAR[],
  unique (torrent_id, index_in_torrent)
);

CREATE INDEX buf_i_file_torrent_id ON buf.file(torrent_id);
CREATE INDEX buf_i_file_index_in_torrent ON buf.file(index_in_torrent);
CREATE INDEX buf_i_file_ext ON buf.file(ext);
CREATE INDEX buf_i_file_basename ON buf.file(basename);
CREATE INDEX buf_i_file_size ON buf.file(size);
CREATE INDEX buf_i_file_is_track ON buf.file(is_track);
CREATE INDEX buf_i_file_parsed ON buf.file(parsed);


CREATE TRIGGER update_file_modtime BEFORE UPDATE ON buf.file FOR EACH ROW EXECUTE PROCEDURE  update_modified_column();

ALTER TABLE buf.parse_queue ADD COLUMN torrent_id VARCHAR REFERENCES buf.torrent(id);
CREATE INDEX buf_i_parse_queue_torrent_id ON buf.parse_queue(torrent_id);


ALTER TABLE buf.torrent ADD COLUMN no_files BOOLEAN NOT NULL DEFAULT false;
CREATE INDEX buf_i_torrent_no_files ON buf.torrent(no_files);