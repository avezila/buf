
ALTER TABLE buf.file ADD COLUMN parse_tried INT NOT NULL DEFAULT 0;
CREATE INDEX buf_i_file_parse_tried ON buf.file(parse_tried);
ALTER TABLE buf.file ADD COLUMN parse_defered TIMESTAMP NOT NULL DEFAULT now();
CREATE INDEX buf_i_file_parse_defered ON buf.file(parse_defered);
