package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"text/tabwriter"
	"time"

	"gorm.io/gorm"

	"github.com/spf13/cobra"
)

var (
	db         *gorm.DB
	apiURL     string
	apiKey     string
	outputJSON bool
	verbose    bool
)

// SetDB sets the database connection for direct access
func SetDB(database *gorm.DB) {
	db = database
}

// SetAPIConfig sets the API configuration for remote access
func SetAPIConfig(url, key string) {
	apiURL = url
	apiKey = key
}

// SetOutputJSON sets the output format preference
func SetOutputJSON(json bool) {
	outputJSON = json
}

// SetVerbose sets verbose output
func SetVerbose(v bool) {
	verbose = v
}

// HTTPClient is a configured HTTP client for API calls
var HTTPClient = &http.Client{
	Timeout: 30 * time.Second,
}

// APIRequest makes a request to the pLLM API
func APIRequest(method, endpoint string, body interface{}) (*http.Response, error) {
	if apiURL == "" || apiKey == "" {
		return nil, fmt.Errorf("API URL and key required for remote operations")
	}

	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest(method, apiURL+endpoint, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	if verbose {
		fmt.Printf("Making %s request to: %s\n", method, apiURL+endpoint)
	}

	return HTTPClient.Do(req)
}

// OutputTable outputs data in table format
func OutputTable(headers []string, rows [][]string) {
	if outputJSON {
		// Convert table to JSON structure
		var jsonRows []map[string]string
		for _, row := range rows {
			jsonRow := make(map[string]string)
			for i, cell := range row {
				if i < len(headers) {
					jsonRow[headers[i]] = cell
				}
			}
			jsonRows = append(jsonRows, jsonRow)
		}
		OutputJSON(jsonRows)
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Print headers
	for i, header := range headers {
		if i > 0 {
			_, _ = fmt.Fprint(w, "\t")
		}
		_, _ = fmt.Fprint(w, header)
	}
	_, _ = fmt.Fprintln(w)

	// Print separator
	for i := range headers {
		if i > 0 {
			_, _ = fmt.Fprint(w, "\t")
		}
		_, _ = fmt.Fprint(w, "---")
	}
	_, _ = fmt.Fprintln(w)

	// Print rows
	for _, row := range rows {
		for i, cell := range row {
			if i > 0 {
				_, _ = fmt.Fprint(w, "\t")
			}
			_, _ = fmt.Fprint(w, cell)
		}
		_, _ = fmt.Fprintln(w)
	}

	_ = w.Flush()
}

// OutputJSON outputs data in JSON format
func OutputJSON(data interface{}) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
	}
}

// IsDirectDBAccess returns true if we have database access
func IsDirectDBAccess() bool {
	return db != nil
}

// IsAPIAccess returns true if we have API access configured
func IsAPIAccess() bool {
	return apiURL != "" && apiKey != ""
}

// NewConfigCommand creates a new config command for managing CLI configuration
func NewConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage CLI configuration",
		Long:  "Configure the pLLM CLI for database or API access",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			config := map[string]interface{}{
				"database_access": IsDirectDBAccess(),
				"api_access":      IsAPIAccess(),
				"output_json":     outputJSON,
				"verbose":         verbose,
			}

			if IsAPIAccess() {
				config["api_url"] = apiURL
			}

			if outputJSON {
				OutputJSON(config)
			} else {
				fmt.Printf("Database Access: %v\n", IsDirectDBAccess())
				fmt.Printf("API Access: %v\n", IsAPIAccess())
				if IsAPIAccess() {
					fmt.Printf("API URL: %s\n", apiURL)
				}
				fmt.Printf("JSON Output: %v\n", outputJSON)
				fmt.Printf("Verbose: %v\n", verbose)
			}

			return nil
		},
	})

	return cmd
}
