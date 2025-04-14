package repo

import (
    "database/sql"
    "encoding/json"
    "fmt"

    pb "irelia/api"
)

type SQLInterviewRepository struct {
    db *sql.DB
}

func NewSQLInterviewRepository(db *sql.DB) *SQLInterviewRepository {
    return &SQLInterviewRepository{db: db}
}

// SaveInterview saves or updates an interview
func (r *SQLInterviewRepository) SaveInterview(interview *pb.Interview) error {
    skillsJSON, err := json.Marshal(interview.Skills)
    if err != nil {
        return fmt.Errorf("failed to marshal skills: %v", err)
    }

    totalScoreJSON, err := json.Marshal(interview.TotalScore)
    if err != nil {
        return fmt.Errorf("failed to marshal total score: %v", err)
    }

    query := `
        INSERT INTO interviews (
            id, position, experience, language, voice_id, speed, skills, skip_code, max_questions,
            remaining_questions, total_score, positive_feedback, actionable_feedback,
            final_comment, status
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON DUPLICATE KEY UPDATE
            position = VALUES(position),
            experience = VALUES(experience),
            language = VALUES(language),
            voice_id = VALUES(voice_id),
            speed = VALUES(speed),
            skills = VALUES(skills),
            skip_code = VALUES(skip_code),
            max_questions = VALUES(max_questions),
            remaining_questions = VALUES(remaining_questions),
            total_score = VALUES(total_score),
            positive_feedback = VALUES(positive_feedback),
            actionable_feedback = VALUES(actionable_feedback),
            final_comment = VALUES(final_comment),
            status = VALUES(status),
            updated_at = CURRENT_TIMESTAMP
    `

    _, err = r.db.Exec(query,
        interview.Id, interview.Position, interview.Experience, interview.Language, interview.VoiceId,
        interview.Speed, skillsJSON, interview.SkipCode, interview.MaxQuestions, interview.RemainingQuestions,
        totalScoreJSON, interview.PositiveFeedback, interview.ActionableFeedback,
        interview.FinalComment, interview.Status,
    )

    if err != nil {
        return fmt.Errorf("failed to save interview: %v", err)
    }

    return nil
}

// GetInterview retrieves an interview by ID
func (r *SQLInterviewRepository) GetInterview(id string) (*pb.Interview, error) {
    query := `
        SELECT id, position, experience, language, voice_id, speed, skills, max_questions,
               remaining_questions, total_score, positive_feedback, actionable_feedback,
               final_comment, status, created_at, updated_at
        FROM interviews
        WHERE id = ?
    `
    row := r.db.QueryRow(query, id)

    var interview pb.Interview
    var skillsJSON []byte
    var totalScoreJSON []byte
    err := row.Scan(
        &interview.Id, &interview.Position, &interview.Experience, &interview.Language,
        &interview.VoiceId, &interview.Speed, &skillsJSON, &interview.MaxQuestions,
        &interview.RemainingQuestions, &totalScoreJSON,
        &interview.PositiveFeedback, &interview.ActionableFeedback, &interview.FinalComment,
        &interview.Status, &interview.CreatedAt, &interview.UpdatedAt,
    )
    if err == sql.ErrNoRows {
        return nil, sql.ErrNoRows
    } else if err != nil {
        return nil, fmt.Errorf("failed to get interview: %v", err)
    }

    if err := json.Unmarshal(skillsJSON, &interview.Skills); err != nil {
        return nil, fmt.Errorf("failed to unmarshal skills: %v", err)
    }
    if err := json.Unmarshal(totalScoreJSON, &interview.TotalScore); err != nil {
        return nil, fmt.Errorf("failed to unmarshal total score: %v", err)
    }

    return &interview, nil
}

