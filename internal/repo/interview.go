package repo

import (
	"context"
	"errors"

	"entgo.io/ent/dialect/sql"
	amqp "github.com/rabbitmq/amqp091-go"
	"google.golang.org/protobuf/encoding/protojson"

	pb "irelia/api"
	"irelia/internal/utils/sort"
	"irelia/internal/utils/tx"
	"irelia/pkg/ent"
	einterview "irelia/pkg/ent/interview"
	efavorite "irelia/pkg/ent/interviewfavorite"
)

type IInterview interface {
    Create(ctx context.Context, tx tx.Tx, ownerId uint64, interview *ent.Interview) error
    Update(ctx context.Context, tx tx.Tx, ownerId uint64, interview *ent.Interview) error
    Delete(ctx context.Context, tx tx.Tx, ownerId uint64, interviewID string) error
    Get(ctx context.Context, id string) (*ent.Interview, error)
    GetContext(ctx context.Context, interviewID string) (*pb.StartInterviewRequest, error)
    List(ctx context.Context, req *pb.GetInterviewHistoryRequest, userId *uint64) ([]*ent.Interview, int32, int32, error)
    Exists(ctx context.Context, interviewID string) (bool, error)
    Favorite(ctx context.Context, ownerId uint64, interviewID string) error
    ReceiveScore(ctx context.Context, msg amqp.Delivery) error
}

type EntInterview struct {
    client *ent.Client
}

func NewInterviewRepository(client *ent.Client) IInterview {
    return &EntInterview{client: client}
}

// Create creates a new interview in the database
func (r *EntInterview) Create(ctx context.Context, tx tx.Tx, ownerId uint64, interview *ent.Interview) error {
    _, err := tx.Client().Interview.
        Create().
        SetID(interview.ID).
        SetUserID(ownerId).
        SetPosition(interview.Position).
        SetExperience(interview.Experience).
        SetLanguage(interview.Language).
        SetVoiceID(interview.VoiceID).
        SetSpeed(interview.Speed).
        SetSkills(interview.Skills).
        SetSkipCode(interview.SkipCode).
        SetTotalQuestions(interview.TotalQuestions).
        SetRemainingQuestions(interview.RemainingQuestions).
        SetTotalScore(interview.TotalScore).
        SetPositiveFeedback(interview.PositiveFeedback).
        SetActionableFeedback(interview.ActionableFeedback).
        SetFinalComment(interview.FinalComment).
        SetStatus(pb.InterviewStatus_INTERVIEW_STATUS_IN_PROGRESS).
        Save(ctx)
    return err
}

// Update updates an existing interview in the database
func (r *EntInterview) Update(ctx context.Context, tx tx.Tx, ownerId uint64, interview *ent.Interview) error {
    _, err := tx.Client().Interview.
        UpdateOneID(interview.ID).
        SetPosition(interview.Position).
        SetExperience(interview.Experience).
        SetLanguage(interview.Language).
        SetVoiceID(interview.VoiceID).
        SetSpeed(interview.Speed).
        SetSkills(interview.Skills).
        SetSkillsScore(interview.SkillsScore).
        SetSkipCode(interview.SkipCode).
        SetTotalQuestions(interview.TotalQuestions).
        SetRemainingQuestions(interview.RemainingQuestions).
        SetTotalScore(interview.TotalScore).
        SetPositiveFeedback(interview.PositiveFeedback).
        SetActionableFeedback(interview.ActionableFeedback).
        SetFinalComment(interview.FinalComment).
        SetStatus(interview.Status).
        Save(ctx)
    return err
}

func (r *EntInterview) Delete(ctx context.Context, tx tx.Tx, ownerId uint64, interviewID string) error {
    _, err := tx.Client().Interview.
        Delete().
        Where(
            einterview.ID(interviewID),
            einterview.UserID(ownerId),
        ).
        Exec(ctx)
    if err != nil {
        return err
    }
    return nil
}

// Get retrieves an interview by ID
func (r *EntInterview) Get(ctx context.Context, id string) (*ent.Interview, error) {
    return r.client.Interview.
        Query().
        Where(einterview.ID(id)).
        Only(ctx)
}

// GetContext retrieves the context of an interview by its ID
func (r *EntInterview) GetContext(ctx context.Context, interviewID string) (*pb.StartInterviewRequest, error) {
    entInterview, err := r.client.Interview.
        Query().
        Where(einterview.ID(interviewID)).
        Only(ctx)
    if err != nil {
        return nil, err
    }

    return &pb.StartInterviewRequest{
        Position:       entInterview.Position,
        Experience:     entInterview.Experience,
        Language:       entInterview.Language,
        Skills:         entInterview.Skills,
        TotalQuestions: entInterview.TotalQuestions,
        Models:         entInterview.VoiceID,
        Speed:          entInterview.Speed,
        SkipCode:       entInterview.SkipCode,
    }, nil
}

