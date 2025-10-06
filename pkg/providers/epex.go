package providers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"kcas/new/internal/datastore"
)

// EPEXProvider implements MarketDataProvider for EPEX market data
type EPEXProvider struct {
	baseURL string
	params  map[string]string
	timeout time.Duration
}

// NewEPEXProvider creates a new EPEX market data provider with configuration
func NewEPEXProvider(baseURL string, params map[string]string) *EPEXProvider {
	// Set default values if not provided
	if baseURL == "" {
		baseURL = "https://www.epexspot.com/en/market-results"
	}
	if params == nil {
		params = map[string]string{
			"market_area":  "FR",
			"auction":      "IDA1",
			"modality":     "Auction",
			"sub_modality": "Intraday",
			"data_mode":    "table",
		}
	}

	return &EPEXProvider{
		baseURL: baseURL,
		params:  params,
		timeout: 30 * time.Second,
	}
}

// NewDefaultEPEXProvider creates an EPEX provider with default settings
func NewDefaultEPEXProvider() *EPEXProvider {
	return NewEPEXProvider("", nil)
}

// GetName returns the provider name
func (p *EPEXProvider) GetName() string {
	return "EPEX"
}

// GetDataPath returns the file path for the given date
func (p *EPEXProvider) GetDataPath(date time.Time) string {
	return fmt.Sprintf("epex_data_%s.csv", date.Format("2006-01-02"))
}

// FetchData fetches EPEX market data for the given date
func (p *EPEXProvider) FetchData(ctx context.Context, date time.Time) ([]datastore.MarketDataPoint, error) {
	tradingDate := date.AddDate(0, 0, -1).Format("2006-01-02")
	deliveryDate := date.Format("2006-01-02")

	// Build URL with configurable parameters
	url := p.buildURL(tradingDate, deliveryDate)

	client := &http.Client{Timeout: p.timeout}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return p.parseHTMLData(string(body))
}

// parseHTMLData parses HTML content to extract market data
func (p *EPEXProvider) parseHTMLData(html string) ([]datastore.MarketDataPoint, error) {
	periods := p.extractPeriods(html)
	volumes, prices := p.extractTableData(html)

	if len(periods) == 0 || len(volumes) == 0 || len(prices) == 0 {
		return nil, fmt.Errorf("failed to extract data from HTML")
	}

	minLen := minInt(len(periods), len(volumes), len(prices))
	data := make([]datastore.MarketDataPoint, 0, minLen)

	for i := 0; i < minLen; i++ {
		volume, err := strconv.ParseFloat(volumes[i], 64)
		if err != nil {
			continue // Skip invalid data
		}

		price, err := strconv.ParseFloat(prices[i], 64)
		if err != nil {
			continue // Skip invalid data
		}

		data = append(data, datastore.MarketDataPoint{
			Period: periods[i],
			Volume: volume,
			Price:  price,
		})
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("no valid data points extracted")
	}

	return data, nil
}

// extractPeriods extracts time periods from HTML
func (p *EPEXProvider) extractPeriods(html string) []string {
	var periods []string

	re := regexp.MustCompile(`<a href="#">(\d{2}:\d{2}\s*-\s*\d{2}:\d{2})</a>`)
	matches := re.FindAllStringSubmatch(html, -1)

	for _, match := range matches {
		if len(match) > 1 {
			period := strings.ReplaceAll(match[1], " ", "")
			periods = append(periods, period)
		}
	}

	return periods
}

// extractTableData extracts volume and price data from HTML table
func (p *EPEXProvider) extractTableData(html string) ([]string, []string) {
	var volumes []string
	var prices []string

	// Find tbody section
	tbodyStart := strings.Index(html, "<tbody>")
	tbodyEnd := strings.Index(html, "</tbody>")

	if tbodyStart == -1 || tbodyEnd == -1 {
		return volumes, prices
	}

	tbodyContent := html[tbodyStart:tbodyEnd]

	// Try primary extraction method
	if vols, prs := p.extractFromRows(tbodyContent); len(vols) > 0 {
		return vols, prs
	}

	// Fallback to alternative method
	return p.extractFromCells(tbodyContent)
}

// extractFromRows extracts data from table rows
func (p *EPEXProvider) extractFromRows(tbodyContent string) ([]string, []string) {
	var volumes []string
	var prices []string

	trRe := regexp.MustCompile(`<tr\s+class="child[^"]*"[^>]*>([\s\S]*?)</tr>`)
	trMatches := trRe.FindAllStringSubmatch(tbodyContent, -1)

	for _, trMatch := range trMatches {
		if len(trMatch) < 2 {
			continue
		}

		rowContent := trMatch[1]
		tdRe := regexp.MustCompile(`<td[^>]*>([^<]+)</td>`)
		tdMatches := tdRe.FindAllStringSubmatch(rowContent, -1)

		// Each row should have 4 columns: Buy Volume, Sell Volume, Volume, Price
		if len(tdMatches) == 4 {
			volume := strings.TrimSpace(tdMatches[2][1]) // 3rd column = Volume
			price := strings.TrimSpace(tdMatches[3][1])  // 4th column = Price

			volumes = append(volumes, volume)
			prices = append(prices, price)
		}
	}

	return volumes, prices
}

// extractFromCells extracts data from individual cells (fallback method)
func (p *EPEXProvider) extractFromCells(tbodyContent string) ([]string, []string) {
	var volumes []string
	var prices []string

	tdRe := regexp.MustCompile(`<td[^>]*>([^<]+)</td>`)
	tdMatches := tdRe.FindAllStringSubmatch(tbodyContent, -1)

	// Data is in groups of 4: Buy, Sell, Volume, Price
	for i := 0; i+3 < len(tdMatches); i += 4 {
		volume := strings.TrimSpace(tdMatches[i+2][1]) // 3rd column
		price := strings.TrimSpace(tdMatches[i+3][1])  // 4th column

		volumes = append(volumes, volume)
		prices = append(prices, price)
	}

	return volumes, prices
}

// buildURL constructs the EPEX URL with configurable parameters
func (p *EPEXProvider) buildURL(tradingDate, deliveryDate string) string {
	baseParams := fmt.Sprintf("trading_date=%s&delivery_date=%s", tradingDate, deliveryDate)

	// Add configured parameters
	var params []string
	params = append(params, baseParams)

	for key, value := range p.params {
		params = append(params, fmt.Sprintf("%s=%s", key, value))
	}

	// Add empty parameters that EPEX expects
	params = append(params, "underlying_year=", "technology=", "period=", "production_period=")

	return fmt.Sprintf("%s?%s", p.baseURL, strings.Join(params, "&"))
}

// minInt returns the minimum of three integers
func minInt(a, b, c int) int {
	result := a
	if b < result {
		result = b
	}
	if c < result {
		result = c
	}
	return result
}
