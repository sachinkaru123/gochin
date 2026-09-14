package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// NewMakeCommand creates the make command with its subcommands
func NewMakeCommand() *cobra.Command {
	makeCmd := &cobra.Command{
		Use:   "make",
		Short: "Generate application components",
		Long:  "Generate application components: controllers, models, services, middleware and migrations.",
	}

	makeCmd.AddCommand(newMakeControllerCommand())
	makeCmd.AddCommand(newMakeModelCommand())
	makeCmd.AddCommand(newMakeServiceCommand())
	makeCmd.AddCommand(newMakeMiddlewareCommand())
	makeCmd.AddCommand(newMakeMigrationCommand())
	makeCmd.AddCommand(newMakeSeederCommand())

	return makeCmd
}

// ---------------------------------------------------------------- controller

func newMakeControllerCommand() *cobra.Command {
	var resource, force bool

	cmd := &cobra.Command{
		Use:   "controller [name]",
		Short: "Generate a new controller",
		Long:  "Generate a controller in app/Controllers, optionally with full CRUD actions.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return generateController(args[0], resource, force)
		},
	}

	cmd.Flags().BoolVarP(&resource, "resource", "r", false, "Generate Index/Show/Store/Update/Destroy actions")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite the file if it already exists")
	return cmd
}

const controllerHeader = `package controllers

import (
	"net/http"

	services "%[4]s/app/Services"
	"github.com/sachinkaru123/gochin/pkg/router"
)

// %[1]sController handles HTTP requests for %[2]s.
//
// Keep actions thin: read input, call one service method, shape the response.
type %[1]sController struct {
	%[3]s *services.%[1]sService
}

func New%[1]sController(%[3]s *services.%[1]sService) *%[1]sController {
	return &%[1]sController{%[3]s: %[3]s}
}
`

const controllerIndexAction = `
// Index returns a paginated list of %[2]s.
func (c *%[1]sController) Index(ctx *router.Context) error {
	page := ctx.QueryInt("page", 1)
	perPage := ctx.QueryInt("per_page", 0)

	result, err := c.%[3]s.List(ctx.Ctx(), page, perPage)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, result)
}
`

const controllerResourceActions = `
// Show returns a single %[4]s.
func (c *%[1]sController) Show(ctx *router.Context) error {
	id, err := ctx.ParamInt("id")
	if err != nil {
		return err
	}

	record, err := c.%[3]s.Get(ctx.Ctx(), id)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, record)
}

// Store creates a %[4]s.
func (c *%[1]sController) Store(ctx *router.Context) error {
	// TODO: bind a request struct from app/Requests, e.g.
	//   var req requests.Create%[1]s
	//   if err := ctx.Bind(&req); err != nil { return err }
	return router.NewError(http.StatusNotImplemented, "Store is not implemented yet")
}

// Update modifies an existing %[4]s.
func (c *%[1]sController) Update(ctx *router.Context) error {
	if _, err := ctx.ParamInt("id"); err != nil {
		return err
	}
	return router.NewError(http.StatusNotImplemented, "Update is not implemented yet")
}

// Destroy deletes a %[4]s.
func (c *%[1]sController) Destroy(ctx *router.Context) error {
	id, err := ctx.ParamInt("id")
	if err != nil {
		return err
	}

	if err := c.%[3]s.Delete(ctx.Ctx(), id); err != nil {
		return err
	}
	return ctx.NoContent(http.StatusNoContent)
}
`