// SaveQuestion saves a new question in the database
func (r *SQLInterviewRepository) SaveQuestion(question *pb.Question) error {
    lipsyncJSON, err := json.Marshal(question.Lipsync)
    if err != nil {
        return fmt.Errorf("failed to marshal lipsync data: %v", err)
    }

    query := `
        INSERT INTO questions (interview_id, question_index, content, audio, lipsync, answer, record_proof, comment, score, status)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON DUPLICATE KEY UPDATE
            content = VALUES(content),
            audio = VALUES(audio),
            lipsync = VALUES(lipsync),
            answer = VALUES(answer),
            record_proof = VALUES(record_proof),
            comment = VALUES(comment),
            score = VALUES(score),
            status = VALUES(status)
    `
    _, err = r.db.Exec(query, question.InterviewId, question.Index, question.Content, question.Audio, lipsyncJSON, question.Answer, question.RecordProof, question.Comment, question.Score, question.Status)
    if err != nil {
        return fmt.Errorf("failed to save question: %v", err)
    }

    return nil
}

// GetQuestion retrieves a question by interview ID and question index
func (r *SQLInterviewRepository) GetQuestion(interviewID string, questionIndex int32) (*pb.Question, error) {
    query := `
        SELECT question_index, interview_id, content, audio, lipsync, answer, record_proof, comment, score, status, created_at, updated_at
        FROM questions
        WHERE interview_id = ? AND question_index = ?
    `
    row := r.db.QueryRow(query, interviewID, questionIndex)

    var question pb.Question
    var lipsyncJSON []byte
    err := row.Scan(
        &question.Index, &question.InterviewId, &question.Content, &question.Audio,
        &lipsyncJSON, &question.Answer, &question.RecordProof, &question.Comment,
        &question.Score, &question.Status, &question.CreatedAt, &question.UpdatedAt,
    )
    if err == sql.ErrNoRows {
        return nil, fmt.Errorf("question with index %d not found for interview ID %s", questionIndex, interviewID)
    } else if err != nil {
        return nil, fmt.Errorf("failed to get question: %v", err)
    }

    if err := json.Unmarshal(lipsyncJSON, &question.Lipsync); err != nil {
        return nil, fmt.Errorf("failed to unmarshal lipsync data: %v", err)
    }

    return &question, nil
}

// GetInterviewHistory retrieves a paginated list of interview summaries
func (r *SQLInterviewRepository) GetInterviewHistory(offset, limit int32) ([]*pb.InterviewSummary, error) {
    query := `
        SELECT id, position, experience, total_score, created_at
        FROM interviews
        ORDER BY created_at DESC
        LIMIT ? OFFSET ?
    `
    rows, err := r.db.Query(query, limit, offset)
    if err != nil {
        return nil, fmt.Errorf("failed to retrieve interview history: %v", err)
    }
    defer rows.Close()

    history := make([]*pb.InterviewSummary, 0)
    for rows.Next() {
        var summary pb.InterviewSummary
        var totalScoreJSON []byte
        err := rows.Scan(
            &summary.InterviewId, &summary.Position, &summary.Experience,
            &totalScoreJSON, &summary.CreatedAt,
        )
        if err != nil {
            return nil, fmt.Errorf("failed to scan interview summary: %v", err)
        }

        // Unmarshal the total score JSON into the TotalScore position
        if err := json.Unmarshal(totalScoreJSON, &summary.TotalScore); err != nil {
            return nil, fmt.Errorf("failed to unmarshal total score: %v", err)
        }

        history = append(history, &summary)
    }

    return history, nil
}

// GetTotalInterviewCount retrieves the total number of interviews
func (r *SQLInterviewRepository) GetTotalInterviewCount() (int32, error) {
    query := `SELECT COUNT(*) FROM interviews`
    var count int32
    err := r.db.QueryRow(query).Scan(&count)
    if err != nil {
        return 0, fmt.Errorf("failed to retrieve total interview count: %v", err)
    }
    return count, nil
}

