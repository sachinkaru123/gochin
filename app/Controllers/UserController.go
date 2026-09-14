// Package controllers holds HTTP entry points.
//
// Controllers stay thin: read input from the Context, call exactly one
// service method, shape the response. Business logic and SQL belong in
// app/Services.
package controllers

import (
	"fmt"
	"net/http"

	requests "github.com/gochin/framework/app/Requests"
	services "github.com/gochin/framework/app/Services"
	"github.com/gochin/framework/pkg/router"
)

type UserController struct {
	users *services.UserService
}

func NewUserController(users *services.UserService) *UserController {
	return &UserController{users: users}
}

// Index returns a paginated list of users.
func (ct *UserController) Index(c *router.Context) error {
	page := c.QueryInt("page", 1)
	perPage := c.QueryInt("per_page", 0)

	result, err := ct.users.List(c.Ctx(), page, perPage)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, result)
}

// GetUsers is an alias of Index, matching the /get-users route.
func (ct *UserController) GetUsers(c *router.Context) error {
	return ct.Index(c)
}

// Show returns a single user.
func (ct *UserController) Show(c *router.Context) error {
	id, err := c.ParamInt("id")
	if err != nil {
		return err
	}

	user, err := ct.users.Get(c.Ctx(), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, user)
}

// Store creates a user.
func (ct *UserController) Store(c *router.Context) error {
	var req requests.CreateUser
	if err := c.Bind(&req); err != nil {
		return err
	}

	user, err := ct.users.Create(c.Ctx(), req)
	if err != nil {
		return err
	}

	c.Header().Set("Location", fmt.Sprintf("/api/v1/users/%d", user.ID))
	return c.JSON(http.StatusCreated, user)
}

// Update applies a partial update.
func (ct *UserController) Update(c *router.Context) error {
	id, err := c.ParamInt("id")
	if err != nil {
		return err
	}

	var req requests.UpdateUser
	if err := c.Bind(&req); err != nil {
		return err
	}

	user, err := ct.users.Update(c.Ctx(), id, req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, user)
}

// Destroy deletes a user.
func (ct *UserController) Destroy(c *router.Context) error {
	id, err := c.ParamInt("id")
	if err != nil {
		return err
	}

	if err := ct.users.Delete(c.Ctx(), id); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
