CREATE TABLE interviews (
    id VARCHAR(255) PRIMARY KEY,
    field VARCHAR(255) NOT NULL,
    position VARCHAR(255) NOT NULL,
    language VARCHAR(255) NOT NULL,
    voice_id VARCHAR(255) NOT NULL,
    speed INT NOT NULL,
    level VARCHAR(255) NOT NULL,
    coding BOOLEAN NOT NULL DEFAULT FALSE,
    max_questions INT NOT NULL,
    remaining_questions INT NOT NULL,
    total_score JSON,
    areas_of_improvement TEXT,
    final_comment TEXT,
    completed BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);


CREATE TABLE questions (
	interview_id VARCHAR(255) NOT NULL,
    question_index INT NOT NULL,
    content TEXT NOT NULL, 
    audio TEXT, -- Bytes64 encoded audio
    lipsync JSON, -- JSON data for lipsync
    answer TEXT,
    record_proof BLOB,
    comment TEXT,
    status VARCHAR(50), -- Status of the question ("full", "partial", "none", "answered", "inactive"))
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (interview_id, question_index),
    FOREIGN KEY (interview_id) REFERENCES interviews(id) ON DELETE CASCADE
);