// SaveAnswer saves an answer in the database
func (r *SQLInterviewRepository) SaveAnswer(interviewID string, answer *pb.AnswerResult) error {
    query := `
        UPDATE questions
        SET answer = ?, record_proof = ?, comment = ?, score = ?
        WHERE interview_id = ? AND question_index = ?
    `
    _, err := r.db.Exec(query, answer.Answer, answer.RecordProof, answer.Comment, answer.Score, interviewID, answer.Index)
    if err != nil {
        return fmt.Errorf("failed to save answer: %v", err)
    }
    return nil
}

// GetQaPair retrieves a question and its corresponding answer
func (r *SQLInterviewRepository) GetQaPair(interviewID string) ([]*pb.QaPair, error) {
    query := `
        SELECT content, answer
        FROM questions
        WHERE interview_id = ?
    `
    rows, err := r.db.Query(query, interviewID)
    if err != nil {
        return nil, fmt.Errorf("failed to retrieve submissions: %v", err)
    }
    defer rows.Close()

    qaPairs := make([]*pb.QaPair, 0)
    for rows.Next() {
        var answer pb.QaPair
        err := rows.Scan(&answer.Question, &answer.Answer)
        if err != nil {
            return nil, fmt.Errorf("failed to scan answer: %v", err)
        }
        qaPairs = append(qaPairs, &answer)
    }
    return qaPairs, nil
}

// GetAnswers retrieves all answers for an interview
func (r *SQLInterviewRepository) GetAnswers(interviewID string) ([]*pb.AnswerResult, error) {
    query := `
        SELECT question_index, content, answer, record_proof, comment, score, status
        FROM questions
        WHERE interview_id = ?
        ORDER BY question_index
    `
    rows, err := r.db.Query(query, interviewID)
    if err != nil {
        return nil, fmt.Errorf("failed to retrieve answers: %v", err)
    }
    defer rows.Close()

    answers := make([]*pb.AnswerResult, 0)
    for rows.Next() {
        var answer pb.AnswerResult
        err := rows.Scan(&answer.Index, &answer.Content, &answer.Answer, &answer.RecordProof, &answer.Comment, &answer.Score)
        if err != nil {
            return nil, fmt.Errorf("failed to scan answer: %v", err)
        }
        answers = append(answers, &answer)
    }

    return answers, nil
}

// QuestionExists checks if a question exists in the database
func (r *SQLInterviewRepository) QuestionExists(interviewID string, questionIndex int32) (bool, error) {
    query := `
        SELECT EXISTS(
            SELECT 1
            FROM questions
            WHERE interview_id = ? AND question_index = ?
        )
    `
    var exists bool
    err := r.db.QueryRow(query, interviewID, questionIndex).Scan(&exists)
    if err != nil {
        return false, fmt.Errorf("failed to check if question exists: %v", err)
    }
    return exists, nil
}

// GetInterviewContext retrieves the context of an interview by its ID
func (r *SQLInterviewRepository) GetInterviewContext(interviewID string) (*pb.StartInterviewRequest, error) {
    query := `
        SELECT position, experience, language, skills, max_questions, voice_id, speed, skip_code
        FROM interviews
        WHERE id = ?
    `
    var context pb.StartInterviewRequest
    var skillsJSON []byte
    err := r.db.QueryRow(query, interviewID).Scan(
        &context.Position,
        &context.Experience,
        &context.Language,
        &skillsJSON,
        &context.MaxQuestions,
        &context.Models,
        &context.Speed,
        &context.SkipCode,
    )

    if err == sql.ErrNoRows {
        return nil, fmt.Errorf("interview with ID %s not found", interviewID)
    } else if err != nil {
        return nil, fmt.Errorf("failed to retrieve interview context: %v", err)
    }

    if err := json.Unmarshal(skillsJSON, &context.Skills); err != nil {
        return nil, fmt.Errorf("failed to unmarshal skills: %v", err)
    }

    return &context, nil
}