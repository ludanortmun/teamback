CREATE TABLE users (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    role TEXT NOT NULL CHECK(role IN ('teacher', 'student')),
    google_sub TEXT UNIQUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
