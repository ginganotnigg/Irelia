package schema

import (
    "entgo.io/ent"
    "entgo.io/ent/schema/field"
)

// Question holds the schema definition for the Question entity.
type PublicQuestion struct {
    ent.Schema
}

func (PublicQuestion) Mixin() []ent.Mixin {
	return []ent.Mixin{
		Base{},
	}
}

func (PublicQuestion) Fields() []ent.Field {
    return []ent.Field{
        field.String("position").NotEmpty(),
        field.String("experience").NotEmpty(),
		field.String("language").NotEmpty(),
        field.Text("content").NotEmpty(),
        field.Text("answer").Optional(),
    }
}