// Code generated by ent, DO NOT EDIT.

package ent

import (
	"context"
	"errors"
	"fmt"
	"irelia/pkg/ent/interview"
	"irelia/pkg/ent/interviewfavorite"
	"time"

	"entgo.io/ent/dialect/sql/sqlgraph"
	"entgo.io/ent/schema/field"
)

// InterviewFavoriteCreate is the builder for creating a InterviewFavorite entity.
type InterviewFavoriteCreate struct {
	config
	mutation *InterviewFavoriteMutation
	hooks    []Hook
}

// SetCreatedAt sets the "created_at" field.
func (ifc *InterviewFavoriteCreate) SetCreatedAt(t time.Time) *InterviewFavoriteCreate {
	ifc.mutation.SetCreatedAt(t)
	return ifc
}

// SetNillableCreatedAt sets the "created_at" field if the given value is not nil.
func (ifc *InterviewFavoriteCreate) SetNillableCreatedAt(t *time.Time) *InterviewFavoriteCreate {
	if t != nil {
		ifc.SetCreatedAt(*t)
	}
	return ifc
}

// SetUpdatedAt sets the "updated_at" field.
func (ifc *InterviewFavoriteCreate) SetUpdatedAt(t time.Time) *InterviewFavoriteCreate {
	ifc.mutation.SetUpdatedAt(t)
	return ifc
}

// SetNillableUpdatedAt sets the "updated_at" field if the given value is not nil.
func (ifc *InterviewFavoriteCreate) SetNillableUpdatedAt(t *time.Time) *InterviewFavoriteCreate {
	if t != nil {
		ifc.SetUpdatedAt(*t)
	}
	return ifc
}

// SetUserID sets the "user_id" field.
func (ifc *InterviewFavoriteCreate) SetUserID(u uint64) *InterviewFavoriteCreate {
	ifc.mutation.SetUserID(u)
	return ifc
}

// SetInterviewID sets the "interview_id" field.
func (ifc *InterviewFavoriteCreate) SetInterviewID(s string) *InterviewFavoriteCreate {
	ifc.mutation.SetInterviewID(s)
	return ifc
}

// SetInterview sets the "interview" edge to the Interview entity.
func (ifc *InterviewFavoriteCreate) SetInterview(i *Interview) *InterviewFavoriteCreate {
	return ifc.SetInterviewID(i.ID)
}

// Mutation returns the InterviewFavoriteMutation object of the builder.
func (ifc *InterviewFavoriteCreate) Mutation() *InterviewFavoriteMutation {
	return ifc.mutation
}

// Save creates the InterviewFavorite in the database.
func (ifc *InterviewFavoriteCreate) Save(ctx context.Context) (*InterviewFavorite, error) {
	ifc.defaults()
	return withHooks(ctx, ifc.sqlSave, ifc.mutation, ifc.hooks)
}

// SaveX calls Save and panics if Save returns an error.
func (ifc *InterviewFavoriteCreate) SaveX(ctx context.Context) *InterviewFavorite {
	v, err := ifc.Save(ctx)
	if err != nil {
		panic(err)
	}
	return v
}

// Exec executes the query.
func (ifc *InterviewFavoriteCreate) Exec(ctx context.Context) error {
	_, err := ifc.Save(ctx)
	return err
}

// ExecX is like Exec, but panics if an error occurs.
func (ifc *InterviewFavoriteCreate) ExecX(ctx context.Context) {
	if err := ifc.Exec(ctx); err != nil {
		panic(err)
	}
}

// defaults sets the default values of the builder before save.
func (ifc *InterviewFavoriteCreate) defaults() {
	if _, ok := ifc.mutation.CreatedAt(); !ok {
		v := interviewfavorite.DefaultCreatedAt()
		ifc.mutation.SetCreatedAt(v)
	}
	if _, ok := ifc.mutation.UpdatedAt(); !ok {
		v := interviewfavorite.DefaultUpdatedAt()
		ifc.mutation.SetUpdatedAt(v)
	}
}

// check runs all checks and user-defined validators on the builder.
func (ifc *InterviewFavoriteCreate) check() error {
	if _, ok := ifc.mutation.CreatedAt(); !ok {
		return &ValidationError{Name: "created_at", err: errors.New(`ent: missing required field "InterviewFavorite.created_at"`)}
	}
	if _, ok := ifc.mutation.UpdatedAt(); !ok {
		return &ValidationError{Name: "updated_at", err: errors.New(`ent: missing required field "InterviewFavorite.updated_at"`)}
	}
	if _, ok := ifc.mutation.UserID(); !ok {
		return &ValidationError{Name: "user_id", err: errors.New(`ent: missing required field "InterviewFavorite.user_id"`)}
	}
	if _, ok := ifc.mutation.InterviewID(); !ok {
		return &ValidationError{Name: "interview_id", err: errors.New(`ent: missing required field "InterviewFavorite.interview_id"`)}
	}
	if len(ifc.mutation.InterviewIDs()) == 0 {
		return &ValidationError{Name: "interview", err: errors.New(`ent: missing required edge "InterviewFavorite.interview"`)}
	}
	return nil
}

