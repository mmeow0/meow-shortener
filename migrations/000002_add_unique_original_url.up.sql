-- Добавление уникального индекса на поле original_url
CREATE UNIQUE INDEX IF NOT EXISTS idx_urls_original_url ON urls(original_url);

