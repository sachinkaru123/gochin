package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/gochin/framework/internal/server"
	"github.com/gochin/framework/pkg/config"
)

// NewRouteCommand creates the route command with its subcommands.
func NewRouteCommand() *cobra.Command {
	routeCmd := &cobra.Command{
		Use:   "route",
		Short: "Inspect application routes",
		Long:  "Inspect the routes registered by the files in app/Routes.",
	}

	routeCmd.AddCommand(newRouteListCommand())
	return routeCmd
}

func newRouteListCommand() *cobra.Command {
	var showAll, asJSON, methodFilter, pathFilter = false, false, "", ""

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all registered routes",
		Long:  "List every route registered by app/Routes. Does not require a database connection.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listRoutes(showAll, asJSON, methodFilter, pathFilter)
		},
	}

	cmd.Flags().BoolVarP(&showAll, "all", "a", false, "Include synthesized 405/OPTIONS routes")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output as JSON")
	cmd.Flags().StringVarP(&methodFilter, "method", "m", "", "Only show routes for this HTTP method")
	cmd.Flags().StringVarP(&pathFilter, "path", "p", "", "Only show routes whose path contains this substring")

	return cmd
}

func listRoutes(showAll, asJSON bool, methodFilter, pathFilter string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Building the router runs every registrar. It must not need a database:
	// that constraint is what keeps service constructors lazy.
	r, err := server.BuildRouter(cfg)
	if err != nil {
		return fmt.Errorf("failed to build router: %w", err)
	}

	routes := r.Routes(showAll)

	if methodFilter != "" {
		methodFilter = strings.ToUpper(methodFilter)
		filtered := routes[:0]
		for _, rt := range routes {
			if rt.Method == methodFilter {
				filtered = append(filtered, rt)
			}
		}
		routes = filtered
	}
	if pathFilter != "" {
		filtered := routes[:0]
		for _, rt := range routes {
			if strings.Contains(rt.Path, pathFilter) {
				filtered = append(filtered, rt)
			}
		}
		routes = filtered
	}

	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(routes)
	}

	if len(routes) == 0 {
		fmt.Println("No routes registered. Add a route file under app/Routes.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "METHOD\tPATH\tHANDLER\tMIDDLEWARE")
	fmt.Fprintln(w, "------\t----\t-------\t----------")

	for _, rt := range routes {
		mw := "-"
		if len(rt.Middleware) > 0 {
			mw = strings.Join(rt.Middleware, ", ")
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", rt.Method, rt.Path, rt.Handler, mw)
	}

	if err := w.Flush(); err != nil {
		return err
	}

	fmt.Printf("\n%d route(s)\n", len(routes))
	return nil
}
