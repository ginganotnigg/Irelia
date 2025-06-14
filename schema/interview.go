package schema

import (
    pb "irelia/api"
    
    "entgo.io/ent"
    "entgo.io/ent/dialect/entsql"
    "entgo.io/ent/schema/edge"
    "entgo.io/ent/schema/field"
)

// Interview holds the schema definition for the Interview entity.
type Interview struct {
    ent.Schema
}

func (Interview) Mixin() []ent.Mixin {
	return []ent.Mixin{
		Base{},
	}
}

func (Interview) Fields() []ent.Field {
    return []ent.Field{
        field.String("id").Unique().Immutable(),
        field.Uint64("user_id").Immutable(),
        field.String("position").NotEmpty(),
        field.String("experience").Optional(),
        field.String("language").NotEmpty(),
        field.String("voice_id").Optional(),
        field.Int32("speed").Default(1),
        field.JSON("skills", []string{}).Optional(),
        field.JSON("skills_score", []string{}).Optional(),
        field.Bool("skip_code").Default(false),
        field.Int32("total_questions").Default(10),
        field.Int32("remaining_questions").Default(10),
        field.JSON("total_score", &pb.TotalScore{}).Optional(),
        field.Float("overall_score").Default(0),
        field.String("positive_feedback").Optional(),
        field.String("actionable_feedback").Optional(),
        field.String("final_comment").Optional(),
        field.Int32("status").GoType(pb.InterviewStatus(0)),
    }
}

func (Interview) Edges() []ent.Edge {
    return []ent.Edge{
        edge.To("questions", Question.Type).Annotations(entsql.Annotation{OnDelete: entsql.Cascade}),
        edge.To("favorites", InterviewFavorite.Type).Annotations(entsql.Annotation{OnDelete: entsql.Cascade}),
    }
}