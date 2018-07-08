CREATE OR REPLACE FUNCTION update_modified_column()	
RETURNS TRIGGER AS $$
BEGIN
    NEW.modified = now();
    RETURN NEW;	
END;
$$ language 'plpgsql';


CREATE TABLE buf.image (
  id VARCHAR PRIMARY KEY DEFAULT gen_random_uuid(),
  href VARCHAR,
  preview BYTEA,
  source BYTEA,
  size bigint,
  width int,
  height int,
  slow BOOLEAN NOT NULL DEFAULT FALSE,
  broken BOOLEAN NOT NULL DEFAULT FALSE,
  contentType VARCHAR,

  db_add_time TIMESTAMP NOT NULL DEFAULT now(),
  modified TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX buf_i_image_slow ON buf.image (slow);
CREATE INDEX buf_i_image_broken ON buf.image (broken);

CREATE TRIGGER update_image_modtime BEFORE UPDATE ON buf.image FOR EACH ROW EXECUTE PROCEDURE  update_modified_column();