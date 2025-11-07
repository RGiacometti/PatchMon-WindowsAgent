package utils

import (
	"os"
	"time"
)

// GetTimezone returns the configured timezone from environment variable
// Defaults to UTC if not set
func GetTimezone() string {
	tz := os.Getenv("TZ")
	if tz == "" {
		tz = os.Getenv("TIMEZONE")
	}
	if tz == "" {
		return "UTC"
	}
	return tz
}

// GetTimezoneLocation returns a time.Location for the configured timezone
// Defaults to UTC if not set or invalid
func GetTimezoneLocation() *time.Location {
	tz := GetTimezone()
	
	// Handle UTC explicitly
	if tz == "UTC" || tz == "Etc/UTC" {
		return time.UTC
	}
	
	// Try to load the timezone
	loc, err := time.LoadLocation(tz)
	if err != nil {
		// Fallback to UTC if timezone is invalid
		return time.UTC
	}
	
	return loc
}

// GetCurrentTime returns the current time in the configured timezone
// For database storage, we should use UTC, but this function returns
// the time in the configured timezone for display purposes
func GetCurrentTime() time.Time {
	loc := GetTimezoneLocation()
	return time.Now().In(loc)
}

// GetCurrentTimeUTC returns the current time in UTC
// This should be used for database storage to ensure consistency
func GetCurrentTimeUTC() time.Time {
	return time.Now().UTC()
}

// FormatTimeISO formats a time to ISO 8601 string
func FormatTimeISO(t time.Time) string {
	return t.Format(time.RFC3339)
}

// ParseTime parses a time string and returns a time.Time
// Handles various formats including RFC3339 and Unix timestamps
func ParseTime(timeStr string) (time.Time, error) {
	// Try RFC3339 first (ISO 8601)
	if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
		return t, nil
	}
	
	// Try RFC3339Nano
	if t, err := time.Parse(time.RFC3339Nano, timeStr); err == nil {
		return t, nil
	}
	
	// Try common formats
	formats := []string{
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z07:00",
	}
	
	for _, format := range formats {
		if t, err := time.Parse(format, timeStr); err == nil {
			return t, nil
		}
	}
	
	// If all else fails, return zero time
	return time.Time{}, nil
}

// FormatTimeForDisplay formats a time for display in the configured timezone
func FormatTimeForDisplay(t time.Time) string {
	loc := GetTimezoneLocation()
	return t.In(loc).Format("2006-01-02T15:04:05")
}

