package lookup

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func Lenovo(serial string) (*WarrantyInfo, error) {
	url := fmt.Sprintf("https://pcsupport.lenovo.com/us/en/api/v4/mse/getProductInfoBySerial?serialNumber=%s", serial)
	client := &http.Client{Timeout: 8 * time.Second}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lenovo API: HTTP %d", resp.StatusCode)
	}

	var result struct {
		Data struct {
			WarrantyInfo []struct {
				EndDate string `json:"warrantyEndDate"`
			} `json:"warrantyInfo"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	info := &WarrantyInfo{Vendor: "Lenovo", Model: result.Data.Name}
	for _, w := range result.Data.WarrantyInfo {
		if w.EndDate > info.Expiry {
			info.Expiry = w.EndDate
		}
	}
	return info, nil
}

// TryAll attempts Dell, HP, and Lenovo lookups in order and returns the first success.
// Never fails hard — always returns nil, nil if all lookups fail.
func TryAll(serial string) *WarrantyInfo {
	if serial == "" {
		return nil
	}
	for _, fn := range []func(string) (*WarrantyInfo, error){Dell, HP, Lenovo} {
		if info, err := fn(serial); err == nil && info != nil {
			return info
		}
	}
	return nil
}
