package yazio

import "time"

type Credentials struct {
	Email    string
	Password string
}

type Token struct {
	AccessToken  string    `json:"access_token" yaml:"access_token"`
	RefreshToken string    `json:"refresh_token" yaml:"refresh_token"`
	TokenType    string    `json:"token_type" yaml:"token_type"`
	ExpiresAt    time.Time `json:"expires_at" yaml:"expires_at"`
}

func (t Token) Expired(now time.Time) bool {
	if t.ExpiresAt.IsZero() {
		return false
	}
	return !now.Before(t.ExpiresAt.Add(-30 * time.Second))
}

type User struct {
	Email               string  `json:"email"`
	PremiumType         string  `json:"premium_type"`
	Sex                 string  `json:"sex"`
	FirstName           string  `json:"first_name"`
	LastName            string  `json:"last_name"`
	City                string  `json:"city"`
	Country             string  `json:"country"`
	Goal                string  `json:"goal"`
	RegistrationDate    string  `json:"registration_date"`
	TimezoneOffset      int     `json:"timezone_offset"`
	UnitLength          string  `json:"unit_length"`
	UnitMass            string  `json:"unit_mass"`
	UnitServing         string  `json:"unit_serving"`
	UnitEnergy          string  `json:"unit_energy"`
	FoodDatabaseCountry string  `json:"food_database_country"`
	ProfileImage        string  `json:"profile_image"`
	UserToken           string  `json:"user_token"`
	StartWeight         float64 `json:"start_weight"`
	EmailConfirmation   string  `json:"email_confirmation_status"`
	NewsletterOptIn     bool    `json:"newsletter_opt_in"`
	LoginType           string  `json:"login_type"`
	UUID                string  `json:"uuid"`
	ActivityDegree      string  `json:"activity_degree"`
	DateOfBirth         string  `json:"date_of_birth"`
	WeightChangePerWeek float64 `json:"weight_change_per_week"`
	BodyHeight          float64 `json:"body_height"`
	SIWAUserID          *string `json:"siwa_user_id"`
	ResetDate           *string `json:"reset_date"`
	StripeCustomerID    *string `json:"stripe_customer_id"`
}

type BasicNutrients struct {
	Energy  float64 `json:"energy.energy"`
	Carb    float64 `json:"nutrient.carb"`
	Protein float64 `json:"nutrient.protein"`
	Fat     float64 `json:"nutrient.fat"`
}

type MealSummary struct {
	EnergyGoal float64        `json:"energy_goal"`
	Nutrients  BasicNutrients `json:"nutrients"`
}

type Meals struct {
	Breakfast MealSummary `json:"breakfast"`
	Lunch     MealSummary `json:"lunch"`
	Dinner    MealSummary `json:"dinner"`
	Snack     MealSummary `json:"snack"`
}

type Goals struct {
	Energy  float64 `json:"energy.energy"`
	Water   float64 `json:"water"`
	Steps   float64 `json:"activity.step"`
	Protein float64 `json:"nutrient.protein"`
	Fat     float64 `json:"nutrient.fat"`
	Carb    float64 `json:"nutrient.carb"`
	Weight  float64 `json:"bodyvalue.weight"`
}

type Units struct {
	Mass    string `json:"unit_mass"`
	Energy  string `json:"unit_energy"`
	Serving string `json:"unit_serving"`
	Length  string `json:"unit_length"`
}

type SummaryUser struct {
	StartWeight   *float64 `json:"start_weight"`
	CurrentWeight *float64 `json:"current_weight"`
	Goal          *string  `json:"goal"`
	Sex           *string  `json:"sex"`
}

type DailySummary struct {
	ActivityEnergy            float64     `json:"activity_energy"`
	ConsumeActivityEnergy     bool        `json:"consume_activity_energy"`
	Steps                     int         `json:"steps"`
	WaterIntake               int         `json:"water_intake"`
	Goals                     Goals       `json:"goals"`
	Units                     Units       `json:"units"`
	Meals                     Meals       `json:"meals"`
	User                      SummaryUser `json:"user"`
	ActiveFastingCountdownKey *string     `json:"active_fasting_countdown_template_key"`
}

type SearchOptions struct {
	Query     string
	Sex       string
	Countries []string
	Locales   []string
}

type ProductSearchResult struct {
	Score           float64        `json:"score"`
	Name            string         `json:"name"`
	ProductID       string         `json:"product_id"`
	Serving         string         `json:"serving"`
	ServingQuantity float64        `json:"serving_quantity"`
	Amount          float64        `json:"amount"`
	BaseUnit        string         `json:"base_unit"`
	Producer        string         `json:"producer"`
	IsVerified      bool           `json:"is_verified"`
	Nutrients       BasicNutrients `json:"nutrients"`
	Countries       []string       `json:"countries"`
	Language        string         `json:"language"`
}

type ConsumedItem struct {
	ID              string   `json:"id"`
	Date            string   `json:"date"`
	Daytime         string   `json:"daytime"`
	Type            string   `json:"type"`
	ProductID       string   `json:"product_id"`
	Amount          float64  `json:"amount"`
	Serving         *string  `json:"serving"`
	ServingQuantity *float64 `json:"serving_quantity"`
}

type ConsumedItemsResponse struct {
	Products       []ConsumedItem `json:"products"`
	RecipePortions []any          `json:"recipe_portions"`
	SimpleProducts []any          `json:"simple_products"`
}

type AddConsumedItemRequest struct {
	ID              string
	ProductID       string
	Date            time.Time
	Daytime         string
	Amount          float64
	Serving         *string
	ServingQuantity *float64
}
