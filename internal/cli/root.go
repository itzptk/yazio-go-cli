package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"
	"github.com/itzptk/yazio-go-cli/internal/config"
	"github.com/itzptk/yazio-go-cli/internal/yazio"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type App struct {
	out           io.Writer
	v             *viper.Viper
	cfgPath       string
	cfg           config.File
	baseURL       string
	output        string
	clientFactory func(string) apiClient
}

type apiClient interface {
	Login(context.Context, yazio.Credentials) (yazio.Token, error)
	Refresh(context.Context, yazio.Token) (yazio.Token, error)
	GetUser(context.Context, yazio.Token) (yazio.User, error)
	GetDailySummary(context.Context, yazio.Token, time.Time) (yazio.DailySummary, error)
	GetConsumedItems(context.Context, yazio.Token, time.Time) (yazio.ConsumedItemsResponse, error)
	SearchProducts(context.Context, yazio.Token, yazio.SearchOptions) ([]yazio.ProductSearchResult, error)
	AddConsumedItem(context.Context, yazio.Token, yazio.AddConsumedItemRequest) error
	RemoveConsumedItem(context.Context, yazio.Token, string) error
}

func NewRootCommand(out io.Writer, version string) (*cobra.Command, error) {
	return newRootCommand(out, version, func(baseURL string) apiClient {
		return yazio.NewClient(baseURL)
	})
}

func newRootCommand(out io.Writer, version string, clientFactory func(string) apiClient) (*cobra.Command, error) {
	defaultConfigPath, err := config.DefaultPath()
	if err != nil {
		return nil, err
	}

	app := &App{
		out:           out,
		v:             viper.New(),
		clientFactory: clientFactory,
	}
	app.v.SetEnvPrefix("YAZIO")
	app.v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	app.v.AutomaticEnv()

	cmd := &cobra.Command{
		Use:     "yazio",
		Short:   "Interact with the unofficial YAZIO API from a Go CLI",
		Version: version,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cfgPath := app.v.GetString("config")
			if cfgPath == "" {
				cfgPath = defaultConfigPath
			}
			app.cfgPath = cfgPath

			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}

			if envBaseURL, ok := os.LookupEnv("YAZIO_BASE_URL"); ok {
				cfg.BaseURL = envBaseURL
			}
			if flag := cmd.Root().PersistentFlags().Lookup("base-url"); flag != nil && flag.Changed {
				cfg.BaseURL = flag.Value.String()
			}
			if app.v.IsSet("output") {
				cfg.Output = app.v.GetString("output")
			}
			baseURL, err := config.NormalizeBaseURL(cfg.BaseURL)
			if err != nil {
				return err
			}
			output, err := config.NormalizeOutput(cfg.Output)
			if err != nil {
				return err
			}
			cfg.BaseURL = baseURL
			cfg.Output = output
			app.cfg = cfg
			app.baseURL = cfg.BaseURL
			app.output = cfg.Output
			return nil
		},
	}

	cmd.PersistentFlags().String("config", defaultConfigPath, "Path to config file")
	cmd.PersistentFlags().String("base-url", config.DefaultBaseURL, "YAZIO API base URL")
	cmd.PersistentFlags().String("output", config.DefaultOutput, "Output format: table or json")
	_ = app.v.BindPFlag("config", cmd.PersistentFlags().Lookup("config"))
	_ = app.v.BindPFlag("base_url", cmd.PersistentFlags().Lookup("base-url"))
	_ = app.v.BindPFlag("output", cmd.PersistentFlags().Lookup("output"))

	cmd.AddCommand(app.newAuthCommand())
	cmd.AddCommand(app.newConfigCommand())
	cmd.AddCommand(app.newUserCommand())
	cmd.AddCommand(app.newSummaryCommand())
	cmd.AddCommand(app.newConsumedCommand())
	cmd.AddCommand(app.newExportCommand())
	cmd.AddCommand(app.newSearchCommand())
	cmd.AddCommand(app.newAddCommand())
	cmd.AddCommand(app.newRemoveCommand())

	return cmd, nil
}

func (a *App) newAuthCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "auth", Short: "Authentication helpers"}
	cmd.AddCommand(a.newLoginCommand())
	cmd.AddCommand(a.newAuthStatusCommand())
	return cmd
}

func (a *App) newConfigCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Config file helpers"}
	cmd.AddCommand(a.newConfigInitCommand())
	return cmd
}

