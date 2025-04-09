CREATE TABLE IF NOT EXISTS questions (
    interview_id VARCHAR(255) NOT NULL,
    question_index INT NOT NULL,
    content TEXT NOT NULL,
    audio MEDIUMTEXT,
    lipsync JSON,
    answer TEXT,
    record_proof MEDIUMTEXT,
    comment TEXT,
    score CHAR, -- 'A', 'B', 'C', 'D', 'F'
    status VARCHAR(50),    -- 'submitted', 'answered', 'not answered'
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (interview_id, question_index),
    FOREIGN KEY (interview_id) REFERENCES interviews(id) ON DELETE CASCADE
);