// List retrieves a list of completed interviews with search, paging, and ordering
func (r *EntInterview) List(ctx context.Context, req *pb.GetInterviewHistoryRequest, userId *uint64) ([]*ent.Interview, int32, int32, error) {
    if req.PageIndex == 0 {
        req.PageIndex = 1 // Default to the first page
    }
    if req.PageSize == 0 {
        req.PageSize = 10 // Default to 10 items per page
    }
    query := r.client.Interview.Query().Where(einterview.StatusEQ(pb.InterviewStatus_INTERVIEW_STATUS_COMPLETED))

    if (req.From == nil && req.To != nil) || (req.From != nil && req.To == nil) {
		return nil, 0, 0, errors.New("invalid time")
	} else if req.From != nil {
		if req.From.AsTime().After(req.To.AsTime()) {
			return nil, 0, 0, errors.New("invalid time")
		}
		query = query.Where(einterview.CreatedAtGTE(req.From.AsTime()), einterview.CreatedAtLTE(req.To.AsTime()))
	}

    if req.SearchContent != nil {
        query = query.Where(einterview.Or(
            einterview.PositionContainsFold(*req.SearchContent),
            einterview.ExperienceContainsFold(*req.SearchContent),
            einterview.LanguageContainsFold(*req.SearchContent),
            einterview.VoiceIDContainsFold(*req.SearchContent),
        ))
    }

    if userId != nil {
        if req.IsFavorite != nil && *req.IsFavorite {
			query = query.Where(einterview.HasFavoritesWith(efavorite.UserID(*userId)))
		}
    }

    sorts, err := sort.GetSort(einterview.Columns, einterview.Table, req.Sort)
    if err != nil {
        return nil, 0, 0, err
    }
    totalCount, _ := query.Count(ctx)
    if totalCount == 0 {
        return nil, 0, 0, nil
    }
    totalPage := (int32(totalCount)-1)/req.PageSize + 1

    interviews, err := query.
        Modify(func(s *sql.Selector) {
            s.OrderBy(sorts...)
        }).
        Offset(int(req.PageIndex-1) * int(req.PageSize)).
        Limit(int(req.PageSize)).
        Select(
            einterview.FieldID,
            einterview.FieldPosition,
            einterview.FieldExperience,
            einterview.FieldTotalScore,
            einterview.FieldCreatedAt,
            einterview.FieldUpdatedAt,
        ).
        All(ctx)
    if err != nil {
        return nil, 0, 0, err
    }

    return interviews, int32(totalCount), totalPage, nil
}

// Exists checks if an interview exists in the database
func (r *EntInterview) Exists(ctx context.Context, interviewID string) (bool, error) {
    count, err := r.client.Interview.
        Query().
        Where(einterview.ID(interviewID)).
        Count(ctx)
    if err != nil {
        return false, err
    }
    return count > 0, nil
}

// Favorite toggles the favorite status of an interview for a user
func (r *EntInterview) Favorite(ctx context.Context, ownerId uint64, interviewID string) error {
    exists, err := r.client.InterviewFavorite.
        Query().
        Where(
            efavorite.UserID(ownerId),
            efavorite.InterviewID(interviewID),
        ).
        Exist(ctx)
    if err != nil {
        return err
    }

    if exists {
        _, err := r.client.InterviewFavorite.
            Delete().
            Where(
                efavorite.UserID(ownerId),
                efavorite.InterviewID(interviewID),
            ).
            Exec(ctx)
        return err
    }

    _, err = r.client.InterviewFavorite.
        Create().
        SetUserID(ownerId).
        SetInterviewID(interviewID).
        Save(ctx)
    return err
}

func (r *EntInterview) ReceiveScore(ctx context.Context, msg amqp.Delivery) error {
    var score pb.ScoreInterviewRequest
    if err := protojson.Unmarshal(msg.Body, &score); err != nil {
        return err
    }

    interview, err := r.client.Interview.
        Query().
        Where(einterview.ID(score.InterviewId)).
        Only(ctx)
    if err != nil {
        return err
    }

    return tx.WithTransaction(ctx, r.client, func(ctx context.Context, tx tx.Tx) error {
		_, err := tx.Client().Interview.
        UpdateOneID(interview.ID).
        SetSkillsScore(interview.SkillsScore).
        SetTotalScore(interview.TotalScore).
        SetPositiveFeedback(interview.PositiveFeedback).
        SetActionableFeedback(interview.ActionableFeedback).
        SetFinalComment(interview.FinalComment).
        SetStatus(interview.Status).
        Save(ctx)
        return err
	})
}