func generateController(name string, resource, force bool) error {
	module, err := moduleName()
	if err != nil {
		return err
	}

	name = pascalCase(strings.TrimSuffix(pascalCase(name), "Controller"))
	plural := pluralize(strings.ToLower(name))
	receiver := camelCase(pluralize(name))

	content := fmt.Sprintf(controllerHeader, name, plural, receiver, module)
	content += fmt.Sprintf(controllerIndexAction, name, plural, receiver)
	if resource {
		content += fmt.Sprintf(controllerResourceActions, name, plural, receiver, strings.ToLower(name))
	}

	path, err := writeGenerated(filepath.Join("app", "Controllers"), name+"Controller.go", content, force)
	if err != nil {
		return err
	}

	fmt.Printf("✅ Controller created: %s\n", path)
	fmt.Printf("   Requires app/Services/%sService.go — run: gochin make service %s\n", name, name)
	fmt.Println("\n   Register it in a route file under app/Routes:")
	fmt.Printf("     %s := controllers.New%sController(services.New%sService())\n", receiver, name, name)
	fmt.Printf("     v1.Get(\"/%s\", %s.Index)\n", kebabCase(plural), receiver)
	return nil
}

// ------------------------------------------------------------------- service

func newMakeServiceCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "service [name]",
		Short: "Generate a new service",
		Long:  "Generate a service in app/Services holding business logic and data access.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return generateService(args[0], force)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite the file if it already exists")
	return cmd
}

const serviceTemplate = `package services

import (
	"context"

	models "%[4]s/app/Models"
	"github.com/sachinkaru123/gochin/pkg/orm"
)

// %[1]sService holds the business rules for %[2]s.
//
// Keep it stateless: the ORM resolves the database connection lazily, so a
// service constructor never needs to open one.
type %[1]sService struct{}

func New%[1]sService() *%[1]sService { return &%[1]sService{} }

// List returns one page of %[2]s.
func (s *%[1]sService) List(ctx context.Context, page, perPage int) (*orm.Page[models.%[1]s], error) {
	return orm.Query[models.%[1]s]().
		WithContext(ctx).
		OrderBy("id", "DESC").
		Paginate(page, perPage)
}

// Get returns a single %[3]s by id.
func (s *%[1]sService) Get(ctx context.Context, id int64) (*models.%[1]s, error) {
	return orm.FindCtx[models.%[1]s](ctx, id)
}

// Create persists a new %[3]s.
func (s *%[1]sService) Create(ctx context.Context, record *models.%[1]s) error {
	return orm.CreateCtx(ctx, record)
}

// Update persists changes to an existing %[3]s.
func (s *%[1]sService) Update(ctx context.Context, record *models.%[1]s) error {
	return orm.UpdateCtx(ctx, record)
}

// Delete removes a %[3]s by id.
func (s *%[1]sService) Delete(ctx context.Context, id int64) error {
	record, err := orm.FindCtx[models.%[1]s](ctx, id)
	if err != nil {
		return err
	}
	return orm.DeleteCtx(ctx, record)
}
`

func generateService(name string, force bool) error {
	module, err := moduleName()
	if err != nil {
		return err
	}

	name = pascalCase(strings.TrimSuffix(pascalCase(name), "Service"))
	plural := pluralize(strings.ToLower(name))

	content := fmt.Sprintf(serviceTemplate, name, plural, strings.ToLower(name), module)

	path, err := writeGenerated(filepath.Join("app", "Services"), name+"Service.go", content, force)
	if err != nil {
		return err
	}

	fmt.Printf("✅ Service created: %s\n", path)
	fmt.Printf("   Requires app/Models/%s.go — run: gochin make model %s\n", name, name)
	return nil
}

// ---------------------------------------------------------------- middleware

func newMakeMiddlewareCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "middleware [name]",
		Short: "Generate a new middleware",
		Long:  "Generate a middleware in app/Middleware.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return generateMiddleware(args[0], force)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite the file if it already exists")
	return cmd
}

const middlewareTemplate = `package middleware

import (
	"github.com/sachinkaru123/gochin/pkg/router"
)

// %[1]s returns a middleware that runs around matching routes.
//
// Return without calling next to short-circuit the request; returning an
// error hands control to the router's central error handler.
func %[1]s() router.Middleware {
	return func(next router.Handler) router.Handler {
		return func(c *router.Context) error {
			// TODO: pre-processing goes here.

			if err := next(c); err != nil {
				return err
			}

			// TODO: post-processing goes here.
			return nil
		}
	}
}
`

