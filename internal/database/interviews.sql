CREATE TABLE IF NOT EXISTS interviews (
    id VARCHAR(255) PRIMARY KEY,
    position VARCHAR(255) NOT NULL,
    experience VARCHAR(255) NOT NULL,
    language VARCHAR(255) NOT NULL,
    voice_id VARCHAR(255) NOT NULL,
    speed INT NOT NULL,
    skills JSON NOT NULL,
    skip_code BOOLEAN NOT NULL DEFAULT TRUE,
    max_questions INT NOT NULL,
    remaining_questions INT NOT NULL,
    total_score JSON,
    positive_feedback TEXT,
    actionable_feedback TEXT,
    final_comment TEXT,
    status VARCHAR(255) NOT NULL DEFAULT "InProgress", -- 'pending', 'in progress', 'completed'
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);