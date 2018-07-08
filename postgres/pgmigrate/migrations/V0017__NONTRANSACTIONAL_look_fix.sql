
drop index IF EXISTS torrent_en_rum_i;
drop index IF EXISTS torrent_ru_rum_i;
drop index IF EXISTS file_ru_rum_i;
drop index IF EXISTS file_en_rum_i;

CREATE OR REPLACE FUNCTION make_tsvector(a TEXT, b TEXT)
  RETURNS tsvector AS $$
BEGIN
  RETURN (setweight(to_tsvector(a),'A') || setweight(to_tsvector(b), 'B'));
END
$$ LANGUAGE 'plpgsql' IMMUTABLE;

create index IF NOT EXISTS torrent_rum_i on buf.torrent using rum(make_tsvector(title,    content_text) rum_tsvector_ops);
create index IF NOT EXISTS file_rum_i    on buf.file    using rum(make_tsvector(basename, path        ) rum_tsvector_ops);


CREATE OR REPLACE FUNCTION look_torrent (query tsquery, qlimit INT) returns TABLE(
  torrent_id VARCHAR, 
  file_id  VARCHAR,
  rank REAL, 
  title TEXT, 
  fragments text
) AS $$ 
  SELECT
    id AS torrent_id,
    null AS file_id,
    make_tsvector(title, content_text) <=> query AS rank,
    ts_headline(title, query) AS title,
    ts_headline(content_text, query, 'MaxWords = 5 ,MinWords = 2,MaxFragments = 5') AS fragments
  FROM buf.torrent
  WHERE    make_tsvector(title, content_text) @@ query
  ORDER BY make_tsvector(title, content_text) <=> query  LIMIT qlimit
$$ LANGUAGE SQL;


CREATE OR REPLACE FUNCTION look_file (query tsquery, qlimit INT) returns TABLE(
  torrent_id VARCHAR, 
  file_id  VARCHAR,
  rank REAL, 
  title TEXT, 
  fragments text
) AS $$ 
  SELECT
    null AS torrent_id,
    id AS file_id,
    make_tsvector(basename, path) <=> query AS rank,
    ts_headline(basename, query) AS title,
    ts_headline(path, query, 'MaxWords = 5 ,MinWords = 2,MaxFragments = 5') AS fragments
  FROM buf.file
  WHERE    make_tsvector(basename, path) @@ query
  ORDER BY make_tsvector(basename, path) <=> query  LIMIT qlimit
$$ LANGUAGE SQL;

CREATE OR REPLACE FUNCTION look (query TEXT, qlimit INT) returns TABLE(
  torrent_id VARCHAR, 
  file_id  VARCHAR,
  rank REAL, 
  title TEXT, 
  fragments text
) AS $$ 
  SELECT * FROM look_torrent(phraseto_tsquery(query), qlimit) UNION
  SELECT * FROM look_file(phraseto_tsquery(query), qlimit)
  ORDER BY rank LIMIT qlimit
$$ LANGUAGE SQL;

CREATE OR REPLACE FUNCTION lookp (query TEXT, qlimit INT) returns TABLE(
  torrent_id VARCHAR, 
  file_id  VARCHAR,
  rank REAL, 
  title TEXT, 
  fragments text
) AS $$ 
  SELECT * FROM look_torrent(plainto_tsquery(query), qlimit) UNION
  SELECT * FROM look_file(plainto_tsquery(query), qlimit)
  ORDER BY rank LIMIT qlimit
$$ LANGUAGE SQL;

CREATE OR REPLACE FUNCTION look_prefix (query TEXT, prefix TEXT, qlimit INT) returns TABLE(
  torrent_id VARCHAR, 
  file_id  VARCHAR,
  rank REAL, 
  title TEXT, 
  fragments text
) AS $$ 
  SELECT * FROM look_torrent(phraseto_tsquery(query) || to_tsquery(prefix || ':*'), qlimit) UNION
  SELECT * FROM look_file(phraseto_tsquery(query) || to_tsquery(prefix || ':*'), qlimit)
  ORDER BY rank LIMIT qlimit
$$ LANGUAGE SQL;

CREATE TABLE file_words AS SELECT * FROM ts_stat('(SELECT to_tsvector(''simple'', path) FROM buf.file WHERE path is not null)') WHERE length(word) > 2 ORDER BY nentry DESC;
CREATE TABLE words AS SELECT word, sum(ndoc) AS ndoc, sum(nentry) AS nentry FROM (SELECT * FROM torrent_words UNION ALL SELECT * FROM file_words) AS t GROUP BY word;
CREATE INDEX words_i ON words USING GIN (word gin_trgm_ops);
-- 

CREATE OR REPLACE FUNCTION suggest (query TEXT) returns TABLE(
  word text,
  similarity  real,
  nentry bigint
) AS $$ 
  select word, similarity(word,query), nentry  from words
  where 
    word % query and 
    abs(length(word)-length(query))<2
  order by nentry desc limit 10
$$ LANGUAGE SQL;