func generateMiddleware(name string, force bool) error {
	name = pascalCase(name)
	content := fmt.Sprintf(middlewareTemplate, name)

	path, err := writeGenerated(filepath.Join("app", "Middleware"), name+".go", content, force)
	if err != nil {
		return err
	}

	fmt.Printf("✅ Middleware created: %s\n", path)
	fmt.Printf("   Apply it per route:  v1.Get(\"/path\", ctrl.Action, middleware.%s())\n", name)
	fmt.Printf("   Or per group:        v1 := r.Group(\"/v1\", middleware.%s())\n", name)
	return nil
}

// ----------------------------------------------------------------- migration

func newMakeMigrationCommand() *cobra.Command {
	var table string
	var force bool

	cmd := &cobra.Command{
		Use:   "migration [name]",
		Short: "Generate a new migration",
		Long:  "Generate a versioned migration in app/Migrations, e.g. create_posts_table.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return generateMigration(args[0], table, force)
		},
	}

	cmd.Flags().StringVarP(&table, "table", "t", "", "Table the migration operates on")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite the file if it already exists")
	return cmd
}

const migrationTemplate = `package migrations

import (
	"database/sql"

	"github.com/sachinkaru123/gochin/pkg/orm"
)

func init() {
	orm.Register(orm.Migration{
		Version: "%[1]s",
		Name:    "%[2]s",
		Up: func(tx *sql.Tx) error {
			_, err := tx.Exec(` + "`" + `
				CREATE TABLE IF NOT EXISTS %[3]s (
					id SERIAL PRIMARY KEY,
					created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
					updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
				)
			` + "`" + `)
			return err
		},
		Down: func(tx *sql.Tx) error {
			_, err := tx.Exec(` + "`" + `DROP TABLE IF EXISTS %[3]s` + "`" + `)
			return err
		},
	})
}

// Remember an index for every column you filter or join on: a missing index
// turns an index scan into a sequential scan and dominates request latency.
//   CREATE INDEX idx_%[3]s_<column> ON %[3]s (<column>);
`

var migrationFilePattern = regexp.MustCompile(`^(\d{6})_.+\.go$`)

func generateMigration(name, table string, force bool) error {
	name = snakeCase(strings.ReplaceAll(strings.TrimSpace(name), " ", "_"))
	if name == "" {
		return fmt.Errorf("migration name is required")
	}

	if table == "" {
		table = guessTableFromMigration(name)
	}

	version, err := nextMigrationVersion()
	if err != nil {
		return err
	}

	content := fmt.Sprintf(migrationTemplate, version, name, table)

	path, err := writeGenerated(filepath.Join("app", "Migrations"), version+"_"+name+".go", content, force)
	if err != nil {
		return err
	}

	fmt.Printf("✅ Migration created: %s\n", path)
	fmt.Printf("   Version: %s  Table: %s\n", version, table)
	fmt.Println("   Apply it with: gochin db migrate")
	return nil
}

// nextMigrationVersion scans existing migrations and returns the next
// zero-padded version, so files sort lexically in apply order.
func nextMigrationVersion() (string, error) {
	dir := filepath.Join("app", "Migrations")

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "000001", nil
		}
		return "", fmt.Errorf("failed to read %s: %w", dir, err)
	}

	versions := make([]string, 0, len(entries))
	for _, e := range entries {
		if m := migrationFilePattern.FindStringSubmatch(e.Name()); m != nil {
			versions = append(versions, m[1])
		}
	}
	if len(versions) == 0 {
		return "000001", nil
	}

	sort.Strings(versions)
	last := versions[len(versions)-1]

	var n int
	if _, err := fmt.Sscanf(last, "%d", &n); err != nil {
		return "", fmt.Errorf("unrecognized migration version %q", last)
	}
	return fmt.Sprintf("%06d", n+1), nil
}

