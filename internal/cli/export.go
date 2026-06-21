package cli

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/itzptk/yazio-go-cli/internal/yazio"
	"github.com/spf13/cobra"
)

func (a *App) newExportCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export YAZIO diary data",
	}
	cmd.AddCommand(a.newExportDiaryCommand())
	return cmd
}

func (a *App) newExportDiaryCommand() *cobra.Command {
	var fromValue string
	var toValue string
	var filePath string

	cmd := &cobra.Command{
		Use:   "diary [YYYY-MM-DD]",
		Short: "Export consumed diary entries as CSV",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dates, err := resolveDiaryExportDates(args, fromValue, toValue)
			if err != nil {
				return err
			}
			token, err := a.ensureToken(cmd.Context())
			if err != nil {
				return err
			}

			client := a.client()
			days := make([]datedConsumedItems, 0, len(dates))
			for _, date := range dates {
				items, err := client.GetConsumedItems(cmd.Context(), token, date)
				if err != nil {
					return err
				}
				days = append(days, datedConsumedItems{Date: date, Items: items})
			}

			var csvOutput bytes.Buffer
			count, err := writeDiaryCSV(&csvOutput, days)
			if err != nil {
				return err
			}

			if filePath == "" {
				_, err := a.out.Write(csvOutput.Bytes())
				return err
			}
			if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
				return err
			}
			if err := writePrivateFile(filePath, csvOutput.Bytes()); err != nil {
				return err
			}
			_, err = fmt.Fprintf(a.out, "exported %d diary entries to %s\n", count, filePath)
			return err
		},
	}
	cmd.Flags().StringVar(&fromValue, "from", "", "Start date YYYY-MM-DD for an inclusive export range")
	cmd.Flags().StringVar(&toValue, "to", "", "End date YYYY-MM-DD for an inclusive export range")
	cmd.Flags().StringVar(&filePath, "file", "", "Write CSV to a file instead of stdout")
	return cmd
}

func writePrivateFile(path string, content []byte) (err error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
	}()

	if err := file.Chmod(0o600); err != nil {
		return err
	}
	if err := file.Truncate(0); err != nil {
		return err
	}
	if _, err := file.Seek(0, 0); err != nil {
		return err
	}
	n, err := file.Write(content)
	if err != nil {
		return err
	}
	if n != len(content) {
		return io.ErrShortWrite
	}
	return nil
}

type datedConsumedItems struct {
	Date  time.Time
	Items yazio.ConsumedItemsResponse
}

const maxDiaryExportDays = 366

func resolveDiaryExportDates(args []string, fromValue, toValue string) ([]time.Time, error) {
	if len(args) > 0 && (fromValue != "" || toValue != "") {
		return nil, errors.New("pass either a positional date or --from/--to, not both")
	}

	if len(args) == 1 {
		date, err := parseDateFlag(args[0])
		if err != nil {
			return nil, err
		}
		return []time.Time{date}, nil
	}

	if fromValue == "" && toValue == "" {
		return []time.Time{time.Now().UTC()}, nil
	}
	if fromValue == "" {
		fromValue = toValue
	}
	if toValue == "" {
		toValue = fromValue
	}

	from, err := parseDateFlag(fromValue)
	if err != nil {
		return nil, fmt.Errorf("invalid --from: %w", err)
	}
	to, err := parseDateFlag(toValue)
	if err != nil {
		return nil, fmt.Errorf("invalid --to: %w", err)
	}
	if from.After(to) {
		return nil, errors.New("--from must be on or before --to")
	}

	dates := make([]time.Time, 0, int(to.Sub(from).Hours()/24)+1)
	for date := from; !date.After(to); date = date.AddDate(0, 0, 1) {
		if len(dates) == maxDiaryExportDays {
			return nil, errors.New("diary export range cannot exceed 366 days")
		}
		dates = append(dates, date)
	}
	return dates, nil
}

func writeDiaryCSV(out io.Writer, days []datedConsumedItems) (int, error) {
	writer := csv.NewWriter(out)
	if err := writer.Write([]string{"date", "category", "meal", "entry_id", "type", "product_id", "name", "producer", "amount", "serving", "serving_quantity", "raw_json"}); err != nil {
		return 0, err
	}

	count := 0
	for _, day := range days {
		for _, item := range day.Items.Products {
			if err := writer.Write(productDiaryCSVRecord(day.Date, item)); err != nil {
				return count, err
			}
			count++
		}
		for _, item := range day.Items.RecipePortions {
			if err := writer.Write(genericDiaryCSVRecord(day.Date, "recipe_portion", item)); err != nil {
				return count, err
			}
			count++
		}
		for _, item := range day.Items.SimpleProducts {
			if err := writer.Write(genericDiaryCSVRecord(day.Date, "simple_product", item)); err != nil {
				return count, err
			}
			count++
		}
	}
	writer.Flush()
	return count, writer.Error()
}

func productDiaryCSVRecord(date time.Time, item yazio.ConsumedItem) []string {
	entryDate := item.Date
	if entryDate == "" {
		entryDate = date.Format("2006-01-02")
	}
	entryType := item.Type
	if entryType == "" {
		entryType = "product"
	}

	serving := ""
	if item.Serving != nil {
		serving = *item.Serving
	}
	servingQuantity := ""
	if item.ServingQuantity != nil {
		servingQuantity = formatCSVFloat(*item.ServingQuantity)
	}

	return []string{
		entryDate,
		"product",
		item.Daytime,
		item.ID,
		entryType,
		item.ProductID,
		item.Name,
		item.Producer,
		formatCSVFloat(item.Amount),
		serving,
		servingQuantity,
		"",
	}
}

func genericDiaryCSVRecord(date time.Time, category string, item any) []string {
	fields := mapFields(item)
	entryDate := fieldValue(fields, "date")
	if entryDate == "" {
		entryDate = date.Format("2006-01-02")
	}
	entryType := fieldValue(fields, "type")
	if entryType == "" {
		entryType = category
	}

	return []string{
		entryDate,
		category,
		fieldValue(fields, "daytime", "meal"),
		fieldValue(fields, "id", "entry_id", "uuid"),
		entryType,
		fieldValue(fields, "product_id", "recipe_id", "simple_product_id"),
		fieldValue(fields, "name", "title"),
		fieldValue(fields, "producer", "brand"),
		fieldValue(fields, "amount"),
		fieldValue(fields, "serving"),
		fieldValue(fields, "serving_quantity"),
		rawJSON(item),
	}
}

func mapFields(item any) map[string]any {
	if fields, ok := item.(map[string]any); ok {
		return fields
	}
	content, err := json.Marshal(item)
	if err != nil {
		return nil
	}
	var fields map[string]any
	if err := json.Unmarshal(content, &fields); err != nil {
		return nil
	}
	return fields
}

func fieldValue(fields map[string]any, names ...string) string {
	for _, name := range names {
		value, ok := fields[name]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			return typed
		case float64:
			return formatCSVFloat(typed)
		case float32:
			return formatCSVFloat(float64(typed))
		case int:
			return strconv.Itoa(typed)
		case int64:
			return strconv.FormatInt(typed, 10)
		case int32:
			return strconv.FormatInt(int64(typed), 10)
		case uint:
			return strconv.FormatUint(uint64(typed), 10)
		case uint64:
			return strconv.FormatUint(typed, 10)
		case uint32:
			return strconv.FormatUint(uint64(typed), 10)
		default:
			return fmt.Sprint(typed)
		}
	}
	return ""
}

func rawJSON(item any) string {
	content, err := json.Marshal(item)
	if err != nil {
		return ""
	}
	return string(content)
}

func formatCSVFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
