-- Создание таблицы для хранения URL
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

-- Комментарии к таблице и колонкам
COMMENT ON TABLE urls IS 'Таблица для хранения сокращённых URL';
COMMENT ON COLUMN urls.id IS 'Уникальный идентификатор записи';
COMMENT ON COLUMN urls.short_id IS 'Короткий идентификатор URL';
COMMENT ON COLUMN urls.original_url IS 'Оригинальный URL';
COMMENT ON COLUMN urls.user_id IS 'Идентификатор пользователя';
COMMENT ON COLUMN urls.created_at IS 'Дата и время создания записи';
COMMENT ON COLUMN urls.updated_at IS 'Дата и время последнего обновления записи';

