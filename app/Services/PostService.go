package services

import (
	"context"

	models "github.com/gochin/framework/app/Models"
	"github.com/gochin/framework/pkg/orm"
)

type PostService struct{}

func NewPostService() *PostService { return &PostService{} }

// List returns a page of posts with their author and tags attached.
//
// Cost is three queries total regardless of page size: one for the posts, one
// for every author, one for every tag. Loading relations inside the loop
// instead would make it 1 + 2N.
func (s *PostService) List(ctx context.Context, page, perPage int) (*orm.Page[models.Post], error) {
	result, err := orm.Query[models.Post]().
		WithContext(ctx).
		OrderBy("id", "DESC").
		Paginate(page, perPage)
	if err != nil {
		return nil, err
	}

	if err := s.attachRelations(ctx, result.Data); err != nil {
		return nil, err
	}
	return result, nil
}

// Get returns one post with its relations attached.
func (s *PostService) Get(ctx context.Context, id int64) (*models.Post, error) {
	post, err := orm.FindCtx[models.Post](ctx, id)
	if err != nil {
		return nil, err
	}

	if err := s.attachRelations(ctx, []*models.Post{post}); err != nil {
		return nil, err
	}
	return post, nil
}

// ByUser returns every post belonging to a user.
func (s *PostService) ByUser(ctx context.Context, userID int64) ([]*models.Post, error) {
	return orm.Query[models.Post]().
		WithContext(ctx).
		Where("user_id", "=", userID).
		OrderBy("id", "DESC").
		Get()
}

// Create persists a post. A user_id with no matching user is rejected by the
// database's foreign key, which surfaces as a 400 through the error mapper.
func (s *PostService) Create(ctx context.Context, post *models.Post) error {
	return orm.CreateCtx(ctx, post)
}

func (s *PostService) Update(ctx context.Context, post *models.Post) error {
	return orm.UpdateCtx(ctx, post)
}

func (s *PostService) Delete(ctx context.Context, id int64) error {
	post, err := orm.FindCtx[models.Post](ctx, id)
	if err != nil {
		return err
	}
	return orm.DeleteCtx(ctx, post)
}

// attachRelations loads the author and tags for a batch of posts.
func (s *PostService) attachRelations(ctx context.Context, posts []*models.Post) error {
	if len(posts) == 0 {
		return nil
	}

	if err := orm.LoadBelongsTo(ctx, posts,
		func(p *models.Post) int64 { return p.UserID },
		func(p *models.Post, u *models.User) { p.Author = u },
	); err != nil {
		return err
	}

	return orm.LoadBelongsToMany(ctx, posts, "post_tag", "post_id", "tag_id",
		func(p *models.Post) int64 { return p.ID },
		func(p *models.Post, tags []*models.Tag) { p.Tags = tags },
	)
}

// AttachTag links a tag to a post through the pivot table.
func (s *PostService) AttachTag(ctx context.Context, postID, tagID int64) error {
	db, err := orm.DefaultExecutor()
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx,
		`INSERT INTO post_tag (post_id, tag_id) VALUES ($1, $2)
		 ON CONFLICT (post_id, tag_id) DO NOTHING`,
		postID, tagID)
	return err
}