func (ifc *InterviewFavoriteCreate) sqlSave(ctx context.Context) (*InterviewFavorite, error) {
	if err := ifc.check(); err != nil {
		return nil, err
	}
	_node, _spec := ifc.createSpec()
	if err := sqlgraph.CreateNode(ctx, ifc.driver, _spec); err != nil {
		if sqlgraph.IsConstraintError(err) {
			err = &ConstraintError{msg: err.Error(), wrap: err}
		}
		return nil, err
	}
	id := _spec.ID.Value.(int64)
	_node.ID = int(id)
	ifc.mutation.id = &_node.ID
	ifc.mutation.done = true
	return _node, nil
}

func (ifc *InterviewFavoriteCreate) createSpec() (*InterviewFavorite, *sqlgraph.CreateSpec) {
	var (
		_node = &InterviewFavorite{config: ifc.config}
		_spec = sqlgraph.NewCreateSpec(interviewfavorite.Table, sqlgraph.NewFieldSpec(interviewfavorite.FieldID, field.TypeInt))
	)
	if value, ok := ifc.mutation.CreatedAt(); ok {
		_spec.SetField(interviewfavorite.FieldCreatedAt, field.TypeTime, value)
		_node.CreatedAt = value
	}
	if value, ok := ifc.mutation.UpdatedAt(); ok {
		_spec.SetField(interviewfavorite.FieldUpdatedAt, field.TypeTime, value)
		_node.UpdatedAt = value
	}
	if value, ok := ifc.mutation.UserID(); ok {
		_spec.SetField(interviewfavorite.FieldUserID, field.TypeUint64, value)
		_node.UserID = value
	}
	if nodes := ifc.mutation.InterviewIDs(); len(nodes) > 0 {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.M2O,
			Inverse: true,
			Table:   interviewfavorite.InterviewTable,
			Columns: []string{interviewfavorite.InterviewColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(interview.FieldID, field.TypeString),
			},
		}
		for _, k := range nodes {
			edge.Target.Nodes = append(edge.Target.Nodes, k)
		}
		_node.InterviewID = nodes[0]
		_spec.Edges = append(_spec.Edges, edge)
	}
	return _node, _spec
}

// InterviewFavoriteCreateBulk is the builder for creating many InterviewFavorite entities in bulk.
type InterviewFavoriteCreateBulk struct {
	config
	err      error
	builders []*InterviewFavoriteCreate
}

// Save creates the InterviewFavorite entities in the database.
func (ifcb *InterviewFavoriteCreateBulk) Save(ctx context.Context) ([]*InterviewFavorite, error) {
	if ifcb.err != nil {
		return nil, ifcb.err
	}
	specs := make([]*sqlgraph.CreateSpec, len(ifcb.builders))
	nodes := make([]*InterviewFavorite, len(ifcb.builders))
	mutators := make([]Mutator, len(ifcb.builders))
	for i := range ifcb.builders {
		func(i int, root context.Context) {
			builder := ifcb.builders[i]
			builder.defaults()
			var mut Mutator = MutateFunc(func(ctx context.Context, m Mutation) (Value, error) {
				mutation, ok := m.(*InterviewFavoriteMutation)
				if !ok {
					return nil, fmt.Errorf("unexpected mutation type %T", m)
				}
				if err := builder.check(); err != nil {
					return nil, err
				}
				builder.mutation = mutation
				var err error
				nodes[i], specs[i] = builder.createSpec()
				if i < len(mutators)-1 {
					_, err = mutators[i+1].Mutate(root, ifcb.builders[i+1].mutation)
				} else {
					spec := &sqlgraph.BatchCreateSpec{Nodes: specs}
					// Invoke the actual operation on the latest mutation in the chain.
					if err = sqlgraph.BatchCreate(ctx, ifcb.driver, spec); err != nil {
						if sqlgraph.IsConstraintError(err) {
							err = &ConstraintError{msg: err.Error(), wrap: err}
						}
					}
				}
				if err != nil {
					return nil, err
				}
				mutation.id = &nodes[i].ID
				if specs[i].ID.Value != nil {
					id := specs[i].ID.Value.(int64)
					nodes[i].ID = int(id)
				}
				mutation.done = true
				return nodes[i], nil
			})
			for i := len(builder.hooks) - 1; i >= 0; i-- {
				mut = builder.hooks[i](mut)
			}
			mutators[i] = mut
		}(i, ctx)
	}
	if len(mutators) > 0 {
		if _, err := mutators[0].Mutate(ctx, ifcb.builders[0].mutation); err != nil {
			return nil, err
		}
	}
	return nodes, nil
}

// SaveX is like Save, but panics if an error occurs.
func (ifcb *InterviewFavoriteCreateBulk) SaveX(ctx context.Context) []*InterviewFavorite {
	v, err := ifcb.Save(ctx)
	if err != nil {
		panic(err)
	}
	return v
}

// Exec executes the query.
func (ifcb *InterviewFavoriteCreateBulk) Exec(ctx context.Context) error {
	_, err := ifcb.Save(ctx)
	return err
}

// ExecX is like Exec, but panics if an error occurs.
func (ifcb *InterviewFavoriteCreateBulk) ExecX(ctx context.Context) {
	if err := ifcb.Exec(ctx); err != nil {
		panic(err)
	}
}
