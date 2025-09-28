package config

import (
	"fmt"
	"reflect"
	"time"

	"github.com/go-viper/mapstructure/v2"
)

// MetricsConfig represents the root metrics configuration
type MetricsConfig struct {
    Version      string       `mapstructure:"version,omitempty"`     // default 1.0
    Description  string       `mapstructure:"description,omitempty"` // default ""
    Global       GlobalConfig `mapstructure:"global,omitempty"`
    MetricGroups []MetricGroup `mapstructure:"metric-groups"`
}

// GlobalConfig contains global settings
type GlobalConfig struct {
    DefaultInterval     Duration `mapstructure:"default-interval,omitempty"`      // default 30s
    DefaultQueryTimeout Duration `mapstructure:"default-query-timeout,omitempty"` // default 10s
    DefaultMaxRetries   int      `mapstructure:"default-max-retries,omitempty"`   // default 0
    DefaultRetryDelay   Duration `mapstructure:"default-retry-delay,omitempty"`   // default 5s
}

// MetricGroup represents a group of related metrics
type MetricGroup struct {
    Name        string   `mapstructure:"name"`
    Description string   `mapstructure:"description,omitempty"` // default ""
    Enabled     bool     `mapstructure:"enabled,omitempty"`     // default true
    Metrics     []Metric `mapstructure:"metrics"`
}

// Metric defines a single metric to collect
type Metric struct {
    Name           string                 `mapstructure:"name"`
    Description    string                 `mapstructure:"description,omitempty"`  // default ""
    ValueType      ValueType              `mapstructure:"value-type"`             // default "float"
    Interval       Duration               `mapstructure:"interval,omitempty"`     // default global.default-interval
    CollectionType CollectionType         `mapstructure:"collection-type"`        // default "sql"
    SQLFile        string                 `mapstructure:"sql-file,omitempty"`     // required for sql collection type
    GoFunction     string                 `mapstructure:"go-function,omitempty"`  // required for go_func collection type
	QueryTimeout   Duration               `mapstructure:"query-timeout,omitempty"`// default global.default-query-timeout
    MaxRetries     int                    `mapstructure:"max-retries,omitempty"`  // default global.default-max-retries
    RetryDelay     Duration               `mapstructure:"retry-delay,omitempty"`  // default default-retry-delay
    Unit           string                 `mapstructure:"unit,omitempty"`         // default ""
}

// Custom types for type safety
type MetricType string
type ValueType string
type CollectionType string

const (    
    ValueTypeInt    ValueType = "int"
    ValueTypeFloat  ValueType = "float"
    ValueTypeString ValueType = "string"
    ValueTypeBool   ValueType = "bool"
    ValueTypeTable  ValueType = "table"
    ValueTypeInt64  ValueType = "int64"
    
    CollectionTypeSQL    CollectionType = "sql"
    CollectionTypeGoFunc CollectionType = "go_func"
)

// Duration wraps time.Duration for custom unmarshaling
type Duration struct {
    time.Duration
}

// UnmarshalMapstructure implements mapstructure decode hook
func (d *Duration) UnmarshalMapstructure(input interface{}) error {
    switch v := input.(type) {
    case string:
        dur, err := time.ParseDuration(v)
        if err != nil {
            return fmt.Errorf("invalid duration '%s': %w", v, err)
        }
        d.Duration = dur
    case int, int64, float64:
        // Handle numeric values as seconds
        seconds, ok := input.(float64)
        if !ok {
            return fmt.Errorf("invalid duration type: %T", input)
        }
        d.Duration = time.Duration(seconds * float64(time.Second))
    default:
        return fmt.Errorf("unsupported duration type: %T", input)
    }
    return nil
}

// String returns the duration as string
func (d Duration) String() string {
    return d.Duration.String()
}

// customDurationHook returns a mapstructure.DecodeHookFunc that handles 
// conversion from string (or number) to the custom config.Duration type.
// It relies on the UnmarshalMapstructure logic defined in the Duration type.
func customDurationHook() mapstructure.DecodeHookFunc {
    return func(
        f reflect.Type, // The source type
        t reflect.Type, // The target type
        data interface{},
    ) (interface{}, error) {
        // Only proceed if the target is our custom Duration struct
        if t != reflect.TypeOf(Duration{}) {
            return data, nil
        }
        
        // We expect the source data to be a string or a number (handled by UnmarshalMapstructure)
        
        // Create a temporary Duration struct
        d := Duration{} 
        
        // Use the custom logic defined in the Duration struct
        if err := d.UnmarshalMapstructure(data); err != nil {
            return nil, err
        }
        
        // Return the successfully converted struct
        return d, nil
    }
}