package services

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// IPAPIGeoIPService implements GeoIPProvider using ip-api.com (free tier)
type IPAPIGeoIPService struct {
	cache map[string]*GeoIPData
	mu    sync.RWMutex
}

// NewIPAPIGeoIPService creates a new service
func NewIPAPIGeoIPService() *IPAPIGeoIPService {
	return &IPAPIGeoIPService{
		cache: make(map[string]*GeoIPData),
	}
}

// IPAPIResponse represents the response from ip-api.com
type IPAPIResponse struct {
	Status      string  `json:"status"`
	Country     string  `json:"country"`
	CountryCode string  `json:"countryCode"`
	Region      string  `json:"regionName"`
	City        string  `json:"city"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	Query       string  `json:"query"`
}

// Lookup performs a GeoIP lookup
func (s *IPAPIGeoIPService) Lookup(ip string) *GeoIPData {
	// 1. Handle localhost/private IPs
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return nil
	}

	if parsedIP.IsLoopback() || isPrivateIP(parsedIP) {
		// Return dummy data for localhost testing so charts aren't empty
		return &GeoIPData{
			CountryCode: "LO",
			CountryName: "Localhost",
			City:        "Dev Environment",
			Region:      "Local",
			Latitude:    0.0,
			Longitude:   0.0,
		}
	}

	// 2. Check cache
	s.mu.RLock()
	if val, ok := s.cache[ip]; ok {
		s.mu.RUnlock()
		return val
	}
	s.mu.RUnlock()

	// 3. Call API (with timeout)
	client := http.Client{
		Timeout: 5 * time.Second, // Increased timeout
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("http://ip-api.com/json/%s", ip), nil)
	if err != nil {
		fmt.Printf("[GeoIP] Failed to create request: %v\n", err)
		return nil
	}
	req.Header.Set("User-Agent", "QRApp/1.0 (internal test)")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("[GeoIP] Request failed for IP %s: %v\n", ip, err)
		return nil // Fail silently
	}
	defer resp.Body.Close()

	var result IPAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Printf("[GeoIP] Decode failed for IP %s: %v\n", ip, err)
		return nil
	}

	if result.Status != "success" {
		fmt.Printf("[GeoIP] API returned error for IP %s: %s\n", ip, result.Status)
		return nil
	}

	data := &GeoIPData{
		CountryCode: result.CountryCode,
		CountryName: result.Country,
		City:        result.City,
		Region:      result.Region,
		Latitude:    result.Lat,
		Longitude:   result.Lon,
	}

	// 4. Update cache
	s.mu.Lock()
	s.cache[ip] = data
	s.mu.Unlock()

	return data
}

func isPrivateIP(ip net.IP) bool {
	// Simple check for private ranges (10.x, 172.16-31.x, 192.168.x)
	// This is a simplified check
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}

	if ip4[0] == 10 {
		return true
	}
	if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 {
		return true
	}
	if ip4[0] == 192 && ip4[1] == 168 {
		return true
	}
	return false
}
