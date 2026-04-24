package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	wso2seagent "github.com/Tharsanan1/wso2-se-agent"
	"github.com/Tharsanan1/wso2-se-agent/internal/config"
)

// gatherAvailableProducts returns every (product, version) pair the CLI knows
// about, taken as the UNION of the local config dir and the embedded FS. This
// is wider than config.ListProducts / ListVersions, which prefer local over
// embedded and therefore hide embedded versions the moment any local product
// dir exists — confusing behaviour for a "what's available" query.
func gatherAvailableProducts() (map[string][]string, error) {
	result := map[string]map[string]struct{}{} // product -> set of versions

	add := func(product, version string) {
		if _, ok := result[product]; !ok {
			result[product] = map[string]struct{}{}
		}
		result[product][version] = struct{}{}
	}

	// Local
	if configDir, err := config.GetConfigDir(); err == nil {
		local := filepath.Join(configDir, "products")
		if pEntries, err := os.ReadDir(local); err == nil {
			for _, pe := range pEntries {
				if !pe.IsDir() {
					continue
				}
				vEntries, err := os.ReadDir(filepath.Join(local, pe.Name()))
				if err != nil {
					continue
				}
				for _, ve := range vEntries {
					if ve.IsDir() {
						add(pe.Name(), ve.Name())
					}
				}
			}
		}
	}

	// Embedded
	if pEntries, err := wso2seagent.ProductsFS.ReadDir("products"); err == nil {
		for _, pe := range pEntries {
			if !pe.IsDir() {
				continue
			}
			vEntries, err := wso2seagent.ProductsFS.ReadDir("products/" + pe.Name())
			if err != nil {
				continue
			}
			for _, ve := range vEntries {
				if ve.IsDir() {
					add(pe.Name(), ve.Name())
				}
			}
		}
	}

	// Flatten + sort for stable output.
	out := map[string][]string{}
	for product, versions := range result {
		list := make([]string, 0, len(versions))
		for v := range versions {
			list = append(list, v)
		}
		sort.Strings(list)
		out[product] = list
	}
	return out, nil
}

// listCmd is a grouping parent for `list <thing>` queries. Keeping it a group
// leaves room to add `list workspaces`, `list repos`, etc. without stealing a
// top-level verb.
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured resources (products, etc.)",
}

var listProductsCmd = &cobra.Command{
	Use:   "products",
	Short: "List available products with their versions and setup status",
	Long: `Lists each product/version known to the CLI, where its config came from
(local vs embedded), and how many of its repos have been registered via
setup-repos. Pure read — no side effects.`,
	RunE: runListProducts,
}

func init() {
	listCmd.AddCommand(listProductsCmd)
}

type productRow struct {
	Product     string
	Version     string
	ConfigSrc   string // "local" or "embedded"
	ReposReady  int
	ReposTotal  int
	SkillsRef   string
	MissingRepo string // first unregistered repo, or "" if all registered
}

func (r productRow) StatusSymbol() string {
	switch {
	case r.ReposTotal == 0:
		return "-"
	case r.ReposReady == r.ReposTotal:
		return "✓"
	default:
		return "⚠"
	}
}

func runListProducts(cmd *cobra.Command, args []string) error {
	productVersions, err := gatherAvailableProducts()
	if err != nil || len(productVersions) == 0 {
		fmt.Println("No products available.")
		fmt.Println("Run: wso2-se-agent config init")
		return nil
	}
	products := make([]string, 0, len(productVersions))
	for p := range productVersions {
		products = append(products, p)
	}
	sort.Strings(products)

	// Load repo registry (may be empty or missing — both are fine).
	registry, _ := config.LoadRepoRegistry()
	if registry == nil || registry.Repos == nil {
		registry = &config.RepoRegistry{Repos: map[string]config.RepoEntry{}}
	}

	// Is <configdir>/products/<p>/<v>/product-config.yaml present? Distinguishes
	// "local" (copied by `config init`, editable) from "embedded" (still only
	// in the binary's read-only FS).
	configDir, _ := config.GetConfigDir()
	localProductPath := func(p, v string) string {
		return filepath.Join(configDir, "products", p, v, "product-config.yaml")
	}

	var rows []productRow
	for _, product := range products {
		versions := productVersions[product]

		for _, version := range versions {
			pc, err := config.LoadProductConfig(product, version)
			if err != nil {
				// Surface broken configs in-line rather than silently skipping.
				rows = append(rows, productRow{
					Product: product, Version: version, ConfigSrc: "error",
					MissingRepo: err.Error(),
				})
				continue
			}

			src := "embedded"
			if _, statErr := os.Stat(localProductPath(product, version)); statErr == nil {
				src = "local"
			}

			ready, total := 0, len(pc.Repos)
			firstMissing := ""
			for _, repo := range pc.Repos {
				if _, ok := registry.Repos[repo.Name]; ok {
					ready++
				} else if firstMissing == "" {
					firstMissing = repo.Name
				}
			}

			rows = append(rows, productRow{
				Product:     product,
				Version:     version,
				ConfigSrc:   src,
				ReposReady:  ready,
				ReposTotal:  total,
				SkillsRef:   pc.SkillsRef,
				MissingRepo: firstMissing,
			})
		}
	}

	// Render as an aligned table. One row per product/version.
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PRODUCT\tVERSION\tCONFIG\tREPOS\tSKILLS_REF\tSTATUS")
	for _, r := range rows {
		status := r.StatusSymbol()
		hint := ""
		switch {
		case r.ConfigSrc == "error":
			hint = "  " + r.MissingRepo
		case r.ReposTotal == 0:
			hint = ""
		case r.ReposReady < r.ReposTotal:
			hint = fmt.Sprintf(
				"  run: wso2-se-agent setup-repos --product %s --version %s",
				r.Product, r.Version)
			if r.MissingRepo != "" {
				hint += fmt.Sprintf("  (missing: %s)", r.MissingRepo)
			}
		}
		reposCol := "-"
		if r.ReposTotal > 0 {
			reposCol = fmt.Sprintf("%d/%d", r.ReposReady, r.ReposTotal)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s%s\n",
			r.Product, r.Version, r.ConfigSrc, reposCol, truncate(r.SkillsRef, 32), status, hint)
	}
	_ = w.Flush()

	// Footer hint when nothing is set up yet.
	allEmbedded := true
	anyRegistered := false
	for _, r := range rows {
		if r.ConfigSrc == "local" {
			allEmbedded = false
		}
		if r.ReposReady > 0 {
			anyRegistered = true
		}
	}
	if allEmbedded {
		fmt.Println()
		fmt.Println("All products are embedded (read-only). Run `wso2-se-agent config init` to copy them locally so you can edit.")
	} else if !anyRegistered {
		fmt.Println()
		fmt.Println("No product has any repos registered yet. Run `wso2-se-agent setup-repos --product <name> --version <version>` for the one you plan to use.")
	}

	return nil
}

func truncate(s string, n int) string {
	if s == "" {
		return "-"
	}
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

