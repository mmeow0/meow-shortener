-- Откат создания таблицы urls
DROP INDEX IF EXISTS idx_urls_user_id;
DROP INDEX IF EXISTS idx_urls_short_id;
DROP TABLE IF EXISTS urls;

