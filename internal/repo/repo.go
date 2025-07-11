package repo

import "irelia/pkg/ent"

type Repository struct {
	Interview IInterview
	Question  IQuestion
	PublicQuestion IPublicQuestion
	Ent       *ent.Client
}

func New(ent *ent.Client) *Repository {
	return &Repository{
		Ent:       ent,
		Interview: NewInterviewRepository(ent),
		Question:  NewQuestionRepository(ent),
		PublicQuestion: NewPublicQuestionRepository(ent),
	}
}