func (a *App) newConfigInitCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Write an example config file to the resolved config path",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				if _, err := os.Stat(a.cfgPath); err == nil {
					return fmt.Errorf("config file already exists at %s (use --force to overwrite)", a.cfgPath)
				} else if !errors.Is(err, os.ErrNotExist) {
					return err
				}
			}
			if err := os.MkdirAll(filepath.Dir(a.cfgPath), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(a.cfgPath, []byte(config.ExampleFile), 0o600); err != nil {
				return err
			}
			fmt.Fprintf(a.out, "wrote example config to %s\n", a.cfgPath)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite an existing config file")
	return cmd
}

func (a *App) newLoginCommand() *cobra.Command {
	var email string
	var password string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Exchange email/password for an API token and save it locally",
		RunE: func(cmd *cobra.Command, args []string) error {
			if email == "" {
				email = a.v.GetString("email")
			}
			if password == "" {
				password = a.v.GetString("password")
			}
			if email == "" || password == "" {
				return errors.New("both --email and --password are required (or YAZIO_EMAIL / YAZIO_PASSWORD env vars)")
			}

			token, err := a.client().Login(cmd.Context(), yazio.Credentials{Email: email, Password: password})
			if err != nil {
				return err
			}
			a.cfg.Token = toConfigToken(token)
			if err := config.Save(a.cfgPath, a.cfg); err != nil {
				return err
			}
			fmt.Fprintf(a.out, "login successful; token saved to %s\n", a.cfgPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "YAZIO account email")
	cmd.Flags().StringVar(&password, "password", "", "YAZIO account password")
	_ = a.v.BindEnv("email")
	_ = a.v.BindEnv("password")
	return cmd
}

func (a *App) newAuthStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show stored authentication status",
		RunE: func(cmd *cobra.Command, args []string) error {
			if a.cfg.Token == nil {
				fmt.Fprintf(a.out, "not logged in; config path: %s\n", a.cfgPath)
				return nil
			}
			token := fromConfigToken(*a.cfg.Token)
			state := "valid"
			if token.Expired(time.Now()) {
				state = "expired"
			}
			fmt.Fprintf(a.out, "status: %s\nexpires: %s\nconfig: %s\n", state, token.ExpiresAt.Format(time.RFC3339), a.cfgPath)
			return nil
		},
	}
}

func (a *App) newUserCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "user", Short: "User profile commands"}
	cmd.AddCommand(&cobra.Command{
		Use:   "profile",
		Short: "Fetch the current user profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			token, err := a.ensureToken(cmd.Context())
			if err != nil {
				return err
			}
			profile, err := a.client().GetUser(cmd.Context(), token)
			if err != nil {
				return err
			}
			if a.output == "json" {
				return writeJSON(a.out, profile)
			}
			fmt.Fprintf(a.out, "Name: %s %s\nEmail: %s\nGoal: %s\nCountry: %s\nPremium: %s\nUnits: %s / %s\nRegistered: %s\n", profile.FirstName, profile.LastName, profile.Email, profile.Goal, profile.Country, profile.PremiumType, profile.UnitMass, profile.UnitEnergy, profile.RegistrationDate)
			return nil
		},
	})
	return cmd
}

func (a *App) newSummaryCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "summary [YYYY-MM-DD]",
		Short: "Fetch the daily summary",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			date, err := parseDateArg(args)
			if err != nil {
				return err
			}
			token, err := a.ensureToken(cmd.Context())
			if err != nil {
				return err
			}
			summary, err := a.client().GetDailySummary(cmd.Context(), token, date)
			if err != nil {
				return err
			}
			if a.output == "json" {
				return writeJSON(a.out, summary)
			}
			fmt.Fprintf(a.out, "Date: %s\nCalories: %.1f/%0.1f %s\nProtein: %.1f/%0.1f g\nCarbs: %.1f/%0.1f g\nFat: %.1f/%0.1f g\nWater: %d/%0.1f ml\nSteps: %d/%0.1f\n",
				date.Format("2006-01-02"),
				summary.Meals.Breakfast.Nutrients.Energy+summary.Meals.Lunch.Nutrients.Energy+summary.Meals.Dinner.Nutrients.Energy+summary.Meals.Snack.Nutrients.Energy,
				summary.Goals.Energy,
				summary.Units.Energy,
				summary.Meals.Breakfast.Nutrients.Protein+summary.Meals.Lunch.Nutrients.Protein+summary.Meals.Dinner.Nutrients.Protein+summary.Meals.Snack.Nutrients.Protein,
				summary.Goals.Protein,
				summary.Meals.Breakfast.Nutrients.Carb+summary.Meals.Lunch.Nutrients.Carb+summary.Meals.Dinner.Nutrients.Carb+summary.Meals.Snack.Nutrients.Carb,
				summary.Goals.Carb,
				summary.Meals.Breakfast.Nutrients.Fat+summary.Meals.Lunch.Nutrients.Fat+summary.Meals.Dinner.Nutrients.Fat+summary.Meals.Snack.Nutrients.Fat,
				summary.Goals.Fat,
				summary.WaterIntake,
				summary.Goals.Water,
				summary.Steps,
				summary.Goals.Steps,
			)
			return nil
		},
	}
}

