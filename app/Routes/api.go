// Package routes declares the application's HTTP routes.
//
// Every file in this package registers itself from init(), so adding a new
// route file requires no change anywhere else — the server blank-imports this
// package once and picks all of them up.
package routes

import (
	controllers "github.com/gochin/framework/app/Controllers"
	middleware "github.com/gochin/framework/app/Middleware"
	services "github.com/gochin/framework/app/Services"
	"github.com/gochin/framework/pkg/config"
	"github.com/gochin/framework/pkg/router"
	mw "github.com/gochin/framework/pkg/router/middleware"
)

func init() {
	router.Register(func(r *router.Router) {
		// Composition root: controllers get their dependencies explicitly,
		// which is what keeps them testable without a container.
		guard := services.MustAuthGuard()
		userCtrl := controllers.NewUserController(services.NewUserService())
		postCtrl := controllers.NewPostController(services.NewPostService())
		authCtrl := controllers.NewAuthController(services.NewAuthService(), guard)

		// Full prefix is GOCHIN_API_PREFIX + /v1, e.g. /api/v1
		v1 := r.Group("/v1")

		// --- authentication ---------------------------------------------
		// Credential endpoints carry their own tight per-IP budget: the
		// global limiter is disabled by default, so without this there is no
		// brute-force protection at all.
		loginLimit := mw.RateLimit(mw.RateLimitConfig{
			RequestsPerMinute: config.Get().Auth.LoginRatePerMinute,
			Burst:             config.Get().Auth.LoginRatePerMinute,
		})

		auth := v1.Group("/auth")
		auth.Post("/register", authCtrl.Register, loginLimit)
		auth.Post("/login", authCtrl.Login, loginLimit)
		auth.Get("/me", authCtrl.Me, middleware.Auth())
		auth.Post("/logout", authCtrl.Logout, middleware.Auth())
		auth.Post("/logout-all", authCtrl.LogoutAll, middleware.Auth())

		// --- users ------------------------------------------------------
		// There is deliberately no POST /users: accounts are created through
		// /auth/register so that a password is always hashed and set.
		users := v1.Group("/users", middleware.Auth())
		users.Get("", userCtrl.Index)
		users.Get("/{id}", userCtrl.Show)
		users.Put("/{id}", userCtrl.Update)
		users.Delete("/{id}", userCtrl.Destroy)
		users.Get("/{id}/posts", postCtrl.ByUser)

		v1.Get("/get-users", userCtrl.GetUsers, middleware.Auth())

		// --- posts ------------------------------------------------------
		// Reads are public; writes require a token.
		posts := v1.Group("/posts")
		posts.Get("", postCtrl.Index)
		posts.Get("/{id}", postCtrl.Show)
		posts.Post("", postCtrl.Store, middleware.Auth())
		posts.Delete("/{id}", postCtrl.Destroy, middleware.Auth())
		posts.Post("/{id}/tags/{tagID}", postCtrl.AttachTag, middleware.Auth())
	})
}
