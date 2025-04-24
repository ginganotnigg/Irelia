package repo

import (
	"context"

	pb "irelia/api"
	"irelia/internal/utils/tx"
	"irelia/pkg/ent"
	equestion "irelia/pkg/ent/question"
)

type IQuestion interface {
    Create(ctx context.Context, tx tx.Tx, userId uint64, question *ent.Question) error
    Update(ctx context.Context, tx tx.Tx, userId uint64, question *ent.Question) error
    Get(ctx context.Context, interviewID string, questionIndex int32) (*ent.Question, error)
    List(ctx context.Context, interviewID string) ([]*pb.AnswerResult, error)
    Exists(ctx context.Context, interviewID string, questionIndex int32) (bool, error)
    GetAnswers(ctx context.Context, interviewID string) ([]*pb.AnswerResult, error)
    GetQaPair(ctx context.Context, interviewID string, contextQALength int) ([]*pb.QaPair, error)
}

type EntQuestion struct {
    client *ent.Client
}

func NewQuestionRepository(client *ent.Client) IQuestion {
    return &EntQuestion{client: client}
}

// Create creates a new question in the database
func (r *EntQuestion) Create(ctx context.Context, tx tx.Tx, userId uint64, question *ent.Question) error {
    _, err := tx.Client().Question.
        Create().
        SetInterviewID(question.InterviewID).
        SetQuestionIndex(question.QuestionIndex).
        SetContent(question.Content).
        SetAudio(question.Audio).
        SetLipsync(question.Lipsync).
        SetAnswer(question.Answer).
        SetRecordProof(question.RecordProof).
        SetComment(question.Comment).
        SetScore(question.Score).
        SetStatus(pb.QuestionStatus_QUESTION_STATUS_NEW).
        Save(ctx)
    return err
}

// Update updates an existing question in the database
func (r *EntQuestion) Update(ctx context.Context, tx tx.Tx, userId uint64, question *ent.Question) error {
    _, err := tx.Client().Question.
        Update().
        Where(
            equestion.InterviewID(question.InterviewID),
            equestion.QuestionIndex(question.QuestionIndex),
        ).
        SetContent(question.Content).
        SetAudio(question.Audio).
        SetLipsync(question.Lipsync).
        SetAnswer(question.Answer).
        SetRecordProof(question.RecordProof).
        SetComment(question.Comment).
        SetScore(question.Score).
        SetStatus(question.Status).
        Save(ctx)
    return err
}

// Get retrieves a question by interview ID and question index
func (r *EntQuestion) Get(ctx context.Context, interviewID string, questionIndex int32) (*ent.Question, error) {
    return r.client.Question.
        Query().
        Where(
            equestion.InterviewID(interviewID),
            equestion.QuestionIndex(questionIndex),
        ).
        Only(ctx)
}

// List retrieves all answers for an interview
func (r *EntQuestion) List(ctx context.Context, interviewID string) ([]*pb.AnswerResult, error) {
    entQuestions, err := r.client.Question.
        Query().
        Where(equestion.InterviewID(interviewID)).
        Order(ent.Asc(equestion.FieldQuestionIndex)).
        All(ctx)
    if err != nil {
        return nil, err
    }

    answers := make([]*pb.AnswerResult, len(entQuestions))
    for i, entQuestion := range entQuestions {
        answers[i] = &pb.AnswerResult{
            Index:       int32(entQuestion.QuestionIndex),
            Content:     entQuestion.Content,
            Answer:      entQuestion.Answer,
            RecordProof: entQuestion.RecordProof,
            Comment:     entQuestion.Comment,
            Score:       entQuestion.Score,
        }
    }

    return answers, nil
}

// Exists checks if a question exists in the database
func (r *EntQuestion) Exists(ctx context.Context, interviewID string, questionIndex int32) (bool, error) {
    count, err := r.client.Question.
        Query().
        Where(
            equestion.InterviewID(interviewID),
            equestion.QuestionIndex(questionIndex),
        ).
        Count(ctx)
    if err != nil {
        return false, err
    }
    return count > 0, nil
}

// GetAnswers retrieves all answers for an interview
func (r *EntQuestion) GetAnswers(ctx context.Context, interviewID string) ([]*pb.AnswerResult, error) {
    entQuestions, err := r.client.Question.
        Query().
        Where(equestion.InterviewID(interviewID)).
        Order(ent.Asc(equestion.FieldQuestionIndex)).
        All(ctx)
    if err != nil {
        return nil, err
    }

    answers := make([]*pb.AnswerResult, len(entQuestions))
    for i, entQuestion := range entQuestions {
        answers[i] = &pb.AnswerResult{
            Index:       int32(entQuestion.QuestionIndex),
            Content:     entQuestion.Content,
            Answer:      entQuestion.Answer,
            RecordProof: entQuestion.RecordProof,
            Comment:     entQuestion.Comment,
            Score:       entQuestion.Score,
            Status:      entQuestion.Status,
        }
    }

    return answers, nil
}

// GetQaPair retrieves a question and its corresponding answer
func (r *EntQuestion) GetQaPair(ctx context.Context, interviewID string, contextQALength int) ([]*pb.QaPair, error) {
    totalCount, err := r.client.Question.
        Query().
        Where(equestion.InterviewID(interviewID)).
        Count(ctx)
	if err != nil {
		return nil, err
	}

    if totalCount < contextQALength {
        contextQALength = totalCount
    }

    entQuestions, err := r.client.Question.
        Query().
        Where(equestion.InterviewID(interviewID)).
        Order(ent.Asc(equestion.FieldQuestionIndex)).
        Offset(totalCount - contextQALength).
        Limit(contextQALength).
        All(ctx)
    if err != nil {
        return nil, err
    }

    qaPairs := make([]*pb.QaPair, len(entQuestions))
    for i, entQuestion := range entQuestions {
        qaPairs[i] = &pb.QaPair{
            Question: entQuestion.Content,
            Answer:   entQuestion.Answer,
        }
    }

    return qaPairs, nil
}