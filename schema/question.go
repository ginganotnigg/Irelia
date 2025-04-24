package schema

import (
    pb "irelia/api"

    "entgo.io/ent"
    "entgo.io/ent/schema/edge"
    "entgo.io/ent/schema/field"
)

// Question holds the schema definition for the Question entity.
type Question struct {
    ent.Schema
}

func (Question) Mixin() []ent.Mixin {
	return []ent.Mixin{
		Base{},
	}
}

func (Question) Fields() []ent.Field {
    return []ent.Field{
        field.String("interview_id").NotEmpty().Immutable(),
        field.Int32("question_index").Immutable(),
        field.Text("content").NotEmpty(),
        field.Text("audio").Optional(),
        field.JSON("lipsync", &pb.LipSyncData{}).Optional(),
        field.Text("answer").Optional(),
        field.Text("record_proof").Optional(),
        field.Text("comment").Optional(),
        field.String("score").Optional(),
        field.Int32("status").GoType(pb.QuestionStatus(0)),
    }
}

func (Question) Edges() []ent.Edge {
    return []ent.Edge{
        edge.From("interview", Interview.Type).Ref("questions").Field("interview_id").Unique().Required().Immutable(),
    }
}