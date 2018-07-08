
CREATE TABLE buf.url_parser (
  url VARCHAR PRIMARY KEY,
  tryed INT DEFAULT 0,
  priority FLOAT NOT NULL DEFAULT 1.0,
  parsed TIMESTAMP DEFAULT NULL,
  add_time TIMESTAMP NOT NULL DEFAULT now(),
  modified TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TRIGGER update_url_parser_modtime BEFORE UPDATE ON buf.url_parser FOR EACH ROW EXECUTE PROCEDURE  update_modified_column();
