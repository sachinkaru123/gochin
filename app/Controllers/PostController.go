package controllers

import (
	"fmt"
	"net/http"

	models "github.com/gochin/framework/app/Models"
	requests "github.com/gochin/framework/app/Requests"
	services "github.com/gochin/framework/app/Services"
	"github.com/gochin/framework/pkg/router"
)

type PostController struct {
	posts *services.PostService
}

func NewPostController(posts *services.PostService) *PostController {
	return &PostController{posts: posts}
}

// Index lists posts with their author and tags.
func (c *PostController) Index(ctx *router.Context) error {
	result, err := c.posts.List(ctx.Ctx(), ctx.QueryInt("page", 1), ctx.QueryInt("per_page", 0))
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, result)
}

// Show returns one post with its relations.
func (c *PostController) Show(ctx *router.Context) error {
	id, err := ctx.ParamInt("id")
	if err != nil {
		return err
	}

	post, err := c.posts.Get(ctx.Ctx(), id)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, post)
}

// ByUser lists the posts belonging to one user.
func (c *PostController) ByUser(ctx *router.Context) error {
	userID, err := ctx.ParamInt("id")
	if err != nil {
		return err
	}

	posts, err := c.posts.ByUser(ctx.Ctx(), userID)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, map[string]any{"data": posts})
}

// Store creates a post. An unknown user_id is rejected by the foreign key.
func (c *PostController) Store(ctx *router.Context) error {
	var req requests.CreatePost
	if err := ctx.Bind(&req); err != nil {
		return err
	}
	if err := req.Validate(); err != nil {
		return err
	}

	post := &models.Post{
		Title:     req.Title,
		Body:      req.Body,
		Published: req.Published,
		UserID:    req.UserID,
	}
	if err := c.posts.Create(ctx.Ctx(), post); err != nil {
		return err
	}

	ctx.Header().Set("Location", fmt.Sprintf("/api/v1/posts/%d", post.ID))
	return ctx.JSON(http.StatusCreated, post)
}

// Destroy deletes a post.
func (c *PostController) Destroy(ctx *router.Context) error {
	id, err := ctx.ParamInt("id")
	if err != nil {
		return err
	}

	if err := c.posts.Delete(ctx.Ctx(), id); err != nil {
		return err
	}
	return ctx.NoContent(http.StatusNoContent)
}

// AttachTag links an existing tag to an existing post.
func (c *PostController) AttachTag(ctx *router.Context) error {
	postID, err := ctx.ParamInt("id")
	if err != nil {
		return err
	}
	tagID, err := ctx.ParamInt("tagID")
	if err != nil {
		return err
	}

	if err := c.posts.AttachTag(ctx.Ctx(), postID, tagID); err != nil {
		return err
	}
	return ctx.NoContent(http.StatusNoContent)
}
