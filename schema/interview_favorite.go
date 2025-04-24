package schema

import (
    "entgo.io/ent"
    "entgo.io/ent/schema/edge"
    "entgo.io/ent/schema/field"
)

// InterviewFavorite holds the schema definition for the InterviewFavorite entity.
type InterviewFavorite struct {
    ent.Schema
}

func (InterviewFavorite) Mixin() []ent.Mixin {
	return []ent.Mixin{
		Base{},
	}
}

func (InterviewFavorite) Fields() []ent.Field {
    return []ent.Field{
        field.Uint64("user_id"),
        field.String("interview_id"),
    }
}

func (InterviewFavorite) Edges() []ent.Edge {
    return []ent.Edge{
        edge.From("interview", Interview.Type).Ref("favorites").Field("interview_id").Unique().Required(),
    }
}