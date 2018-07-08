ALTER TABLE buf.torrent ADD COLUMN tryed_fetch_torrent INT NOT NULL DEFAULT 0;
CREATE INDEX buf_i_torrent_tryed_fetch_torrent ON buf.torrent(tryed_fetch_torrent);