func (a *App) newConsumedCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "consumed [YYYY-MM-DD]",
		Short: "List diary entries for a date",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			date, err := parseDateArg(args)
			if err != nil {
				return err
			}
			token, err := a.ensureToken(cmd.Context())
			if err != nil {
				return err
			}
			items, err := a.client().GetConsumedItems(cmd.Context(), token, date)
			if err != nil {
				return err
			}
			if a.output == "json" {
				return writeJSON(a.out, items)
			}
			return writeConsumedTable(a.out, items)
		},
	}
}

func writeConsumedTable(out io.Writer, items yazio.ConsumedItemsResponse) error {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ENTRY ID\tMEAL\tPRODUCT ID\tAMOUNT\tSERVING")
	for _, item := range items.Products {
		serving := ""
		if item.Serving != nil {
			serving = *item.Serving
		}
		writeConsumedTableRow(w, item.ID, item.Daytime, item.ProductID, fmt.Sprintf("%.2f", item.Amount), serving)
	}
	for _, item := range items.RecipePortions {
		writeGenericConsumedTableRow(w, item)
	}
	for _, item := range items.SimpleProducts {
		writeGenericConsumedTableRow(w, item)
	}
	return w.Flush()
}

func writeGenericConsumedTableRow(w io.Writer, item any) {
	fields := mapFields(item)
	writeConsumedTableRow(
		w,
		fieldValue(fields, "id", "entry_id", "uuid"),
		fieldValue(fields, "daytime", "meal"),
		fieldValue(fields, "product_id", "recipe_id", "simple_product_id"),
		tableAmount(fields),
		fieldValue(fields, "serving"),
	)
}

func writeConsumedTableRow(w io.Writer, id, meal, productID, amount, serving string) {
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", id, meal, productID, amount, serving)
}

func tableAmount(fields map[string]any) string {
	value, ok := fields["amount"]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case float64:
		return fmt.Sprintf("%.2f", typed)
	case float32:
		return fmt.Sprintf("%.2f", typed)
	case int:
		return fmt.Sprintf("%.2f", float64(typed))
	case int64:
		return fmt.Sprintf("%.2f", float64(typed))
	case int32:
		return fmt.Sprintf("%.2f", float64(typed))
	case uint:
		return fmt.Sprintf("%.2f", float64(typed))
	case uint64:
		return fmt.Sprintf("%.2f", float64(typed))
	case uint32:
		return fmt.Sprintf("%.2f", float64(typed))
	case string:
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64); err == nil {
			return fmt.Sprintf("%.2f", parsed)
		}
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

func (a *App) newSearchCommand() *cobra.Command {
	var sex string
	var countries []string
	var locales []string

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search products in the YAZIO database",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			token, err := a.ensureToken(cmd.Context())
			if err != nil {
				return err
			}
			results, err := a.client().SearchProducts(cmd.Context(), token, yazio.SearchOptions{
				Query:     args[0],
				Sex:       sex,
				Countries: countries,
				Locales:   locales,
			})
			if err != nil {
				return err
			}
			if a.output == "json" {
				return writeJSON(a.out, results)
			}
			w := tabwriter.NewWriter(a.out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "PRODUCT ID\tNAME\tAMOUNT\tSERVING\tKCAL\tP\tC\tF")
			for _, result := range results {
				fmt.Fprintf(w, "%s\t%s\t%.0f\t%s\t%.1f\t%.1f\t%.1f\t%.1f\n", result.ProductID, result.Name, result.Amount, result.Serving, result.Nutrients.Energy, result.Nutrients.Protein, result.Nutrients.Carb, result.Nutrients.Fat)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&sex, "sex", "male", "Profile sex to use for search ranking")
	cmd.Flags().StringSliceVar(&countries, "countries", []string{"DE", "US"}, "Country codes for search")
	cmd.Flags().StringSliceVar(&locales, "locales", []string{"en_US", "de_US"}, "Locales for search")
	return cmd
}

