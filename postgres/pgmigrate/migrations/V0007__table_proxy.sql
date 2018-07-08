
CREATE TABLE buf.proxy (
  ip VARCHAR PRIMARY KEY,

  delay FLOAT4,
  qps FLOAT4,
  total_requests bigint,
  failed_requests bigint,

  http    BOOLEAN,
  https   BOOLEAN,
  socks4  BOOLEAN,
  socks5  BOOLEAN,
  
  broken BOOLEAN,
  domain_rutracker BOOLEAN,
  domain_nnmclub BOOLEAN,

  db_add_time TIMESTAMP NOT NULL DEFAULT now(),
  db_check_time TIMESTAMP NOT NULL DEFAULT now(),
  modified TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX buf_i_proxy_domain_rutracker ON buf.proxy(domain_rutracker);
CREATE INDEX buf_i_proxy_domain_nnmclub   ON buf.proxy(domain_nnmclub);



CREATE TRIGGER update_proxy_modtime BEFORE UPDATE ON buf.proxy FOR EACH ROW EXECUTE PROCEDURE  update_modified_column();
