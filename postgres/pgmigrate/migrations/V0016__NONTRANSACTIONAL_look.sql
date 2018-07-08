
CREATE OR REPLACE FUNCTION make_tsvector(dic text, title TEXT, content TEXT)
  RETURNS tsvector AS $$
BEGIN
  RETURN (setweight(to_tsvector(dic::regconfig, title::text),'A') || setweight(to_tsvector(dic::regconfig, content::text), 'B'));
END
$$ LANGUAGE 'plpgsql' IMMUTABLE;


create index IF NOT EXISTS torrent_en_rum_i on buf.torrent using rum(make_tsvector('english_hunspell', title, content_text) rum_tsvector_ops);
create index IF NOT EXISTS torrent_ru_rum_i on buf.torrent using rum(make_tsvector('russian_hunspell', title, content_text) rum_tsvector_ops);
create index IF NOT EXISTS file_ru_rum_i    on buf.file using rum(make_tsvector('russian_hunspell', basename, path) rum_tsvector_ops);
create index IF NOT EXISTS file_en_rum_i    on buf.file using rum(make_tsvector('english_hunspell', basename, path) rum_tsvector_ops);


CREATE OR REPLACE FUNCTION look_torrent (dic text, query tsquery, qlimit INT) returns TABLE(
  torrent_id VARCHAR, 
  file_id  VARCHAR,
  rank REAL, 
  title TEXT, 
  fragments text
) AS $$ 
  SELECT
    id AS torrent_id,
    null AS file_id,
    make_tsvector(dic, title, content_text) <=> query AS rank,
    ts_headline(title, query) AS title,
    ts_headline(content_text, query, 'MaxWords = 5 ,MinWords = 2,MaxFragments = 5') AS fragments
  FROM buf.torrent
  WHERE    make_tsvector(dic, title, content_text) @@ query
  ORDER BY make_tsvector(dic, title, content_text) <=> query  LIMIT qlimit
$$ LANGUAGE SQL;


CREATE OR REPLACE FUNCTION look_file (dic text, query tsquery, qlimit INT) returns TABLE(
  torrent_id VARCHAR, 
  file_id  VARCHAR,
  rank REAL, 
  title TEXT, 
  fragments text
) AS $$ 
  SELECT
    null AS torrent_id,
    id AS file_id,
    make_tsvector(dic, basename, path) <=> query AS rank,
    ts_headline(basename, query) AS title,
    ts_headline(path, query, 'MaxWords = 5 ,MinWords = 2,MaxFragments = 5') AS fragments
  FROM buf.file
  WHERE    make_tsvector(dic, basename, path) @@ query
  ORDER BY make_tsvector(dic, basename, path) <=> query  LIMIT qlimit
$$ LANGUAGE SQL;

CREATE OR REPLACE FUNCTION look (query TEXT, qlimit INT) returns TABLE(
  torrent_id VARCHAR, 
  file_id  VARCHAR,
  rank REAL, 
  title TEXT, 
  fragments text
) AS $$ 
  SELECT * FROM look_torrent('english_hunspell', phraseto_tsquery('english_hunspell', query), qlimit) UNION
  SELECT * FROM look_torrent('russian_hunspell', phraseto_tsquery('russian_hunspell', query), qlimit) UNION
  SELECT * FROM look_file('russian_hunspell', phraseto_tsquery('russian_hunspell', query), qlimit) UNION
  SELECT * FROM look_file('english_hunspell', phraseto_tsquery('english_hunspell', query), qlimit)
  ORDER BY rank LIMIT qlimit
$$ LANGUAGE SQL;

VACUUM (ANALYZE);


CREATE OR REPLACE FUNCTION look_prefix (query TEXT, prefix TEXT, qlimit INT) returns TABLE(
  torrent_id VARCHAR, 
  file_id  VARCHAR,
  rank REAL, 
  title TEXT, 
  fragments text
) AS $$ 
  SELECT * FROM look_torrent('english_hunspell', phraseto_tsquery('english_hunspell', query) || to_tsquery('english_hunspell', prefix || ':*'), qlimit) UNION
  SELECT * FROM look_torrent('russian_hunspell', phraseto_tsquery('russian_hunspell', query) || to_tsquery('russian_hunspell', prefix || ':*'), qlimit) UNION
  SELECT * FROM look_file('russian_hunspell', phraseto_tsquery('russian_hunspell', query) || to_tsquery('russian_hunspell', prefix || ':*'), qlimit) UNION
  SELECT * FROM look_file('english_hunspell', phraseto_tsquery('english_hunspell', query) || to_tsquery('english_hunspell', prefix || ':*'), qlimit)
  ORDER BY rank LIMIT qlimit
$$ LANGUAGE SQL;


-- CREATE INDEX torrent_ru_trgm_i ON buf.torrent USING gin (title gin_trgm_ops);

-- CREATE TABLE words AS SELECT word FROM ts_stat('SELECT to_tsvector(''simple'', title || '' '' || content_text || '' '' || forum_title) FROM buf.torrent UNION SELECT to_tsvector(''simple'', path) FROM buf.file');

CREATE TABLE torrent_words AS SELECT * FROM ts_stat('(SELECT to_tsvector(''simple'', title || '' '' || coalesce(content_text,'''') || '' '' || coalesce(forum_title, '''')) FROM buf.torrent WHERE title is not null)') WHERE length(word) > 2 ORDER BY nentry DESC