func (a *App) newAddCommand() *cobra.Command {
	var productID string
	var meal string
	var amount float64
	var serving string
	var servingQuantity float64
	var dateValue string

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a consumed product to the diary",
		RunE: func(cmd *cobra.Command, args []string) error {
			if productID == "" || meal == "" || amount <= 0 {
				return errors.New("--product-id, --meal, and --amount are required")
			}
			if serving != "" && servingQuantity <= 0 {
				return errors.New("--serving-quantity must be greater than zero when --serving is set")
			}
			date, err := parseDateFlag(dateValue)
			if err != nil {
				return err
			}
			token, err := a.ensureToken(cmd.Context())
			if err != nil {
				return err
			}

			var servingPtr *string
			var quantityPtr *float64
			if serving != "" {
				servingPtr = &serving
				quantityPtr = &servingQuantity
			}

			entryID := uuid.NewString()
			if err := a.client().AddConsumedItem(cmd.Context(), token, yazio.AddConsumedItemRequest{
				ID:              entryID,
				ProductID:       productID,
				Date:            date,
				Daytime:         meal,
				Amount:          amount,
				Serving:         servingPtr,
				ServingQuantity: quantityPtr,
			}); err != nil {
				return err
			}
			fmt.Fprintf(a.out, "added entry %s\n", entryID)
			return nil
		},
	}
	cmd.Flags().StringVar(&productID, "product-id", "", "Product ID returned by search")
	cmd.Flags().StringVar(&meal, "meal", "", "Meal bucket: breakfast, lunch, dinner, snack")
	cmd.Flags().Float64Var(&amount, "amount", 0, "Amount of the serving/base unit")
	cmd.Flags().StringVar(&serving, "serving", "", "Serving unit, e.g. g")
	cmd.Flags().Float64Var(&servingQuantity, "serving-quantity", 1, "Serving quantity multiplier")
	cmd.Flags().StringVar(&dateValue, "date", time.Now().UTC().Format("2006-01-02"), "Diary date YYYY-MM-DD")
	return cmd
}

func (a *App) newRemoveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <entry-id>",
		Short: "Remove a consumed diary entry by entry ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			entryID := args[0]
			if _, err := uuid.Parse(entryID); err != nil {
				return fmt.Errorf("invalid entry ID %q: expected UUID", entryID)
			}
			token, err := a.ensureToken(cmd.Context())
			if err != nil {
				return err
			}
			if err := a.client().RemoveConsumedItem(cmd.Context(), token, entryID); err != nil {
				return err
			}
			fmt.Fprintf(a.out, "removed entry %s\n", entryID)
			return nil
		},
	}
}

func (a *App) client() apiClient {
	return a.clientFactory(a.baseURL)
}

func (a *App) ensureToken(ctx context.Context) (yazio.Token, error) {
	if a.cfg.Token == nil {
		return yazio.Token{}, errors.New("not logged in; run `yazio auth login` first")
	}
	token := fromConfigToken(*a.cfg.Token)
	if !token.Expired(time.Now()) {
		return token, nil
	}
	if token.RefreshToken == "" {
		return yazio.Token{}, errors.New("stored token expired and no refresh token is available; run `yazio auth login` again")
	}
	refreshed, err := a.client().Refresh(ctx, token)
	if err != nil {
		return yazio.Token{}, err
	}
	a.cfg.Token = toConfigToken(refreshed)
	if err := config.Save(a.cfgPath, a.cfg); err != nil {
		return yazio.Token{}, err
	}
	return refreshed, nil
}

func toConfigToken(token yazio.Token) *config.Token {
	return &config.Token{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenType:    token.TokenType,
		ExpiresAt:    token.ExpiresAt,
	}
}

func fromConfigToken(token config.Token) yazio.Token {
	return yazio.Token{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenType:    token.TokenType,
		ExpiresAt:    token.ExpiresAt,
	}
}

func parseDateArg(args []string) (time.Time, error) {
	if len(args) == 0 {
		return time.Now().UTC(), nil
	}
	return parseDateFlag(args[0])
}

func parseDateFlag(value string) (time.Time, error) {
	date, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q: %w", value, err)
	}
	return date.UTC(), nil
}

func writeJSON(out io.Writer, value any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
