package repo

import (
    "context"

    "irelia/pkg/ent"
    pb "irelia/api"
    epq "irelia/pkg/ent/publicquestion"
)

type IPublicQuestion interface {
    List(ctx context.Context, req *pb.GetPublicQuestionRequest) ([]*ent.PublicQuestion, int32, int32, int32, error)
    CreateBulk(ctx context.Context, questions []*ent.PublicQuestion) error
}

type EntPublicQuestion struct {
    client *ent.Client
}

func NewPublicQuestionRepository(client *ent.Client) IPublicQuestion {
    return &EntPublicQuestion{client: client}
}

func (r *EntPublicQuestion) List(ctx context.Context, req *pb.GetPublicQuestionRequest) ([]*ent.PublicQuestion, int32, int32, int32, error) {
    if req.Page == 0 {
        req.Page = 1
    }

	query := r.client.PublicQuestion.Query()
	if req.Pos != nil {
		query = query.Where(epq.PositionEQ(*req.Pos))
	}
	if req.Exp != nil {
		query = query.Where(epq.ExperienceEQ(*req.Exp))
	}
	if req.Lang != nil {
		query = query.Where(epq.LanguageEQ(*req.Lang))
	}
    totalCount, _ := query.Count(ctx)

    const pageSize = 20
    page := int(req.Page)
    if page < 1 {
        page = 1
    }
    offset := (page - 1) * pageSize
    totalPage := int32((totalCount-1)/pageSize + 1)

    questions, err := query.Order(ent.Desc(epq.FieldID)).
        Limit(pageSize).
        Offset(offset).
        Select(
            epq.FieldContent,
            epq.FieldAnswer,
            epq.FieldPosition,
            epq.FieldExperience,
            epq.FieldCreatedAt,
            epq.FieldUpdatedAt,
        ).
        All(ctx)
    if err != nil {
        return nil, 0, 0, 0, err
    }
    return questions, int32(totalCount), int32(pageSize), totalPage, nil
}

func (r *EntPublicQuestion) CreateBulk(ctx context.Context, questions []*ent.PublicQuestion) error {
    builders := make([]*ent.PublicQuestionCreate, len(questions))
    for i, q := range questions {
        builders[i] = r.client.PublicQuestion.
            Create().
            SetPosition(q.Position).
            SetExperience(q.Experience).
			SetLanguage(q.Language).
            SetContent(q.Content)
    }
    _, err := r.client.PublicQuestion.CreateBulk(builders...).Save(ctx)
    return err
}