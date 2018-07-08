
CREATE TABLE buf.torrent (
  id VARCHAR PRIMARY KEY DEFAULT gen_random_uuid(),
  magnet_uri VARCHAR,
  torrent_source_file BYTEA,
  tracker_name VARCHAR,
  tracker_torrent_id VARCHAR,

  register_time TIMESTAMP,
  size BIGINT,
  title VARCHAR,
  content_html VARCHAR,
  content_text VARCHAR,
  forum_id VARCHAR,
  forum_title VARCHAR,

  db_add_time TIMESTAMP NOT NULL DEFAULT now(),
  modified TIMESTAMP  NOT NULL DEFAULT now(),

  hard_magnet BOOLEAN,
  failed_magnet BOOLEAN,
  no_music BOOLEAN,
  no_peers BOOLEAN,

  preview VARCHAR REFERENCES buf.image(id),
  images VARCHAR[] DEFAULT ARRAY[]::VARCHAR[],

  creator_username VARCHAR,
  creator_id VARCHAR,
  seeders INT,
  leechers INT,
  replies_count INT,
  downloaded_count INT,
  last_reply TIMESTAMP,
  log VARCHAR[] DEFAULT ARRAY[]::VARCHAR[],

  unique (tracker_name, tracker_torrent_id)
);

CREATE INDEX buf_i_torrent_tracker_name ON buf.torrent(tracker_name);
CREATE INDEX buf_i_torrent_tracker_torrent_id ON buf.torrent(tracker_torrent_id);
CREATE INDEX buf_i_torrent_register_time ON buf.torrent(register_time);
CREATE INDEX buf_i_torrent_size ON buf.torrent(size);
CREATE INDEX buf_i_torrent_forum_id ON buf.torrent(forum_id);
CREATE INDEX buf_i_torrent_hard_magnet ON buf.torrent(hard_magnet);
CREATE INDEX buf_i_torrent_failed_magnet ON buf.torrent(failed_magnet);
CREATE INDEX buf_i_torrent_no_music ON buf.torrent(no_music);
CREATE INDEX buf_i_torrent_no_peers ON buf.torrent(no_peers);


CREATE TRIGGER update_torrent_modtime BEFORE UPDATE ON buf.torrent FOR EACH ROW EXECUTE PROCEDURE  update_modified_column();

ALTER TABLE buf.torrent ADD COLUMN tryed_fetch_page INT NOT NULL DEFAULT 0;
