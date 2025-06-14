package repo

import (
	"context"
    "github.com/spf13/viper"
    "entgo.io/ent/dialect/sql"
    "entgo.io/ent/dialect/sql/sqljson"

	pb "irelia/api"
	"irelia/pkg/ent"
    "irelia/pkg/ent/predicate"
	einterview "irelia/pkg/ent/interview"
	efavorite "irelia/pkg/ent/interviewfavorite"
)

type IInterview interface {
    Create(ctx context.Context, ownerId uint64, interview *ent.Interview) error
    Update(ctx context.Context, ownerId uint64, interview *ent.Interview) error
    Delete(ctx context.Context, ownerId uint64, interviewID string) error
    Get(ctx context.Context, id string) (*ent.Interview, error)
    GetContext(ctx context.Context, interviewID string) (*pb.StartInterviewRequest, error)
    List(ctx context.Context, req *pb.GetInterviewHistoryRequest, userId *uint64) ([]*ent.Interview, int32, int32, int32, error)
    Exists(ctx context.Context, interviewID string) (bool, error)
    Favorite(ctx context.Context, ownerId uint64, interviewID string) error
}

type EntInterview struct {
    client *ent.Client
}

func NewInterviewRepository(client *ent.Client) IInterview {
    return &EntInterview{client: client}
}

// Create creates a new interview in the database
func (r *EntInterview) Create(ctx context.Context, ownerId uint64, interview *ent.Interview) error {
    _, err := r.client.Interview.
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
        SetOverallScore(interview.OverallScore).
        SetPositiveFeedback(interview.PositiveFeedback).
        SetActionableFeedback(interview.ActionableFeedback).
        SetFinalComment(interview.FinalComment).
        SetStatus(pb.InterviewStatus_INTERVIEW_STATUS_IN_PROGRESS).
        Save(ctx)
    return err
}

// Update updates an existing interview in the database
func (r *EntInterview) Update(ctx context.Context, ownerId uint64, interview *ent.Interview) error {
    _, err := r.client.Interview.
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
        SetOverallScore(interview.OverallScore).
        SetPositiveFeedback(interview.PositiveFeedback).
        SetActionableFeedback(interview.ActionableFeedback).
        SetFinalComment(interview.FinalComment).
        SetStatus(interview.Status).
        Save(ctx)
    return err
}

func (r *EntInterview) Delete(ctx context.Context, ownerId uint64, interviewID string) error {
    _, err := r.client.Interview.
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

func SkillsContains(skill string) predicate.Interview {
    return predicate.Interview(func(s *sql.Selector) {
        s.Where(sqljson.StringContains(einterview.FieldSkills, skill))
    })
}

// List retrieves a list of completed interviews with search, paging, and ordering
func (r *EntInterview) List(ctx context.Context, req *pb.GetInterviewHistoryRequest, userId *uint64) ([]*ent.Interview, int32, int32, int32, error) {
    if req.Page == 0 {
        req.Page = 1
    }

    size := viper.GetInt("page_size")
    
    query := r.client.Interview.Query().Where(einterview.StatusEQ(pb.InterviewStatus_INTERVIEW_STATUS_COMPLETED))

    if req.Query != nil {
        query = query.Where(einterview.Or(
            einterview.PositionContainsFold(*req.Query),
            einterview.ExperienceContainsFold(*req.Query),
            SkillsContains(*req.Query),
        ))
    }

    if userId != nil {
        if req.Fvr != nil && *req.Fvr {
			query = query.Where(einterview.HasFavoritesWith(efavorite.UserID(*userId)))
		}
    }
    if req.En != nil {
        if *req.En {
            query = query.Where(einterview.LanguageEQ("English"))
        } else {
            query = query.Where(einterview.LanguageNEQ("English"))
        }
    }

    switch req.Sort {
    case pb.InterviewSortMethod_RECENTLY_RATED:
        query = query.Order(ent.Desc(einterview.FieldUpdatedAt))
    case pb.InterviewSortMethod_LEAST_RECENTLY_RATED:
        query = query.Order(ent.Asc(einterview.FieldUpdatedAt))
    case pb.InterviewSortMethod_MOST_TOTAL_QUESTIONS:
        query = query.Order(ent.Desc(einterview.FieldTotalQuestions))
    case pb.InterviewSortMethod_FEWEST_TOTAL_QUESTIONS:
        query = query.Order(ent.Asc(einterview.FieldTotalQuestions))
    case pb.InterviewSortMethod_MAX_SCORE:
        query = query.Order(ent.Desc(einterview.FieldOverallScore))
    case pb.InterviewSortMethod_MIN_SCORE:
        query = query.Order(ent.Asc(einterview.FieldOverallScore))
    default:
        query = query.Order(ent.Desc(einterview.FieldUpdatedAt))
    }

    totalCount, _ := query.Count(ctx)
    if totalCount == 0 {
        return nil, 0, 0, 0, nil
    }
    totalPage := int32((totalCount-1)/size + 1)

    interviews, err := query.
        Offset(int(req.Page-1) * size).
        Limit(size).
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
        return nil, 0, 0, 0, err
    }

    return interviews, int32(totalCount), int32(size), totalPage, nil
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