-- Создание таблицы для хранения сокращённых URL
CREATE TABLE IF NOT EXISTS urls (
    id SERIAL PRIMARY KEY,
    short_id VARCHAR(255) UNIQUE NOT NULL,
    original_url TEXT NOT NULL,
    user_id VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Индекс для быстрого поиска по короткому ID
CREATE INDEX IF NOT EXISTS idx_urls_short_id ON urls(short_id);

-- Индекс для поиска URL по user_id
CREATE INDEX IF NOT EXISTS idx_urls_user_id ON urls(user_id);

