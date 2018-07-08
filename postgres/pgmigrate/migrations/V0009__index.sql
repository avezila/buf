
-- CREATE INDEX buf_i_torrent_content_html ON buf.torrent(content_html);
-- CREATE INDEX buf_i_torrent_content_text ON buf.torrent(content_text);
CREATE INDEX buf_i_torrent_tryed_fetch_page ON buf.torrent(tryed_fetch_page);
CREATE INDEX buf_i_torrent_seeders ON buf.torrent(seeders);
CREATE INDEX buf_i_torrent_leechers ON buf.torrent(leechers);

CREATE INDEX buf_i_parse_queue_href ON buf.parse_queue(href);
CREATE INDEX buf_i_parse_queue_done ON buf.parse_queue(done);
CREATE INDEX buf_i_parse_queue_tryed ON buf.parse_queue(tryed);
CREATE INDEX buf_i_parse_queue_defered ON buf.parse_queue(defered);
