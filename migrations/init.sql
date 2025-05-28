CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(255) UNIQUE NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL
);

CREATE TABLE IF NOT EXISTS files (
    id UUID PRIMARY KEY,
    owner_id INT REFERENCES users(id),
    name VARCHAR(255) NOT NULL,
    current_version INT DEFAULT 1,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS file_versions (
    id SERIAL PRIMARY KEY,
    file_id UUID REFERENCES files(id),
    version_number INT NOT NULL,
    storage_key VARCHAR(255) NOT NULL,
    size BIGINT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(file_id, version_number)
);

CREATE TABLE IF NOT EXISTS file_permissions (
    file_id UUID REFERENCES files(id),
    user_id INT REFERENCES users(id),
    permission INT DEFAULT 0,
    PRIMARY KEY (file_id, user_id)
); 