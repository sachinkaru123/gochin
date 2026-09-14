package services

import (
	"context"

	models "github.com/gochin/framework/app/Models"
	requests "github.com/gochin/framework/app/Requests"
	"github.com/gochin/framework/pkg/orm"
)

// UserService holds the business rules for users.
//
// It is stateless: the ORM resolves the database connection lazily per call,
// so constructing a service never opens a connection and `route list` works
// without a database.
type UserService struct{}

func NewUserService() *UserService { return &UserService{} }

// List returns one page of users, newest first.
func (s *UserService) List(ctx context.Context, page, perPage int) (*orm.Page[models.User], error) {
	return orm.Query[models.User]().
		WithContext(ctx).
		OrderBy("id", "DESC").
		Paginate(page, perPage)
}

// Get returns a single user by id.
func (s *UserService) Get(ctx context.Context, id int64) (*models.User, error) {
	return orm.FindCtx[models.User](ctx, id)
}

// FindByEmail returns the user with the given email.
func (s *UserService) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	return orm.Query[models.User]().
		WithContext(ctx).
		Where("email", "=", email).
		First()
}

// Create validates and persists a new user.
func (s *UserService) Create(ctx context.Context, in requests.CreateUser) (*models.User, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}

	user := &models.User{Name: in.Name, Email: in.Email}
	if err := orm.CreateCtx(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

// Update applies a partial update to an existing user.
func (s *UserService) Update(ctx context.Context, id int64, in requests.UpdateUser) (*models.User, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}

	user, err := orm.FindCtx[models.User](ctx, id)
	if err != nil {
		return nil, err
	}

	if in.Name != nil {
		user.Name = *in.Name
	}
	if in.Email != nil {
		user.Email = *in.Email
	}

	if err := orm.UpdateCtx(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

// Delete removes a user by id.
func (s *UserService) Delete(ctx context.Context, id int64) error {
	user, err := orm.FindCtx[models.User](ctx, id)
	if err != nil {
		return err
	}
	return orm.DeleteCtx(ctx, user)
}