// guessTableFromMigration derives a table name from names shaped like
// create_posts_table or add_x_to_posts_table.
func guessTableFromMigration(name string) string {
	trimmed := strings.TrimSuffix(name, "_table")
	if rest, ok := strings.CutPrefix(trimmed, "create_"); ok {
		return rest
	}
	if i := strings.LastIndex(trimmed, "_to_"); i >= 0 {
		return trimmed[i+len("_to_"):]
	}
	if i := strings.LastIndex(trimmed, "_from_"); i >= 0 {
		return trimmed[i+len("_from_"):]
	}
	return trimmed
}

// --------------------------------------------------------------------- model

func newMakeModelCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "model [name]",
		Short: "Generate a new model",
		Long:  "Generate an ORM model in app/Models.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return generateModel(args[0], force)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite the file if it already exists")
	return cmd
}

const modelTemplate = `package models

import (
	"github.com/sachinkaru123/gochin/pkg/orm"
)

// %[1]s represents a row in the "%[2]s" table.
type %[1]s struct {
	orm.Model

	// TODO: add your columns here, e.g.
	// Name  string ` + "`db:\"name\" json:\"name\"`" + `
	// Email string ` + "`db:\"email\" json:\"email\"`" + `
}

// TableName returns the database table name for %[1]s.
//
// The value receiver matters: it lets %[1]s itself satisfy orm.Tabler, which
// is what makes orm.Find[%[1]s](id) compile.
func (%[1]s) TableName() string {
	return "%[2]s"
}

var _ orm.Tabler = %[1]s{}
`

func generateModel(name string, force bool) error {
	name = pascalCase(name)
	table := pluralize(snakeCase(name))

	content := fmt.Sprintf(modelTemplate, name, table)

	path, err := writeGenerated(filepath.Join("app", "Models"), name+".go", content, force)
	if err != nil {
		return err
	}

	fmt.Printf("✅ Model created: %s\n", path)
	fmt.Printf("   Table name: %s (edit TableName() if that is wrong)\n", table)
	return nil
}

// -------------------------------------------------------------------- seeder

func newMakeSeederCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "seeder [name]",
		Short: "Generate a new seeder",
		Long:  "Generate a seeder in app/Seeders that populates development data.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return generateSeeder(args[0], force)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite the file if it already exists")
	return cmd
}

const seederTemplate = `package seeders

import (
	"context"
	"errors"

	models "%[3]s/app/Models"
	"github.com/sachinkaru123/gochin/pkg/logs"
	"github.com/sachinkaru123/gochin/pkg/orm"
)

func init() {
	orm.RegisterSeeder(orm.Seeder{
		Name: "%[2]s",
		// Lower priority runs first; use it when one seeder needs another's rows.
		Priority: 100,
		Run:      seed%[1]s,
	})
}

// seed%[1]s populates the %[2]s table.
//
// Nothing records which seeders have run, so this must be safe to run more
// than once: check for the row before inserting it.
func seed%[1]s(ctx context.Context) error {
	records := []models.%[1]s{
		// TODO: describe your development rows here.
	}

	for i := range records {
		record := records[i]

		existing, err := orm.Query[models.%[1]s]().
			WithContext(ctx).
			Where("id", "=", record.ID).
			First()
		if err != nil && !errors.Is(err, orm.ErrRecordNotFound) {
			return err
		}
		if existing != nil {
			continue
		}

		if err := orm.CreateCtx(ctx, &record); err != nil {
			return err
		}
		logs.Info("seeded %[2]s", "id", record.ID)
	}

	return nil
}
`

func generateSeeder(name string, force bool) error {
	module, err := moduleName()
	if err != nil {
		return err
	}

	name = pascalCase(strings.TrimSuffix(pascalCase(name), "Seeder"))
	table := pluralize(snakeCase(name))

	content := fmt.Sprintf(seederTemplate, name, table, module)

	path, err := writeGenerated(filepath.Join("app", "Seeders"), name+"Seeder.go", content, force)
	if err != nil {
		return err
	}

	fmt.Printf("✅ Seeder created: %s\n", path)
	fmt.Println("   Run it with: gochin db seed --only=" + table)
	return nil
}
