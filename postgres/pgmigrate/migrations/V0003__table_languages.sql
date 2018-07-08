/* pgmigrate-encoding: utf-8 */

CREATE TABLE buf.language (
  lang varchar not null,
  lang_native_name varchar null,
  lang_english_name varchar null,

  primary key (lang)
);

INSERT INTO buf.language
  (lang, lang_native_name, lang_english_name) VALUES
  ('en_US', 'English', 'English'),
  ('ru_RU', 'Русский', 'Russian'),
  ('de_DE', 'Deutsch', 'German');
